package client

import (
	"context"
	"crypto/rand"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/data"
	"github.com/VerTox/l2go/internal/gameserver/gameloop"
	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/packets/inclient"
	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
	"github.com/VerTox/l2go/internal/gameserver/registry"
	"github.com/VerTox/l2go/internal/gameserver/transport/client"
	"github.com/VerTox/l2go/internal/gameserver/usecase"
	"github.com/VerTox/l2go/pkg/l2pkt"
)

// handleEnterWorld processes final world entry after character selection
func (h *Handler) handleEnterWorld(ctx context.Context, c *client.ClientConn, payload []byte) error {
	// Get session to access account and character info
	session := h.getSession(c)
	if session == nil {
		log.Ctx(ctx).Error().Msg("no session found for world entry")
		return nil
	}

	_ = inclient.NewEnterWorld(payload)
	log.Ctx(ctx).Info().
		Str("account", session.AccountName).
		Msg("EnterWorld request - final world entry")

	// Get player from world registry
	playerState, exists := h.world.GetPlayerByAccount(session.AccountName)
	if !exists {
		log.Ctx(ctx).Error().
			Str("account", session.AccountName).
			Msg("player not found in world for EnterWorld")
		return nil
	}

	// Send world entry packet sequence
	if err := h.sendWorldEntryPackets(ctx, c, playerState.Character); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to send world entry packets")
		return err
	}

	// Send nearby NPCs to us. Player-to-player visibility is established by the game
	// loop when it processes CmdPlayerEnteredWorld below.
	h.establishNpcVisibility(ctx, c, playerState)

	// Notify game loop about player entering the world (activates regions + spawns
	// nearby players to us and us to them).
	h.gameLoopCmd <- gameloop.CmdPlayerEnteredWorld{
		CharID:      playerState.CharID,
		AccountName: session.AccountName,
		Position:    playerState.Position,
	}

	log.Ctx(ctx).Info().
		Int32("char_id", playerState.CharID).
		Str("name", playerState.Character.Name).
		Int("x", playerState.Position.X).
		Int("y", playerState.Position.Y).
		Int("z", playerState.Position.Z).
		Msg("Player fully entered world with visibility system")

	return nil
}

// sendWorldEntryPackets sends the sequence of packets needed for world entry
func (h *Handler) sendWorldEntryPackets(ctx context.Context, c *client.ClientConn, char *models.Character) error {
	// 1. Send CharSelected - confirms character selection
	//charSelectedData := h.buildCharSelectedPacket(char)
	//if err := c.Send(charSelectedData); err != nil {
	//	return fmt.Errorf("failed to send CharSelected: %w", err)
	//}

	shortCutData := h.BuildShortCutPacket(char)
	if err := c.Send(shortCutData); err != nil {
		return fmt.Errorf("failed to send ShortCut: %w", err)
	}

	itemListData := h.buildItemListPacket(ctx, char)
	if err := c.Send(itemListData); err != nil {
		return fmt.Errorf("failed to send ItemList: %w", err)
	}

	hennaInfoData := h.buildHennaInfoPacket(ctx, char)
	if err := c.Send(hennaInfoData); err != nil {
		return fmt.Errorf("failed to send HennaInfo: %w", err)
	}

	// 2. Send UserInfo - comprehensive character information
	userInfoData := h.buildUserInfoPacket(char)
	if err := c.Send(userInfoData); err != nil {
		return fmt.Errorf("failed to send UserInfo: %w", err)
	}

	// 3. Send SkillList - character skills
	skillsData := h.buildSkillListPacket(ctx, char)
	if err := c.Send(skillsData); err != nil {
		return fmt.Errorf("failed to send SkillList: %w", err)
	}

	// 4. Send ExBasicActionList - available actions (Walk/Run toggle, etc.)
	// CRITICAL: This creates the action buttons in client UI
	actionListData := outclient.BuildDefaultExBasicActionList() // Just Walk/Run toggle for now
	if err := c.Send(actionListData); err != nil {
		return fmt.Errorf("failed to send ExBasicActionList: %w", err)
	}

	log.Ctx(ctx).Info().
		Str("name", char.Name).
		Int32("char_id", char.ID).
		Int32("x", int32(char.Position.X)).
		Int32("y", int32(char.Position.Y)).
		Int32("z", int32(char.Position.Z)).
		Msg("World entry packets sent successfully (including ExBasicActionList)")

	return nil
}

// buildCharSelectedPacket creates CharSelected packet from character data
func (h *Handler) buildCharSelectedPacket(char *models.Character) []byte {
	charSelected := outclient.CharSelected{
		Name:      char.Name,
		ObjectID:  char.ID,
		Title:     char.Title,
		SessionID: int32(generateSessionID()),
		ClanID:    int32(char.ClanID),
		Sex:       int32(char.Sex),
		Race:      int32(char.Race),
		ClassID:   int32(char.ClassID),
		X:         int32(char.Position.X),
		Y:         int32(char.Position.Y),
		Z:         int32(char.Position.Z),
		CurrentHP: char.CurrentHP,
		CurrentMP: char.CurrentMP,
		SP:        int32(char.SP),
		EXP:       char.Experience,
		Level:     int32(char.Level),
		Karma:     int32(char.Karma),
		PkKills:   int32(char.PKKills),
		// Attributes from character base stats
		INT: int32(char.BaseINT),
		STR: int32(char.BaseSTR),
		CON: int32(char.BaseCON),
		MEN: int32(char.BaseMEN),
		DEX: int32(char.BaseDEX),
		WIT: int32(char.BaseWIT),
		// Game time (placeholder — no game time controller yet)
		GameTime: 0,
	}
	return outclient.BuildCharSelected(charSelected)
}

// buildUserInfoPacket creates UserInfo packet from character data
func (h *Handler) buildUserInfoPacket(char *models.Character) []byte {
	// Get player state from world registry to include run/walk and combat state
	var runningFlag int32 = 1  // Default to running (L2J default)
	var inCombatFlag int32 = 0 // Default to peaceful
	if playerState, exists := h.world.GetPlayer(char.ID); exists {
		if playerState.IsRunning {
			runningFlag = 1
		} else {
			runningFlag = 0
		}
		if playerState.InCombat {
			inCombatFlag = 1
		}
		log.Debug().
			Int32("char_id", char.ID).
			Bool("player_is_running", playerState.IsRunning).
			Bool("player_in_combat", playerState.InCombat).
			Int32("running_flag", runningFlag).
			Int32("in_combat_flag", inCombatFlag).
			Msg("UserInfo: setting run/walk and combat flags from player state")
	} else {
		log.Debug().
			Int32("char_id", char.ID).
			Int32("running_flag", runningFlag).
			Msg("UserInfo: player not found in world registry, using defaults")
	}

	// Load paperdoll data from equipped items
	paperdoll := h.loadPaperdollInfo(context.Background(), char.ID)

	// Compute derived combat stats from base stats and class template
	baseStats := models.CharacterStats{
		STR: char.BaseSTR,
		DEX: char.BaseDEX,
		CON: char.BaseCON,
		INT: char.BaseINT,
		WIT: char.BaseWIT,
		MEN: char.BaseMEN,
	}
	combat := usecase.GetCombatBaseStatsByClass(char.ClassID)
	computed := models.ComputeStats(baseStats, char.Level, combat)

	// Add equipment bonuses from paperdoll items
	reg := registry.GetItemTemplateRegistry()
	items, err := h.characterUseCase.GetCharacterAllItems(context.Background(), char.ID)
	if err == nil {
		for _, item := range items {
			if item.Loc != string(models.LocPaperdoll) {
				continue
			}
			tpl := reg.Get(item.ItemID)
			if tpl == nil {
				continue
			}
			computed.PAtk += tpl.PAtk
			computed.MAtk += tpl.MAtk
			computed.PDef += tpl.PDef
			computed.MDef += tpl.MDef
			if tpl.PAtkSpd > 0 {
				computed.PAtkSpd += tpl.PAtkSpd
			}
			if tpl.MAtkSpd > 0 {
				computed.MAtkSpd += tpl.MAtkSpd
			}
		}
	}

	userInfo := outclient.UserInfo{
		X:        int32(char.Position.X),
		Y:        int32(char.Position.Y),
		Z:        int32(char.Position.Z),
		ObjectID: char.ID,
		Name:     char.Name,
		Race:     int32(char.Race),
		Sex:      int32(char.Sex),
		ClassID:  int32(char.ClassID),
		Level:    int32(char.Level),
		EXP:      char.Experience,
		// EXP-бар клиента заполняется по доле прогресса уровня (0.0–1.0), а не по
		// абсолютному EXP. Без этого поля бар пуст при входе в мир, пока боевой
		// UserInfo из game loop не пришлёт корректное значение (баг l2go-dlk).
		ExpPercent: data.ExpPercent(int(char.Level), char.Experience) / 100.0,
		// Base stats from character
		STR: int32(char.BaseSTR),
		DEX: int32(char.BaseDEX),
		CON: int32(char.BaseCON),
		INT: int32(char.BaseINT),
		WIT: int32(char.BaseWIT),
		MEN: int32(char.BaseMEN),
		// Health and mana
		MaxHP:     int32(char.MaxHP),
		CurrentHP: int32(char.CurrentHP),
		MaxMP:     int32(char.MaxMP),
		CurrentMP: int32(char.CurrentMP),
		MaxCP:     int32(char.MaxCP),
		CurrentCP: int32(char.CurrentCP),
		// Skill points and load
		CurrentSP:   int64(char.SP),
		CurrentLoad: 0,
		MaxLoad:     int32(computed.MaxLoad),
		// Computed combat stats
		PAtk:     int32(computed.PAtk),
		AtkSpd:   int32(computed.PAtkSpd),
		PDef:     int32(computed.PDef),
		Evasion:  int32(computed.Evasion),
		Accuracy: int32(computed.Accuracy),
		Critical: int32(computed.CritRate),
		MAtk:     int32(computed.MAtk),
		CastSpd:  int32(computed.MAtkSpd),
		MDef:     int32(computed.MDef),
		// PvP status
		PvPFlag: 0,
		Karma:   int32(char.Karma),
		// Computed movement speeds
		RunSpd:      int32(computed.RunSpd),
		WalkSpd:     int32(computed.WalkSpd),
		SwimRunSpd:  int32(computed.SwimRunSpd),
		SwimWalkSpd: int32(computed.SwimWalkSpd),
		FlyRunSpd:   0,
		FlyWalkSpd:  0,
		// Clan info
		ClanID:    int32(char.ClanID),
		ClanCrest: 0, // TODO: Load clan crest
		AllyID:    0, // TODO: Load ally ID
		AllyCrest: 0, // TODO: Load ally crest
		// PK/PvP kills
		PKKills:  int32(char.PKKills),
		PVPKills: int32(char.PvPKills),
		// Other attributes
		Cubics:         []int32{},           // TODO: Load active cubics
		AbnormalMask:   0,                   // TODO: Load abnormal effects
		ClanPrivs:      0,                   // TODO: Load clan privileges
		RecomLeft:      0,                   // TODO: Load recommendations left
		RecomHave:      0,                   // TODO: Load recommendations received
		InventoryLimit: 80,                  // TODO: Calculate inventory limit
		ClassId2:       int32(char.ClassID), // TODO: Handle dual class
		Title:          "",                  // TODO: Load character title

		// Vehicle and equipment
		VehicleID: 0, // Not in vehicle

		// Paperdoll - loaded from equipped items
		Paperdoll: paperdoll,

		// Equipment capabilities
		TalismanSlots: 0, // TODO: Calculate based on character level/items
		CanEquipCloak: 1, // Allow cloak equipping

		// Combat state - CRITICAL FIX for run/walk animation
		SittingFlag: 0,            // 0 = standing, 1 = sitting
		RunningFlag: runningFlag,  // Use actual player state
		InCombat:    inCombatFlag, // Use actual combat state
		Deceased:    0,            // 0 = alive, 1 = dead
		Invisible:   0,            // 0 = visible, 1 = invisible

		// T2 Additional fields
		Fame:           0, // TODO: Load character fame
		MinimapAllowed: 1, // Allow minimap usage
		VitalityPoints: 0, // TODO: Load vitality points
		SpecialEffects: 0, // TODO: Load special visual effects

		// Appearance from character model
		HairStyle: int32(char.HairStyle),
		HairColor: int32(char.HairColor),
		Face:      int32(char.Face),

		// Collision per race/sex
		CollisionRadius: getCollisionRadius(char.Race, char.Sex),
		CollisionHeight: getCollisionHeight(char.Race, char.Sex),
	}

	return outclient.BuildUserInfo(userInfo)
}

// loadPaperdollInfo loads equipment data for UserInfo packet
func (h *Handler) loadPaperdollInfo(ctx context.Context, charID int32) *outclient.PaperdollInfo {
	paperdoll := outclient.NewPaperdollInfo()

	// Load equipped items from database
	items, err := h.characterUseCase.GetCharacterAllItems(ctx, charID)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).
			Int32("char_id", charID).
			Msg("failed to load paperdoll items")
		return paperdoll
	}

	// Process only equipped items (PAPERDOLL location)
	equippedCount := 0
	for _, item := range items {
		if item.Loc == string(models.LocPaperdoll) && item.LocData >= 0 && item.LocData < 26 {
			slot := item.LocData
			paperdoll.ObjectIDs[slot] = item.ObjectID
			paperdoll.DisplayIDs[slot] = item.ItemID // ItemID is used for visual display
			paperdoll.AugmentIDs[slot] = int32(item.AugmentationID)
			equippedCount++

			log.Debug().
				Int32("char_id", charID).
				Int("slot", slot).
				Int32("object_id", item.ObjectID).
				Int32("item_id", item.ItemID).
				Msg("loaded paperdoll item")
		}
	}

	log.Debug().
		Int32("char_id", charID).
		Int("equipped_count", equippedCount).
		Msg("paperdoll info loaded")

	return paperdoll
}

// buildInventoryUpdatePacket creates InventoryUpdate packet from character data
func (h *Handler) buildInventoryUpdatePacket(ctx context.Context, char *models.Character) []byte {
	// Load character items for debugging
	items, err := h.characterUseCase.GetCharacterAllItems(ctx, char.ID)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).
			Int32("char_id", char.ID).
			Msg("failed to load character items for InventoryUpdate")
		// Return empty update on error
		inventoryUpdate := outclient.InventoryUpdate{
			Items: []outclient.InventoryItem{},
		}
		return outclient.BuildInventoryUpdate(inventoryUpdate)
	}

	// Log items for InventoryUpdate analysis
	log.Ctx(ctx).Debug().
		Int32("char_id", char.ID).
		Int("item_count", len(items)).
		Msg("Loaded character items for InventoryUpdate")

	// Filter only INVENTORY items (exclude PAPERDOLL for debugging)
	inventoryOnlyItems := make([]models.CharacterItem, 0)
	for _, item := range items {
		if item.Loc == string(models.LocInventory) {
			inventoryOnlyItems = append(inventoryOnlyItems, item)
		}
	}

	log.Ctx(ctx).Debug().
		Int32("char_id", char.ID).
		Int("total_items", len(items)).
		Int("inventory_only_items", len(inventoryOnlyItems)).
		Msg("Filtered inventory items for InventoryUpdate")

	// Convert only inventory items to InventoryUpdate format
	inventoryItems := convertCharacterItemsToInventoryItems(inventoryOnlyItems)

	log.Ctx(ctx).Debug().
		Int32("char_id", char.ID).
		Int("converted_items", len(inventoryItems)).
		Msg("Converted items for InventoryUpdate")

	inventoryUpdate := outclient.InventoryUpdate{
		Items: inventoryItems,
	}
	return outclient.BuildInventoryUpdate(inventoryUpdate)
}

func (h *Handler) BuildShortCutPacket(char *models.Character) []byte {
	shortCut := make([]outclient.ShortCut, 0)
	return outclient.BuildShortCutInit(shortCut)
}

func (h *Handler) buildItemListPacket(ctx context.Context, char *models.Character) []byte {
	// Load character items for debugging
	items, err := h.characterUseCase.GetCharacterAllItems(ctx, char.ID)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).
			Int32("char_id", char.ID).
			Msg("failed to load character items for ItemList")
		// Return empty list on error
		itemList := outclient.ItemList{
			ShowWindow: false,
			Items:      []outclient.ItemEntry{},
		}
		return l2pkt.BuildPacket(itemList)
	}

	// Convert ALL character items to ItemList format (including PAPERDOLL)
	itemEntries := convertCharacterItemsToItemList(items)

	log.Ctx(ctx).Debug().
		Int32("char_id", char.ID).
		Int("total_items", len(items)).
		Int("converted_items", len(itemEntries)).
		Msg("Converted items for ItemList")

	itemList := outclient.ItemList{
		ShowWindow: false, // Don't show inventory window automatically
		Items:      itemEntries,
	}

	return l2pkt.BuildPacket(itemList)
}

func (h *Handler) buildHennaInfoPacket(ctx context.Context, char *models.Character) []byte {
	// Henna stat bonuses — all zero until henna system is implemented
	hennaInfo := outclient.HennaInfo{
		INT:   0,
		STR:   0,
		CON:   0,
		MEN:   0,
		DEX:   0,
		WIT:   0,
		Slots: [3]int32{},
	}
	return l2pkt.BuildPacket(hennaInfo)
}

// buildSkillListPacket creates SkillList packet from character data
func (h *Handler) buildSkillListPacket(ctx context.Context, char *models.Character) []byte {
	// TODO: Load real skills from database
	// For now, send an empty skill list
	skillList := outclient.SkillList{
		Skills: []outclient.SkillInfo{}, // Empty for now
	}

	return outclient.BuildSkillList(skillList)
}

// generateSessionID generates a random session ID
func generateSessionID() uint32 {
	var bytes [4]byte
	rand.Read(bytes[:])
	return uint32(bytes[0])<<24 | uint32(bytes[1])<<16 | uint32(bytes[2])<<8 | uint32(bytes[3])
}

// convertCharacterItemsToInventoryItems converts character items to InventoryUpdate format
func convertCharacterItemsToInventoryItems(items []models.CharacterItem) []outclient.InventoryItem {
	result := make([]outclient.InventoryItem, len(items))
	for i, item := range items {
		equipped := item.Loc == string(models.LocPaperdoll)
		bodyPart := getBodyPartBitmask(item.ItemID, item.LocData, item.Loc)
		itemType := getItemType(item.ItemID)

		// LocationSlot: PAPERDOLL slot index for equipped, -1 for inventory
		var locSlot int32 = -1
		if equipped && item.LocData >= 0 {
			locSlot = int32(item.LocData)
		}

		result[i] = outclient.InventoryItem{
			UpdateType:   outclient.UpdateTypeModify,
			ObjectID:     item.ObjectID,
			ItemID:       item.ItemID,
			LocationSlot: locSlot,
			Count:        item.Count,
			ItemType:     itemType,
			CustomType1:  int32(item.CustomType1),
			Equipped:     equipped,
			BodyPart:     bodyPart,
			EnchantLevel: int32(item.EnchantLevel),
			CustomType2:  int32(item.CustomType2),

			// Augmentation
			AugmentationID: int32(item.AugmentationID),

			// Shadow/mana
			Mana:          int32(item.ManaLeft),
			TimeRemaining: -9999,

			// Elemental attributes
			AttackElementType:   0,
			AttackElementPower:  0,
			DefenseElementFire:  int32(item.AttributeFire),
			DefenseElementWater: int32(item.AttributeWater),
			DefenseElementWind:  int32(item.AttributeWind),
			DefenseElementEarth: int32(item.AttributeEarth),
			DefenseElementHoly:  int32(item.AttributeHoly),
			DefenseElementDark:  int32(item.AttributeDark),
		}

		// Debug logging for each converted item
		log.Debug().
			Int("index", i).
			Int32("object_id", result[i].ObjectID).
			Int32("item_id", result[i].ItemID).
			Int64("count", result[i].Count).
			Int32("item_type", result[i].ItemType).
			Bool("equipped", result[i].Equipped).
			Int32("body_part", result[i].BodyPart).
			Msg("Converted item for InventoryUpdate")
	}
	return result
}

// convertCharacterItemsToItemList converts character items to ItemList format
func convertCharacterItemsToItemList(items []models.CharacterItem) []outclient.ItemEntry {
	result := make([]outclient.ItemEntry, 0, len(items))

	for _, item := range items {
		equipped := item.Loc == string(models.LocPaperdoll)

		// LocationSlot: PAPERDOLL slot index for equipped, -1 for inventory
		var locationSlot int32 = -1
		if equipped && item.LocData >= 0 {
			locationSlot = int32(item.LocData)
		}

		// BodyPart: Get bitmask from item template (or paperdoll slot if equipped)
		bodyPart := getBodyPartBitmask(item.ItemID, item.LocData, item.Loc)

		// ItemType: Type2 classification from item template
		itemType := getItemType(item.ItemID)

		entry := outclient.ItemEntry{
			ObjectID:     item.ObjectID,
			ItemID:       item.ItemID,
			LocationSlot: locationSlot,
			Count:        item.Count,
			ItemType:     itemType,
			Equipped:     equipped,
			BodyPart:     bodyPart,
			EnchantLevel: int32(item.EnchantLevel),
			CustomType1:  int32(item.CustomType1),
			CustomType2:  int32(item.CustomType2),

			// Augmentation
			AugmentationID: int32(item.AugmentationID),
			Mana:           int32(item.ManaLeft),

			// Temporary items (use -9999 for permanent items)
			RemainingTime: -9999,

			// Elemental attributes
			AttackElementType:   0, // TODO: implement based on item data
			AttackElementPower:  0,
			DefenseElementFire:  int32(item.AttributeFire),
			DefenseElementWater: int32(item.AttributeWater),
			DefenseElementWind:  int32(item.AttributeWind),
			DefenseElementEarth: int32(item.AttributeEarth),
			DefenseElementHoly:  int32(item.AttributeHoly),
			DefenseElementDark:  int32(item.AttributeDark),

			// Enchant options (placeholder)
			EnchantOptions: [3]int32{0, 0, 0},
		}

		result = append(result, entry)

		// Debug logging for each converted item
		log.Debug().
			Int32("object_id", entry.ObjectID).
			Int32("item_id", entry.ItemID).
			Int32("location_slot", entry.LocationSlot).
			Int64("count", entry.Count).
			Int32("item_type", entry.ItemType).
			Bool("equipped", entry.Equipped).
			Int32("body_part", entry.BodyPart).
			Str("location", item.Loc).
			Int("loc_data", item.LocData).
			Msg("Converted item for ItemList")
	}

	return result
}

// getItemType determines item type based on item ID
// Uses item template registry for accurate classification
func getItemType(itemID int32) int32 {
	// First try to get from item template registry
	itemType2 := registry.GetItemType2(itemID)
	return int32(itemType2)
}

// establishPlayerVisibility wires up mutual visibility for a player at its current
// position: sends nearby players (CharInfo) and NPCs (NpcInfo, repopulating KnownNPCs)
// to this client, and sends this player (CharInfo) to nearby clients. Shared by world
// entry and teleport arrival (Appearing). Returns the number of other nearby players.
// establishNpcVisibility sends nearby NPCs to the client and repopulates the known
// NPC set. Player-to-player visibility is owned by the game loop (it spawns players
// on CmdPlayerEnteredWorld / movement), so it is intentionally not handled here. (l2go-23g)
func (h *Handler) establishNpcVisibility(ctx context.Context, c *client.ClientConn, playerState *registry.PlayerWorldState) {
	nearbyNPCs := h.world.GetNPCsInRange(playerState.Position, 2500)
	for _, npc := range nearbyNPCs {
		if err := c.Send(outclient.BuildNpcInfo(npc)); err != nil {
			log.Ctx(ctx).Warn().Err(err).Int32("npc_obj_id", npc.ObjectID).Msg("failed to send NpcInfo")
		}
		playerState.KnownNPCs[npc.ObjectID] = true
	}

	log.Ctx(ctx).Debug().
		Int("nearby_npcs", len(nearbyNPCs)).
		Msg("NPC visibility established")
}

// sendPlayerSpawnToClient sends a player's CharInfo (+ RelationChanged) to a client,
// used to refresh appearance after an equipment change. Visuals come from the cached
// paperdoll and live registry state, so no DB lookup is needed.
func (h *Handler) sendPlayerSpawnToClient(ctx context.Context, c *client.ClientConn, char *models.Character) error {
	// Live running/combat/heading from the world registry (fall back to persisted).
	isRunning, inCombat, heading := true, false, int32(char.Heading)
	if playerState, exists := h.world.GetPlayer(char.ID); exists {
		isRunning = playerState.IsRunning
		inCombat = playerState.InCombat
		heading = playerState.Heading
	}

	charInfo := outclient.NewCharInfo(char, &char.Position, char.PaperdollItems, isRunning, inCombat, heading)
	if err := c.Send(charInfo.GetData()); err != nil {
		return fmt.Errorf("failed to send CharInfo packet: %w", err)
	}

	// RelationChanged establishes a normal (non-attackable) cursor for the player.
	relationPacket := outclient.NewSingleRelation(char.ID, int32(char.Karma), 0)
	if err := c.Send(relationPacket.GetData()); err != nil {
		log.Ctx(ctx).Warn().Err(err).Int32("char_id", char.ID).
			Msg("failed to send RelationChanged packet, cursor may show as attackable")
	}

	return nil
}

// getBodyPartBitmask returns the body part bitmask for items
// For equipped items, derives from paperdoll slot
// For inventory items, gets from item template
func getBodyPartBitmask(itemID int32, locationData int, location string) int32 {
	// For equipped items, use paperdoll slot to get body part
	if location == string(models.LocPaperdoll) {
		slot := models.PaperdollSlot(locationData)
		if bitmask, exists := models.PaperdollSlotCodes[slot]; exists {
			return bitmask
		}
	}

	// For all items, try to get body part from item template
	// This allows inventory items to show which body part they can equip to
	bodyPartCode := registry.GetBodyPartCode(itemID)
	if bodyPartCode != 0 {
		return bodyPartCode
	}

	return 0 // no body part or unknown item
}

// collisionKey encodes race+sex into a single lookup key
type collisionKey struct {
	race, sex int
}

// Collision radius values per race/sex from Java L2J pcCollision data (fighter base class)
var collisionRadiusMap = map[collisionKey]float64{
	{0, 0}: 9.0,  // Human Male
	{0, 1}: 8.0,  // Human Female
	{1, 0}: 7.5,  // Elf Male
	{1, 1}: 7.5,  // Elf Female
	{2, 0}: 7.5,  // Dark Elf Male
	{2, 1}: 7.0,  // Dark Elf Female
	{3, 0}: 11.0, // Orc Male
	{3, 1}: 7.0,  // Orc Female
	{4, 0}: 9.0,  // Dwarf Male
	{4, 1}: 5.0,  // Dwarf Female
	{5, 0}: 8.0,  // Kamael Male
	{5, 1}: 7.0,  // Kamael Female
}

// Collision height values per race/sex from Java L2J pcCollision data (fighter base class)
var collisionHeightMap = map[collisionKey]float64{
	{0, 0}: 23.0, // Human Male
	{0, 1}: 23.5, // Human Female
	{1, 0}: 24.0, // Elf Male
	{1, 1}: 23.0, // Elf Female
	{2, 0}: 24.0, // Dark Elf Male
	{2, 1}: 23.5, // Dark Elf Female
	{3, 0}: 28.0, // Orc Male
	{3, 1}: 27.0, // Orc Female
	{4, 0}: 18.0, // Dwarf Male
	{4, 1}: 19.0, // Dwarf Female
	{5, 0}: 25.2, // Kamael Male
	{5, 1}: 22.6, // Kamael Female
}

// getCollisionRadius returns collision radius for the given race and sex
func getCollisionRadius(race, sex int) float64 {
	if r, ok := collisionRadiusMap[collisionKey{race, sex}]; ok {
		return r
	}
	return 9.0 // default Human Male
}

// getCollisionHeight returns collision height for the given race and sex
func getCollisionHeight(race, sex int) float64 {
	if h, ok := collisionHeightMap[collisionKey{race, sex}]; ok {
		return h
	}
	return 23.0 // default Human Male
}
