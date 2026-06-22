package takeout

import "testing"

func TestDishStateMachineMatchesG2Contract(t *testing.T) {
	allowed := map[[2]DishStatus]bool{
		{DishDraft, DishOnShelf}:    true,
		{DishOnShelf, DishSoldOut}:  true,
		{DishOnShelf, DishOffShelf}: true,
		{DishSoldOut, DishOnShelf}:  true,
		{DishSoldOut, DishOffShelf}: true,
		{DishOffShelf, DishOnShelf}: true,
		{DishOffShelf, DishRemoved}: true,
	}
	statuses := []DishStatus{DishDraft, DishOnShelf, DishSoldOut, DishOffShelf, DishRemoved}
	for _, from := range statuses {
		for _, to := range statuses {
			if got, want := CanTransitionDish(from, to), allowed[[2]DishStatus{from, to}]; got != want {
				t.Errorf("CanTransitionDish(%s,%s)=%v want=%v", from, to, got, want)
			}
		}
	}
}

func TestDishStatusValidityAndTerminality(t *testing.T) {
	for _, status := range []DishStatus{DishDraft, DishOnShelf, DishSoldOut, DishOffShelf, DishRemoved} {
		if !status.Valid() {
			t.Errorf("status %s is invalid", status)
		}
		if got, want := status.Terminal(), status == DishRemoved; got != want {
			t.Errorf("status %s Terminal=%v want=%v", status, got, want)
		}
	}
	for _, status := range []DishStatus{"", "active", "deleted"} {
		if status.Valid() || status.Terminal() {
			t.Errorf("invalid status %q accepted", status)
		}
	}
}
