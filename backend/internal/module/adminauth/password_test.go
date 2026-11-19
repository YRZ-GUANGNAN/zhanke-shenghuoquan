package adminauth

import (
	"errors"
	"strings"
	"testing"
)

func TestPasswordPolicyAndArgon2id(t *testing.T) {
	if _, err := hashPassword(""); !errors.Is(err, ErrPasswordPolicy) {
		t.Fatalf("empty password must be rejected: %v", err)
	}
	// 长度不设限制：极短与超长口令都能正常哈希与验证。
	for _, value := range []string{"a", strings.Repeat("a", 200)} {
		encoded, err := hashPassword(value)
		if err != nil || !verifyPassword(encoded, value) {
			t.Fatalf("hashPassword(%d runes) err=%v", len([]rune(value)), err)
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
