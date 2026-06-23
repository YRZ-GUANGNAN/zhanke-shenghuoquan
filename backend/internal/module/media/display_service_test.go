package media

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeDisplayGateway struct {
	err error
}

func (f fakeDisplayGateway) BuildDisplayView(context.Context, Purpose, string, uint16) (MediaDisplayView, error) {
	if f.err != nil {
		return MediaDisplayView{}, f.err
	}
	return MediaDisplayView{Media: MediaMetadataView{MediaID: "01H00000000000000000000000"}}, nil
}

type fakeDisplayRepository struct {
	value DisplayObjectIdentity
	err   error
}

func (f fakeDisplayRepository) FindVisibleDisplayObjectByReference(context.Context, Purpose, string, uint16) (DisplayObjectIdentity, error) {
	if f.err != nil {
		return DisplayObjectIdentity{}, f.err
	}
	return f.value, nil
}

type fakeDisplaySigner struct {
	key       string
	versionID string
	ttl       time.Duration
	called    bool
	err       error
}

func (f *fakeDisplaySigner) SignedGetURL(_ context.Context, key, versionID string, ttl time.Duration) (string, error) {
	f.called = true
	f.key = key
	f.versionID = versionID
	f.ttl = ttl
	if f.err != nil {
		return "", f.err
	}
	return "https://signed.example.test/content?token=short", nil
}

func TestMediaDisplayServiceSignsVisibleFinalObject(t *testing.T) {
	signer := &fakeDisplaySigner{}
	service, err := NewMediaDisplayService(
		fakeDisplayGateway{},
		fakeDisplayRepository{value: DisplayObjectIdentity{MediaID: "01H00000000000000000000000", FinalKey: "final/a.webp", FinalVersionID: "ver-1"}},
		signer,
		MediaDisplayURLTTL,
	)
	if err != nil {
		t.Fatal(err)
	}
	value, err := service.ContentURL(context.Background(), PurposeDish, "dish-1", 0)
	if err != nil || value == "" {
		t.Fatalf("url=%q err=%v", value, err)
	}
	if !signer.called || signer.key != "final/a.webp" || signer.versionID != "ver-1" || signer.ttl != 5*time.Minute {
		t.Fatalf("signer=%#v", signer)
	}
}

func TestMediaDisplayServiceFailClosedBeforeSigning(t *testing.T) {
	signer := &fakeDisplaySigner{}
	service, err := NewMediaDisplayService(
		fakeDisplayGateway{err: ErrMediaNotVisible},
		fakeDisplayRepository{value: DisplayObjectIdentity{FinalKey: "final/a.webp", FinalVersionID: "ver-1"}},
		signer,
		MediaDisplayURLTTL,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ContentURL(context.Background(), PurposeDish, "dish-1", 0); !errors.Is(err, ErrMediaNotVisible) {
		t.Fatalf("err=%v want ErrMediaNotVisible", err)
	}
	if signer.called {
		t.Fatal("non-visible media must not be signed")
	}
}

func TestMediaDisplayServiceRechecksVisibleIdentity(t *testing.T) {
	signer := &fakeDisplaySigner{}
	service, err := NewMediaDisplayService(
		fakeDisplayGateway{},
		fakeDisplayRepository{err: ErrNotFound},
		signer,
		MediaDisplayURLTTL,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ContentURL(context.Background(), PurposeDish, "dish-1", 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err=%v want ErrNotFound", err)
	}
	if signer.called {
		t.Fatal("TOCTOU repository miss must not be signed")
	}
}

func TestMediaDisplayServiceRejectsInvalidReference(t *testing.T) {
	service, err := NewMediaDisplayService(fakeDisplayGateway{}, fakeDisplayRepository{}, &fakeDisplaySigner{}, MediaDisplayURLTTL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ContentURL(context.Background(), Purpose("invalid"), "dish-1", 0); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("err=%v want ErrInvalidRecord", err)
	}
	if _, err := service.ContentURL(context.Background(), PurposeDish, " ", 0); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("err=%v want ErrInvalidRecord", err)
	}
}
