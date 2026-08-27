package user

import "testing"

func TestFeedbackModelsUseMigratedTableNames(t *testing.T) {
	if got := (Feedback{}).TableName(); got != "user_feedbacks" {
		t.Fatalf("Feedback.TableName() = %q, want user_feedbacks", got)
	}
	if got := (FeedbackImage{}).TableName(); got != "user_feedback_images" {
		t.Fatalf("FeedbackImage.TableName() = %q, want user_feedback_images", got)
	}
}
