package utils

func SliceFindIndex[T comparable](inputSlice []T, a T) int {
	findIndex := -1
	for index, element := range inputSlice {
		if element == a {
			findIndex = index
			break
		}
	}

	return findIndex
}
