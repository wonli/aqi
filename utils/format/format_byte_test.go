package format

import "testing"

func TestBite(t *testing.T) {
	bytes := Bites(2624954)
	t.Log(bytes)
}
