package utils

func LimitString(str string, limit int) string {
	// Convert the string to a rune slice to properly handle Unicode characters
	runes := []rune(str)

	if len(runes) > limit {
		// If the length exceeds the limit
		// truncate the slice to the specified length
		runes = runes[:limit]
	}

	return string(runes)
}
