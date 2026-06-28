package outls

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// checkGolden фиксирует байтовый вывод пакета (характеризующий тест).
// Запусти с UPDATE_GOLDEN=1 чтобы (пере)сгенерировать эталоны.
func checkGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run UPDATE_GOLDEN=1 to create)", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s bytes mismatch\n got: %x\nwant: %x", name, got, want)
	}
}

func TestAuthRequest(t *testing.T) {
	p := NewAuthRequest(1, true, []byte{0xDE, 0xAD, 0xBE, 0xEF}, 7777, false, 100,
		[]string{"192.168.0.0/16", "10.0.0.0/8"}, []string{"127.0.0.1"})
	checkGolden(t, "authrequest", p.GetData())
}

func TestPlayerAuthRequest(t *testing.T) {
	p := NewPlayerAuthRequest("test_account", SessionKey{
		PlayOkID1: 0x11111111, PlayOkID2: 0x22222222,
		LoginOkID1: 0x33333333, LoginOkID2: 0x44444444,
	})
	checkGolden(t, "playerauthrequest", p.GetData())
}

func TestChangePassword(t *testing.T) {
	p := NewChangePassword("acc", "char", "oldpw", "newpw")
	checkGolden(t, "changepassword", p.GetData())
}

func TestPlayerInGame(t *testing.T) {
	checkGolden(t, "playeringame_single", NewPlayerInGame("Hero").GetData())
	checkGolden(t, "playeringame_multi", NewPlayerInGameMultiple([]string{"Hero", "Mage", "Tank"}).GetData())
}

func TestPlayerLogout(t *testing.T) {
	checkGolden(t, "playerlogout", NewPlayerLogout("acc").GetData())
}

func TestPlayerTracert(t *testing.T) {
	p := NewPlayerTracert("acc", "1.2.3.4", "h1", "h2", "h3", "h4")
	checkGolden(t, "playertracert", p.GetData())
}

func TestReplyCharacters(t *testing.T) {
	p := NewReplyCharacters("acc", 5, 2)
	checkGolden(t, "replycharacters", p.GetData())
}

func TestServerStatus(t *testing.T) {
	p := NewServerStatus(1, 0, 7777, 100, 1, 1, 80, 18, true, false, true, false)
	checkGolden(t, "serverstatus", p.GetData())
}
