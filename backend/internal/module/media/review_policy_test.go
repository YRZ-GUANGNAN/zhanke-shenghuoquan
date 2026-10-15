package media

import "testing"

func TestMediaReviewPolicyPurposeMatrix(t *testing.T) {
	for _, purpose := range []Purpose{PurposePost, PurposeProduct, PurposeAvatar, PurposeProfileCover, PurposeIdentityApplication, PurposeDish, PurposeMerchantLogo, PurposeDeliveryProof, PurposeDeliveryIM, PurposeFeedback, "unknown"} {
		t.Run(string(purpose), func(t *testing.T) {
			direct := purpose == PurposeAvatar || purpose == PurposeProfileCover || purpose == PurposeIdentityApplication || purpose == PurposeDish || purpose == PurposeMerchantLogo
			if got := purpose.ContentReviewRequired(); got == direct {
				t.Fatalf("review required=%v, direct=%v", got, direct)
			}
			for _, status := range []ProjectionStatus{"", ProjectionPending, ProjectionSubmitted, ProjectionReviewing, ProjectionApproved, ProjectionRejected, ProjectionReviewFailed, ProjectionNotRequired} {
				want := purpose.Valid() && (status == ProjectionApproved || (direct && status == ProjectionNotRequired))
				if got := ReviewAllowsUse(purpose, status); got != want {
					t.Fatalf("status=%s allowed=%v want=%v", status, got, want)
				}
			}
		})
	}
	if !ProjectionNotRequired.Valid() || !ProjectionNotRequired.Terminal() || ProjectionNotRequired.Decision() {
		t.Fatal("usage policy must be terminal but must not be a cloud review decision")
	}
}

func TestDirectMediaStillRequiresExactBindingAndVisibility(t *testing.T) {
	for _, purpose := range []Purpose{PurposeAvatar, PurposeProfileCover, PurposeIdentityApplication, PurposeDish, PurposeMerchantLogo} {
		t.Run(string(purpose), func(t *testing.T) {
			value := visibleGatewaySnapshot()
			value.Purpose, value.ReferenceType, value.ProjectionStatus = purpose, purpose, ProjectionNotRequired
			if evaluateVisibility(value) != VisibilityAllowed {
				t.Fatal("valid direct media was hidden")
			}
			value.ParentVisibility = ParentHidden
			if evaluateVisibility(value) != VisibilityParentHidden {
				t.Fatal("private or unsaved media became public")
			}
			value.ParentVisibility, value.ReferenceType = ParentVisible, PurposePost
			if purpose != PurposePost && evaluateVisibility(value) != VisibilityPurposeMismatch {
				t.Fatal("direct image was accepted for a post")
			}
			value.ReferenceType, value.DeleteRequested = purpose, true
			if evaluateVisibility(value) != VisibilityNotReady {
				t.Fatal("deleted media became visible")
			}
		})
	}
	value := visibleGatewaySnapshot()
	value.ProjectionStatus = ProjectionNotRequired
	if evaluateVisibility(value) != VisibilityNotApproved {
		t.Fatal("a post cannot inherit direct profile upload policy")
	}
}
