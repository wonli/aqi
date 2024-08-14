package format

import (
	"log"
	"testing"
	"time"
)

func TestTimeSince(t *testing.T) {
	past := time.Now()
	time.Sleep(time.Millisecond)
	a := TimeSince(past)

	log.Println(a)
}
