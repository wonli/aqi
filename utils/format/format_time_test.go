package format

import (
	"testing"
)

func TestTime(t *testing.T) {
	fTime := NewFriendTime(1691724579)
	fTime.SetSuffix("哈哈")
	t.Log(fTime.Format())
}
