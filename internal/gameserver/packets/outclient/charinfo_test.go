package outclient

import (
	"encoding/binary"
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// charInfoCursor walks a CharInfo packet body per the L2J High Five structure so
// tests can assert both field VALUES and byte ALIGNMENT. If the builder writes a
// field with the wrong width, every field decoded after it shifts and the asserts
// fail — which is exactly how we catch misalignment bugs.
type charInfoCursor struct {
	b []byte
	p int
}

func (c *charInfoCursor) readC() uint8 { v := c.b[c.p]; c.p++; return v }
func (c *charInfoCursor) readH() uint16 {
	v := binary.LittleEndian.Uint16(c.b[c.p:])
	c.p += 2
	return v
}
func (c *charInfoCursor) readD() int32 {
	v := int32(binary.LittleEndian.Uint32(c.b[c.p:]))
	c.p += 4
	return v
}
func (c *charInfoCursor) skipD(n int) { c.p += 4 * n }
func (c *charInfoCursor) skipF(n int) { c.p += 8 * n }
func (c *charInfoCursor) skipS() {
	for binary.LittleEndian.Uint16(c.b[c.p:]) != 0 {
		c.p += 2
	}
	c.p += 2 // null terminator
}

// decodedCharInfo holds the fields we assert on.
type decodedCharInfo struct {
	standing   uint8
	nameColor  int32
	heading    int32
	titleColor int32
}

func decodeCharInfo(t *testing.T, data []byte) decodedCharInfo {
	t.Helper()
	c := &charInfoCursor{b: data}

	if op := c.readC(); op != 0x31 {
		t.Fatalf("opcode = 0x%x, want 0x31", op)
	}
	c.skipD(5) // x, y, z, vehicle, objId
	c.skipS()  // name
	c.skipD(3) // race, sex, classId
	c.skipD(21) // paperdoll display
	c.skipD(21) // paperdoll augment
	c.skipD(15) // talisman, cloak, pvp, karma, mAtk, pAtk, unknown, run, walk, swimRun, swimWalk, flyRun, flyWalk, flyRun, flyWalk
	c.skipF(4)  // moveMult, atkMult, collisionRadius, collisionHeight
	c.skipD(3)  // hairStyle, hairColor, face
	c.skipS()   // title
	c.skipD(4)  // clanId, clanCrest, allyId, allyCrest

	out := decodedCharInfo{}
	out.standing = c.readC() // L2J: standing = 1, sitting = 0
	c.readC()                // running
	c.readC()                // inCombat
	c.readC()                // dead
	c.readC()                // invisible
	c.readC()                // mountType
	c.readC()                // privateStore

	cubics := c.readH()
	c.p += 2 * int(cubics) // cubic entries

	c.readC()   // partyMatch
	c.skipD(1)  // abnormal
	c.readC()   // zone
	c.readH()   // recHave
	c.skipD(3)  // mountNpcId, classId2, unknown
	c.readC()   // enchant
	c.readC()   // team
	c.skipD(1)  // clanCrestLarge
	c.readC()   // noble
	c.readC()   // hero
	c.readC()   // fishing (MUST be 1 byte — L2J writeC)
	c.skipD(3)  // fishX, fishY, fishZ

	out.nameColor = c.readD()
	out.heading = c.readD()
	c.skipD(2) // pledgeClass, pledgeType
	out.titleColor = c.readD()

	return out
}

// TestBuildCharInfo_AlignmentAndStanding verifies the two wire bugs: the fishing
// flag width (which shifts name/title colors and heading) and the inverted
// standing flag.
func TestBuildCharInfo_AlignmentAndStanding(t *testing.T) {
	info := CharInfo{
		Name:       "Hero",
		Title:      "TT",
		Sitting:    0, // player is standing
		FishingFlag: 0,
		NameColor:  0x00AABB,
		TitleColor: 0x112233,
		Heading:    0x4000,
		Cubics:     []int32{},
	}

	got := decodeCharInfo(t, BuildCharInfo(info))

	if got.standing != 1 {
		t.Errorf("standing byte = %d, want 1 (L2J: standing=1, sitting=0)", got.standing)
	}
	if got.nameColor != 0x00AABB {
		t.Errorf("nameColor = 0x%06X, want 0x00AABB (misaligned?)", got.nameColor)
	}
	if got.heading != 0x4000 {
		t.Errorf("heading = 0x%X, want 0x4000 (misaligned?)", got.heading)
	}
	if got.titleColor != 0x112233 {
		t.Errorf("titleColor = 0x%06X, want 0x112233 (misaligned?)", got.titleColor)
	}
}

// TestBuildCharInfo_SittingPlayer verifies a sitting player encodes as 0.
func TestBuildCharInfo_SittingPlayer(t *testing.T) {
	got := decodeCharInfo(t, BuildCharInfo(CharInfo{Sitting: 1, Cubics: []int32{}}))
	if got.standing != 0 {
		t.Errorf("standing byte = %d, want 0 for a sitting player", got.standing)
	}
}

// TestNewCharInfo_HeadingPassthrough verifies the heading is taken from the caller
// rather than hardcoded to 0.
func TestNewCharInfo_HeadingPassthrough(t *testing.T) {
	char := &models.Character{ID: 1, Name: "Hero", Race: 0, Sex: 1, ClassID: 0}
	pos := &models.Position{X: 10, Y: 20, Z: 30}

	ci := NewCharInfo(char, pos, nil, true, false, 0x7FFF)
	got := decodeCharInfo(t, BuildCharInfo(*ci))

	if got.heading != 0x7FFF {
		t.Errorf("heading = 0x%X, want 0x7FFF (passed through NewCharInfo)", got.heading)
	}
	if got.standing != 1 {
		t.Errorf("standing byte = %d, want 1 for a standing player", got.standing)
	}
}
