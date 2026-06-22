package takeout

import (
	"bytes"
	"testing"
	"time"
)

func TestULIDRoundTripAndValidation(t *testing.T) {
	value, err := NewULIDAt(time.UnixMilli(1_750_000_000_000).UTC(), bytes.NewReader(make([]byte, 10)))
	if err != nil {
		t.Fatal(err)
	}
	if !ValidULID(value) || len(value) != 26 {
		t.Fatalf("invalid generated ULID %q", value)
	}
	for _, invalid := range []string{"", "01J0000000000000000000000I", "81J00000000000000000000000", "01j00000000000000000000000"} {
		if ValidULID(invalid) {
			t.Errorf("accepted invalid ULID %q", invalid)
		}
	}
}

func TestULIDRejectsInvalidInputs(t *testing.T) {
	if _, err := NewULIDAt(time.Unix(-1, 0), bytes.NewReader(make([]byte, 10))); err != ErrInvalidULID {
		t.Fatalf("negative time returned %v", err)
	}
	if _, err := NewULIDAt(time.Now(), nil); err != ErrInvalidULID {
		t.Fatalf("nil randomness returned %v", err)
	}
}
