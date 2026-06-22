package media

import (
	"bytes"
	"testing"
	"time"
)

func TestNewULIDAtProducesCanonicalSortableValue(t *testing.T) {
	first, err := NewULIDAt(time.UnixMilli(1_700_000_000_000), bytes.NewReader(make([]byte, 10)))
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewULIDAt(time.UnixMilli(1_700_000_000_001), bytes.NewReader(make([]byte, 10)))
	if err != nil {
		t.Fatal(err)
	}
	if !ValidULID(first) || !ValidULID(second) {
		t.Fatalf("generated invalid ULIDs: %q %q", first, second)
	}
	if first >= second {
		t.Fatalf("ULIDs are not time sortable: %q >= %q", first, second)
	}
}

func TestValidULIDRejectsInvalidAlphabetAndOverflow(t *testing.T) {
	for _, value := range []string{"", "01ARZ3NDEKTSV4RRFFQ69G5FAI", "81ARZ3NDEKTSV4RRFFQ69G5FAV", "01arz3ndektsv4rrffq69g5fav"} {
		if ValidULID(value) {
			t.Fatalf("accepted invalid ULID %q", value)
		}
	}
}
