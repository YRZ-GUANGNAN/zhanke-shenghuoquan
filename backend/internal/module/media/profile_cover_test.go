package media

import "testing"

func TestProfileCoverUsesExistingReviewedUploadBoundary(t *testing.T) {
	if !PurposeProfileCover.Valid() || !PurposeProfileCover.PubliclyResolvable() {
		t.Fatal("profile cover must enter the normal reviewed media pipeline")
	}
	input := CreateUploadTicketInput{OwnerID: 7, Purpose: PurposeProfileCover, ExpectedSize: 10 << 20, ExpectedMD5: "1B2M2Y8AsgTpgAmY7PhCfg==", ExpectedContentType: "image/jpeg", IdempotencyKey: "isolated-cover-size"}
	if err := validateUploadDeclaration(input); err != nil {
		t.Fatal(err)
	}
	input.ExpectedSize++
	if err := validateUploadDeclaration(input); err == nil {
		t.Fatal("oversized cover accepted")
	}
	input.ExpectedSize = 10
	input.ExpectedContentType = "image/svg+xml"
	if err := validateUploadDeclaration(input); err == nil {
		t.Fatal("unsupported image format accepted")
	}
	input.ExpectedContentType = "image/png"
	input.OwnerID = 0
	if err := validateUploadDeclaration(input); err == nil {
		t.Fatal("anonymous cover upload accepted")
	}
}
