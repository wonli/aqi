package utils

import (
	"log"
	"testing"
)

func TestIntSliceToString(t *testing.T) {
	slice := StringToSlice[int64]("574159,513036,500295,492558,480641,480174,423891,385470,360901,246633")
	log.Println(slice)
}
