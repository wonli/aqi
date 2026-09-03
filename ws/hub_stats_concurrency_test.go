package ws

import (
	"sync"
	"testing"
)

func TestHubStatsConcurrentReadWrite(t *testing.T) {
	hub := NewHubc()

	const workers = 16
	const iterations = 10000

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(workers * 2)

	for i := 0; i < workers; i++ {
		go func(offset int) {
			defer wg.Done()
			<-start
			for n := 0; n < iterations; n++ {
				hub.setCounts(offset+n, offset+n)
			}
		}(i)

		go func() {
			defer wg.Done()
			<-start
			for n := 0; n < iterations; n++ {
				_, _ = hub.Counts()
			}
		}()
	}

	close(start)
	wg.Wait()
}
