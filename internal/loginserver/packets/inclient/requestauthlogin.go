package inclient

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"unicode"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/loginserver/packets"
	"github.com/VerTox/l2go/internal/loginserver/transport"
)

type RequestAuthLogin struct {
	Username string
	Password string
}

func NewRequestAuthLogin(ctx context.Context, client *transport.Client, request []byte) (*RequestAuthLogin, error) {
	var result RequestAuthLogin

	dec128, err := packets.RsaDecryptNoPadding(client.PrivateKey, request[:128])
	if err != nil {
		log.Ctx(ctx).Error().Msgf("Error decrypting request: %v\n", err)

		return nil, err
	}

	user, pass, err := extractCredsFromDecrypted128(dec128)
	if err != nil {
		log.Ctx(ctx).Error().Msgf("Error extracting credentials: %v\n", err)

		return nil, err
	}
	log.Ctx(ctx).Debug().Msgf("user=%q pass=%q\n", user, pass)

	result.Username = user
	result.Password = pass

	return &result, nil
}

func extractCredsFromDecrypted128(dec []byte) (user, pass string, err error) {
	if len(dec) < 128 {
		return "", "", errors.New("decrypted block must be 128 bytes")
	}
	userRaw := dec[0x5E : 0x5E+14]
	passRaw := dec[0x6C : 0x6C+16]

	user = strings.ToLower(trimASCII(userRaw))
	pass = trimASCII(passRaw)

	if !plausibleUser(user) {
		return "", "", errors.New("username not plausible")
	}
	if !plausiblePass(pass) {
		return "", "", errors.New("password not plausible")
	}
	return user, pass, nil
}

func trimASCII(b []byte) string {
	return string(bytes.Trim(b, "\x00 \t\r\n"))
}

func plausibleUser(s string) bool {
	if l := len(s); l == 0 || l > 14 {
		return false
	}
	for _, r := range s {
		if r < 0x20 || r >= 0x7f {
			return false
		}
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

func plausiblePass(s string) bool {
	if l := len(s); l < 3 || l > 16 {
		return false
	}
	for _, r := range s {
		if r < 0x20 || r >= 0x7f {
			return false
		}
	}
	return true
}
