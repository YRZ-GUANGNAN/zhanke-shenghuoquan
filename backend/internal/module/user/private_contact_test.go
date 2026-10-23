package user

import (
	"context"
	"errors"
	"testing"
)

type privateContactStub struct {
	*domainARepositoryStub
	blockError error
}

func (r *privateContactStub) HasBlockBetween(_ context.Context, a, b uint) (bool, error) {
	return r.blocks[a][b] || r.blocks[b][a], r.blockError
}

func TestPrivateContactActivePairAndBidirectionalBlocks(t *testing.T) {
	r := &privateContactStub{domainARepositoryStub: newDomainARepositoryStub(User{ID: 2, Status: StatusActive}, User{ID: 8, Status: StatusActive})}
	s := NewService(r, "secret", "", "", false)
	ctx := context.Background()
	if err := s.ValidatePrivateContact(ctx, 2, 8); err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]uint{{0, 8}, {2, 0}, {2, 2}, {2, 99}} {
		if !errors.Is(s.ValidatePrivateContact(ctx, pair[0], pair[1]), ErrPrivateContactUnavailable) {
			t.Fatal("invalid pair accepted")
		}
	}
	for _, id := range []uint{2, 8} {
		for _, status := range []Status{StatusBanned, StatusClosed, StatusMuted} {
			r.users[id] = User{ID: id, Status: status}
			if !errors.Is(s.ValidatePrivateContact(ctx, 2, 8), ErrPrivateContactUnavailable) {
				t.Fatal("unavailable account accepted")
			}
		}
		r.users[id] = User{ID: id, Status: StatusActive}
	}
	for _, pair := range [][2]uint{{2, 8}, {8, 2}} {
		r.blocks[pair[0]] = map[uint]bool{pair[1]: true}
		if !errors.Is(s.ValidatePrivateContact(ctx, 2, 8), ErrPrivateContactUnavailable) {
			t.Fatal("blocked contact accepted")
		}
		delete(r.blocks, pair[0])
	}
	r.blockError = errors.New("storage unavailable")
	if err := s.ValidatePrivateContact(ctx, 2, 8); !errors.Is(err, r.blockError) {
		t.Fatal("storage failure allowed contact")
	}
}
