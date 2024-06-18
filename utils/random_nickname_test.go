package utils

import "testing"

func TestGetRandomNickname(t *testing.T) {
	for i := 0; i <= 100; i++ {
		println(GetRandomNickname())
	}
}
