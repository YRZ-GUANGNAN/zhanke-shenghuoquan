package media

import (
	"crypto/sha1" // #nosec G505 -- WeChat's callback protocol mandates SHA-1.
	"encoding/hex"
)

func sha1Hex(value string) string {
	sum := sha1.Sum([]byte(value)) // #nosec G401 -- protocol compatibility, not password hashing.
	return hex.EncodeToString(sum[:])
}
