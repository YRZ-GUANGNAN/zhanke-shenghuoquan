package media

import (
	"reflect"
	"strings"
	"testing"
)

func TestIdentityApplicationPurposeIsPrivate(t *testing.T) {
	if !PurposeIdentityApplication.Valid() {
		t.Fatal("identity application purpose must be valid")
	}
	if PurposeIdentityApplication.PubliclyResolvable() {
		t.Fatal("identity application proof must stay private")
	}
}

func TestIdentityApplicationPurposeColumnsAllowFullValue(t *testing.T) {
	assetPurpose, ok := reflect.TypeOf(Asset{}).FieldByName("Purpose")
	if !ok || !strings.Contains(assetPurpose.Tag.Get("gorm"), "size:24") {
		t.Fatalf("Asset.Purpose gorm tag=%q", assetPurpose.Tag.Get("gorm"))
	}
	referenceType, ok := reflect.TypeOf(AssetReference{}).FieldByName("ReferenceType")
	if !ok || !strings.Contains(referenceType.Tag.Get("gorm"), "size:24") {
		t.Fatalf("AssetReference.ReferenceType gorm tag=%q", referenceType.Tag.Get("gorm"))
	}
}
