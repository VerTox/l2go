package client

import (
	"errors"
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
)

func TestCharCreateFailReason(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int32
	}{
		{"name exists", models.ErrCharacterExists, outclient.CharCreateFailReasonNameExists},
		{"invalid name", models.ErrInvalidCharacterName, outclient.CharCreateFailReasonIncorrectName},
		{"too many", models.ErrTooManyCharacters, outclient.CharCreateFailReasonTooManyChars},
		{"invalid race → generic", models.ErrInvalidRace, outclient.CharCreateFailReasonCreationFailed},
		{"unknown → generic", errors.New("boom"), outclient.CharCreateFailReasonCreationFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := charCreateFailReason(tc.err); got != tc.want {
				t.Fatalf("charCreateFailReason(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}
