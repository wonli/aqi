package store

import (
	"fmt"
	"sync"
	"testing"
)

func TestDBConcurrentFirstUseReturnsSingleStore(t *testing.T) {
	assertConcurrentSingleStore(t, "mysql", func(key string) any { return DB(key) })
}

func TestSQLiteConcurrentFirstUseReturnsSingleStore(t *testing.T) {
	assertConcurrentSingleStore(t, "sqlite", func(key string) any { return SQLite(key) })
}

func TestRedisConcurrentFirstUseReturnsSingleStore(t *testing.T) {
	assertConcurrentSingleStore(t, "redis", func(key string) any { return Redis(key) })
}

func TestSqlServerConcurrentFirstUseReturnsSingleStore(t *testing.T) {
	assertConcurrentSingleStore(t, "sqlserver", func(key string) any { return SqlServer(key) })
}

func assertConcurrentSingleStore(t *testing.T, prefix string, get func(string) any) {
	t.Helper()

	const rounds = 200
	const workers = 32

	for round := 0; round < rounds; round++ {
		key := fmt.Sprintf("test.%s.concurrent.%d", prefix, round)
		start := make(chan struct{})
		results := make(chan any, workers)
		var wg sync.WaitGroup

		wg.Add(workers)
		for i := 0; i < workers; i++ {
			go func() {
				defer wg.Done()
				<-start
				results <- get(key)
			}()
		}

		close(start)
		wg.Wait()
		close(results)

		var first any
		for got := range results {
			if first == nil {
				first = got
				continue
			}
			if got != first {
				t.Fatalf("round %d: concurrent first use returned different stores: first=%p got=%p", round, first, got)
			}
		}
	}
}
