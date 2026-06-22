package takeout

var dishTransitions = map[DishStatus]map[DishStatus]struct{}{
	DishDraft:    dishSetOf(DishOnShelf),
	DishOnShelf:  dishSetOf(DishSoldOut, DishOffShelf),
	DishSoldOut:  dishSetOf(DishOnShelf, DishOffShelf),
	DishOffShelf: dishSetOf(DishOnShelf, DishRemoved),
	DishRemoved:  {},
}

func CanTransitionDish(from, to DishStatus) bool {
	_, ok := dishTransitions[from][to]
	return ok
}

func dishSetOf(values ...DishStatus) map[DishStatus]struct{} {
	result := make(map[DishStatus]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}
