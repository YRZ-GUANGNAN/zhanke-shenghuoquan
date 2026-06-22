package takeout

import (
	"crypto/rand"
	"errors"
	"io"
	"math/big"
	"strings"
	"time"
)

const crockfordAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

var ErrInvalidULID = errors.New("invalid ULID")

func NewULID() (string, error) { return NewULIDAt(time.Now().UTC(), rand.Reader) }

func NewULIDAt(at time.Time, randomness io.Reader) (string, error) {
	if randomness == nil {
		return "", ErrInvalidULID
	}
	milliseconds := at.UnixMilli()
	if milliseconds < 0 || milliseconds > (1<<48)-1 {
		return "", ErrInvalidULID
	}
	raw := make([]byte, 16)
	for index := 5; index >= 0; index-- {
		raw[index] = byte(milliseconds)
		milliseconds >>= 8
	}
	if _, err := io.ReadFull(randomness, raw[6:]); err != nil {
		return "", err
	}
	value := new(big.Int).SetBytes(raw)
	base := big.NewInt(32)
	rem := new(big.Int)
	encoded := make([]byte, 26)
	for index := len(encoded) - 1; index >= 0; index-- {
		value.QuoRem(value, base, rem)
		encoded[index] = crockfordAlphabet[rem.Int64()]
	}
	return string(encoded), nil
}

func ValidULID(value string) bool {
	if len(value) != 26 || value[0] > '7' {
		return false
	}
	for _, character := range value {
		if !strings.ContainsRune(crockfordAlphabet, character) {
			return false
		}
	}
	return true
}
