package featureflags

import (
	"errors"
	"testing"

	"campus-circle/backend/internal/module/user"
)

func TestPolicyDefaultsAreSafeAndLabIsExplicit(t *testing.T) {
	Configure(false)
	value := Snapshot()
	if value.LabEnabled || value.PostingMode != "admin_only" {
		t.Fatalf("unexpected default policy: %+v", value)
	}
	if !Can(KeyForum) || !CanPost(user.RoleUser) {
		t.Fatal("disabled lab must preserve the existing production behavior")
	}
}

func TestPolicyUpdateUsesRowVersionAndDoesNotChangeRoles(t *testing.T) {
	Configure(true)
	t.Cleanup(func() { Configure(false) })
	value := Snapshot()
	value.Flags[KeySecondhand] = false
	value, err := Update(9, UpdateInput{Flags: value.Flags, PostingMode: "all_users", ExpectedRowVersion: value.RowVersion})
	if err != nil {
		t.Fatal(err)
	}
	if !value.Flags[KeyForum] || value.Flags[KeySecondhand] || !CanPost(user.RoleUser) {
		t.Fatalf("unexpected updated policy: %+v", value)
	}
	if _, err := Update(9, UpdateInput{Flags: value.Flags, PostingMode: value.PostingMode, ExpectedRowVersion: 1}); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("expected row version conflict, got %v", err)
	}
	Configure(false)
}
