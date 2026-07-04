# GameServer Client Packet Mappings

Verified against Java L2J reference implementation in `/l2jserver/`.

## Incoming Client Packets (Client -> GameServer)

| Packet | Opcode | File | Purpose |
|---|---|---|---|
| ProtocolVersion | `0x0e` | `protocolversion.go` | Protocol negotiation |
| AuthLogin | `0x2b` | `authlogin.go` | Authentication with session keys |
| NewCharacter | `0x13` | `newcharacter.go` | Request character templates |
| CharacterCreate | `0x0c` | `charactercreate.go` | Create character |
| CharacterSelect | `0x12` | `characterselect.go` | Select character |
| CharacterDelete | `0x0d` | `characterdelete.go` | Delete character |
| EnterWorld | `0x11` | `enterworld.go` | Enter world |
| Action | `0x1f` | `action.go` | Target NPC/player |
| MoveBackwardToLocation | `0x01` | `movebackwardtolocation.go` | Movement |
| ValidatePosition | `0x48` | `validateposition.go` | Position sync |
| RequestTargetCancel | `0x48` | `requesttargetcancel.go` | Deselect target |
| Logout | `0x00` | `logout.go` | Logout |
| RequestRestart | `0x57` | `requestrestart.go` | Restart |
| UseItem | `0x19` | `useitem.go` | Use item |
| RequestUnEquipItem | `0x16` | `requestunequipitem.go` | Unequip item |
| RequestActionUse | `0x56` | `requestactionuse.go` | Action use (Walk/Run) |

### Multi-Packet (0xD0) Sub-Opcodes

| Sub-Opcode | Packet | Purpose |
|---|---|---|
| `0x0d` | RequestAutoSoulShot | Auto-shot config |
| `0x21` | RequestKeyMapping | Get key bindings |
| `0x22` | RequestSaveKeyMapping | Save key bindings |
| `0x36` | RequestGotoLobby | Return to lobby |

## Outgoing Server Packets (GameServer -> Client)

| Packet | Opcode | File | Purpose |
|---|---|---|---|
| KeyPacket | `0x2e` | `keypacket.go` | Encryption key exchange |
| CharSelectionInfo | `0x09` | `charselectioninfo.go` | Character list |
| NewCharacterSuccess | `0x0d` | `newcharactersuccess.go` | Templates response |
| CharCreateOk | `0x0f` | `charcreateok.go` | Creation result |
| CharSelected | `0x0b` | `charselected.go` | Selected character data |
| UserInfo | `0x32` | `userinfo.go` | Own character info |
| CharInfo | `0x31` | `charinfo.go` | Other player info |
| NpcInfo | `0x0c` | `npcinfo.go` | NPC display |
| MoveToLocation | `0x2f` | `movetolocation.go` | Movement |
| MyTargetSelected | `0xb9` | `mytargetselected.go` | Target confirmed |
| TargetUnselected | `0x24` | `targetunselected.go` | Target cleared |
| ActionFailed | `0x25` | `actionfailed.go` | Action failed |
| StatusUpdate | `0x18` | `statusupdate.go` | HP/MP bar update |
| NpcHtmlMessage | `0x19` | `npchtmlmessage.go` | NPC dialogue window |
| MoveToPawn | `0x72` | `movetopawn.go` | Move/face toward target |
| DeleteObject | `0x08` | `deleteobject.go` | Remove object |
| LeaveWorld | `0x84` | `leaveworld.go` | Disconnect |
| ItemList | `0x11` | `itemlist.go` | Inventory |
| ExBasicActionList | `0xFE5F` | `exbasicactionlist.go` | Action list |

## Packet Flow

```
Client -> GS: ProtocolVersion (0x0e)
GS -> Client: KeyPacket (0x2e)
Client -> GS: AuthLogin (0x2b)
GS -> Client: CharSelectionInfo (0x09)
Client -> GS: CharacterSelect (0x12)
GS -> Client: CharSelected (0x0b)
Client -> GS: EnterWorld (0x11)
GS -> Client: UserInfo, ItemList, SkillList, ExBasicActionList, NpcInfo...
```
