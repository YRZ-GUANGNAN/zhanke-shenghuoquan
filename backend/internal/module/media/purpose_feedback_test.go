package media

import "testing"

func TestFeedbackPurposeIsValidAndPrivate(t *testing.T) {
	if !PurposeFeedback.Valid() {
		t.Fatal("feedback purpose must be valid for the upload and binding pipeline")
	}
	if PurposeFeedback.PubliclyResolvable() {
		t.Fatal("feedback media must not be publicly resolvable")
	}
}
