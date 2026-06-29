package client

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/packets/inclient"
	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
	"github.com/VerTox/l2go/internal/gameserver/transport/client"
	"github.com/VerTox/l2go/pkg/l2pkt"
)

// handleNewCharacter processes character template requests
func (h *Handler) handleNewCharacter(ctx context.Context, c *client.ClientConn, payload []byte) error {
	_ = inclient.NewNewCharacter(payload)
	log.Ctx(ctx).Info().Msg("Character templates request")

	// Load real templates from use case
	templates, err := h.characterUseCase.GetCharacterTemplates(ctx)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to load character templates")
		// Send empty templates on error
		if err := c.Send(outclient.NewCharacterSuccess()); err != nil {
			return err
		}
		return nil
	}

	// Send character creation templates
	templatesPacket := outclient.NewCharacterSuccess()
	if err := c.Send(templatesPacket); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to send character templates")
		return err
	}

	log.Ctx(ctx).Info().
		Int("template_count", len(templates)).
		Msg("Character templates sent successfully")

	return nil
}

// handleCharacterCreate processes character creation
func (h *Handler) handleCharacterCreate(ctx context.Context, c *client.ClientConn, payload []byte) error {
	// Get session to access account name
	session := h.getSession(c)
	if session == nil {
		log.Ctx(ctx).Error().Msg("no session found for character creation")
		if err := c.Send(outclient.NewCharCreateOk(false)); err != nil {
			return err
		}
		return nil
	}

	packet := &inclient.CharacterCreate{}
	l2pkt.ParsePacket(payload, packet)

	log.Ctx(ctx).Info().
		Str("name", packet.Name).
		Str("account", session.AccountName).
		Int32("race", packet.Race).
		Int32("sex", packet.Sex).
		Int32("class_id", packet.ClassID).
		Msg("Character creation request")

	// Create character using use case
	req := &models.CharacterCreateRequest{
		AccountName: session.AccountName,
		Name:        packet.Name,
		Race:        int(packet.Race),
		Sex:         int(packet.Sex),
		ClassID:     int(packet.ClassID),
		HairStyle:   int(packet.HairStyle),
		HairColor:   int(packet.HairColor),
		Face:        int(packet.Face),
	}

	character, err := h.characterUseCase.CreateCharacter(ctx, req)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Str("name", packet.Name).Msg("character creation failed")

		// Send failure response
		if err := c.Send(outclient.NewCharCreateOk(false)); err != nil {
			return err
		}
		return nil
	}

	log.Ctx(ctx).Info().
		Str("name", character.Name).
		Int32("char_id", character.ID).
		Msg("Character created successfully")

	// Send success response. ВАЖНО: после CharCreateOk НЕ отправляем CharSelectionInfo.
	// L2J (CharacterCreate.initNewChar) лишь кеширует список (setCharSelection), но НЕ
	// шлёт его. Клиент сам пришлёт RequestGotoLobby, и список уйдёт в ответ на него.
	// Лишний CharSelectionInfo здесь оставляет клиент в некорректном состоянии и
	// приводит к дисконнекту при возврате в лобби после создания персонажа.
	if err := c.Send(outclient.NewCharCreateOk(true)); err != nil {
		return err
	}
	return nil
}

// handleCharacterDelete processes character deletion
func (h *Handler) handleCharacterDelete(ctx context.Context, c *client.ClientConn, payload []byte) error {
	// Get session to access account name
	session := h.getSession(c)
	if session == nil {
		log.Ctx(ctx).Error().Msg("no session found for character deletion")
		return nil
	}

	packet := &inclient.CharacterDelete{}
	l2pkt.ParsePacket(payload, packet)

	log.Ctx(ctx).Info().
		Int32("slot", packet.CharacterSlot).
		Str("account", session.AccountName).
		Msg("Character deletion request")

	// Load character list to get character ID from slot
	characters, err := h.characterUseCase.GetCharacterList(ctx, session.AccountName)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to load character list for deletion")
		return err
	}

	if packet.CharacterSlot >= int32(len(characters)) || packet.CharacterSlot < 0 {
		log.Ctx(ctx).Warn().
			Int32("slot", packet.CharacterSlot).
			Int("character_count", len(characters)).
			Msg("invalid character slot for deletion")
		return nil
	}

	characterToDelete := characters[packet.CharacterSlot]

	// Delete character using use case
	err = h.characterUseCase.DeleteCharacter(ctx, characterToDelete.ID, session.AccountName)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).
			Str("name", characterToDelete.Name).
			Int32("char_id", characterToDelete.ID).
			Msg("character deletion failed")
		return err
	}

	log.Ctx(ctx).Info().
		Str("name", characterToDelete.Name).
		Int32("char_id", characterToDelete.ID).
		Msg("Character marked for deletion")

	// Send updated character list after deletion
	return h.sendUpdatedCharacterList(ctx, c, session)
}

// handleCharacterSelect processes character selection
func (h *Handler) handleCharacterSelect(ctx context.Context, c *client.ClientConn, payload []byte) error {
	// Get session to access account name
	session := h.getSession(c)
	if session == nil {
		log.Ctx(ctx).Error().Msg("no session found for character selection")
		return nil
	}

	packet := &inclient.CharacterSelect{}
	l2pkt.ParsePacket(payload, packet)

	log.Ctx(ctx).Info().
		Int32("char_id", packet.CharID).
		Str("account", session.AccountName).
		Msg("Character selection request")

	// Validate character ownership directly by ID
	validChar, err := h.characterUseCase.SelectCharacter(ctx, packet.CharID, session.AccountName)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).
			Int32("char_id", packet.CharID).
			Msg("character selection validation failed")
		return err
	}

	log.Ctx(ctx).Info().
		Str("name", validChar.Name).
		Int32("char_id", validChar.ID).
		Msg("Character selected successfully")

	// CRITICAL FIX: Add to world registry BEFORE sending CharSelected (Java L2J order)
	// This ensures proper character initialization and running state
	if err := h.world.AddPlayer(ctx, validChar); err != nil {
		log.Ctx(ctx).Error().Err(err).
			Int32("char_id", validChar.ID).
			Msg("failed to add character to world registry")
		return err
	}
	
	// Initialize character state (Java L2J: setRunning(), standUp(), etc.)
	if err := h.world.UpdatePlayerRunWalkState(ctx, validChar.ID, true); err != nil {
		log.Ctx(ctx).Warn().Err(err).
			Int32("char_id", validChar.ID).
			Msg("Failed to set initial running state, but continuing")
	} else {
		log.Ctx(ctx).Debug().
			Int32("char_id", validChar.ID).
			Bool("is_running", true).
			Msg("Character initialized with running state (Java L2J default)")
	}

	// Send character selection confirmation AFTER world registration
	charSelectedData := h.buildCharSelectedPacket(validChar)
	if err := c.Send(charSelectedData); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to send CharSelected")
		return err
	}

	return nil
}

// handleRequestGotoLobby processes return to character selection (after character creation)
func (h *Handler) handleRequestGotoLobby(ctx context.Context, c *client.ClientConn, payload []byte) error {
	log.Ctx(ctx).Info().Msg("RequestGotoLobby - returning to character selection")

	// Get session to access account name
	session := h.getSession(c)
	if session == nil {
		log.Ctx(ctx).Error().Msg("no session found for lobby return")
		return nil
	}

	// Parse the packet to ensure it's valid
	packet, err := inclient.ParseRequestGotoLobby(payload)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to parse RequestGotoLobby packet")
		return err
	}

	log.Ctx(ctx).Debug().
		Str("packet", packet.String()).
		Str("account", session.AccountName).
		Msg("RequestGotoLobby packet parsed successfully")

	// Important: Remove player from world if they are in it
	if playerState, exists := h.world.GetPlayerByAccount(session.AccountName); exists {
		log.Ctx(ctx).Info().
			Int32("char_id", playerState.CharID).
			Str("account", session.AccountName).
			Msg("Removing player from world before lobby return")
		
		// Remove player from world registry
		if err := h.world.RemovePlayer(ctx, playerState.CharID); err != nil {
			log.Ctx(ctx).Error().Err(err).Msg("failed to remove player from world")
			// Continue anyway - don't fail the lobby return
		}
	}

	// Send updated character list (same as Java L2J does with CharSelectionInfo)
	return h.sendUpdatedCharacterList(ctx, c, session)
}

func (h *Handler) handleRequestShortCutReg(ctx context.Context, c *client.ClientConn, payload []byte) error {
	packet := &inclient.ShortcutRegisterRequest{}
	l2pkt.ParsePacket(payload, packet)

	log.Ctx(ctx).Info().
		Int32("type", int32(packet.Type)).
		Int32("slot", packet.Slot).
		Int32("page", packet.Page).
		Int32("id", packet.Id).
		Int32("level", packet.Level).
		Int32("character_type", packet.CharacterType).
		Msg("RequestShortCutReg received")

	return nil
}

// paperdollSlotToPacketIndex maps DB paperdoll slot (models.PaperdollSlot) to the
// index in CharSelectionInfo's PaperdollItemIDs array (outclient.PaperdollOrder).
// The DB stores items with LocData = PaperdollSlot value, but the packet writes
// them in a different order matching the client's expected PaperdollOrder.
var paperdollSlotToPacketIndex = map[int]int{
	int(models.SlotUnder):     0,  // under
	int(models.SlotREar):      1,  // rear
	int(models.SlotLEar):      2,  // lear
	int(models.SlotNeck):      3,  // neck
	int(models.SlotRFinger):   4,  // rfinger
	int(models.SlotLFinger):   5,  // lfinger
	int(models.SlotHead):      6,  // head
	int(models.SlotRHand):     7,  // rhand
	int(models.SlotLHand):     8,  // lhand
	int(models.SlotGloves):    9,  // gloves
	int(models.SlotChest):     10, // chest
	int(models.SlotLegs):      11, // legs
	int(models.SlotFeet):      12, // feet
	int(models.SlotBack):      13, // back
	int(models.SlotLRHand):    14, // lrhand
	int(models.SlotHair):      15, // hair
	int(models.SlotHair2):     16, // hair2
	int(models.SlotRBracelet): 17, // rbracelet
	int(models.SlotLBracelet): 18, // lbracelet
	int(models.SlotDeco1):     19, // deco1
	int(models.SlotDeco2):     20, // deco2
	int(models.SlotDeco3):     21, // deco3
	int(models.SlotDeco4):     22, // deco4
	int(models.SlotDeco5):     23, // deco5
	int(models.SlotDeco6):     24, // deco6
	int(models.SlotBelt):      25, // belt
}

// buildPaperdollItemIDs converts loaded paperdoll items from the DB into
// the 26-element ItemID array expected by CharSelectionInfo packet.
func buildPaperdollItemIDs(paperdollItems []models.CharacterItem) []int32 {
	itemIDs := make([]int32, 26)
	for _, item := range paperdollItems {
		if idx, ok := paperdollSlotToPacketIndex[item.LocData]; ok {
			itemIDs[idx] = item.ItemID
		}
	}
	return itemIDs
}

// sendUpdatedCharacterList reloads and sends the character list
func (h *Handler) sendUpdatedCharacterList(ctx context.Context, c *client.ClientConn, session *ClientSession) error {
	// Reload character list
	characters, err := h.characterUseCase.GetCharacterListEntries(ctx, session.AccountName)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to load character list")
		return err
	}

	// Convert domain models to packet format (FIXED: match auth.go exactly)
	chars := make([]outclient.CharSelectInfoPackage, len(characters))
	for i, char := range characters {
		// CharacterListEntry has embedded Character, so we can access fields directly
		chars[i] = outclient.CharSelectInfoPackage{
			Name:             char.Name,
			ObjectID:         char.ID,
			ClanID:           int32(char.ClanID),
			Sex:              int32(char.Sex),
			Race:             int32(char.Race),
			BaseClassID:      int32(char.BaseClass),
			ClassID:          int32(char.ClassID),
			X:                int32(char.Position.X),
			Y:                int32(char.Position.Y),
			Z:                int32(char.Position.Z),
			CurrentHp:        char.CurrentHP,
			CurrentMp:        char.CurrentMP,
			MaxHp:            float64(char.MaxHP),
			MaxMp:            float64(char.MaxMP),
			Sp:               int32(char.SP),
			Exp:              char.Experience,
			Level:            int32(char.Level),
			Karma:            int32(char.Karma),
			PkKills:          int32(char.PKKills),
			PvPKills:         int32(char.PvPKills),
			HairStyle:        int32(char.HairStyle),
			HairColor:        int32(char.HairColor),
			Face:             int32(char.Face),
			DeleteTimerMs:    char.DeleteTime,
			LastAccessMs:     char.LastAccess,
			VitalityPoints:   int32(char.VitalityPoints),
			PaperdollItemIDs: buildPaperdollItemIDs(char.PaperdollItems),
		}
	}

	// Send updated character list
	charSelectionInfo := outclient.CharSelectionInfo{
		LoginName: session.AccountName,
		SessionID: int32(session.SessionID),
		ActiveIdx: -1, // No character selected
		Chars:     chars,
		CharConf: outclient.CharacterConfig{
			CharMaxNumber: 7,
		},
	}

	if err := c.Send(l2pkt.BuildPacket(charSelectionInfo)); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to send updated character list")
		return err
	}

	log.Ctx(ctx).Info().
		Str("account", session.AccountName).
		Int("character_count", len(characters)).
		Msg("Updated character list sent successfully")

	return nil
}

func (h *Handler) handleRequestItemList(ctx context.Context, c *client.ClientConn, payload []byte) error {
	log.Ctx(ctx).Info().
		Msg("RequestItemList received")

	// Get session to access account name
	session := h.getSession(c)
	if session == nil {
		log.Ctx(ctx).Error().Msg("no session found for item list request")
		return nil
	}

	playerState, exists := h.world.GetPlayerByAccount(session.AccountName)
	if !exists {
		log.Ctx(ctx).Error().
			Str("account", session.AccountName).
			Msg("player not found in world for EnterWorld")
		return nil
	}

	itemlist := h.buildItemListPacket(ctx, playerState.Character)

	c.Send(itemlist)

	return nil
}
