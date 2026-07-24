package adminauth

import (
	"errors"
	"strings"
	"testing"
)

func TestPasswordPolicyAndArgon2id(t *testing.T) {
	for _, value := range []string{strings.Repeat("a", 13), strings.Repeat("a", 73)} {
		if _, err := hashPassword(value); !errors.Is(err, ErrPasswordPolicy) {
			t.Fatalf("hashPassword(%d runes) error=%v", len([]rune(value)), err)
		}
	}
	password := "a-correct-password-14"
	first, err := hashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	second, err := hashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("argon2id hashes must use random salts")
	}
	if !strings.HasPrefix(first, "$argon2id$v=19$m=65536,t=3,p=2$") {
		t.Fatalf("unexpected argon2id parameters: %s", first)
	}
	if !verifyPassword(first, password) || verifyPassword(first, password+"x") {
		t.Fatal("password verification mismatch")
	}
	if verifyPassword("invalid", password) {
		t.Fatal("invalid encoded hash must fail closed")
	}
}
