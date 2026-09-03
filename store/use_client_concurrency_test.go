package store

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/spf13/viper"
)

func TestSQLiteConcurrentFirstUseReturnsSingleDB(t *testing.T) {
	key := "test.sqlite.concurrent_use"
	viper.Set(key+".database", filepath.Join(t.TempDir(), "test.db"))
	viper.Set(key+".maxOpenConns", 1)
	viper.Set(key+".maxIdleConns", 1)

	store := &SQLiteStore{configKey: key}
	const workers = 32
	results := make([]any, workers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			<-start
			results[i] = store.Use()
		}(i)
	}

	close(start)
	wg.Wait()

	first := results[0]
	if first == nil {
		t.Fatal("SQLite first Use returned nil")
	}
	for i, got := range results[1:] {
		if got != first {
			t.Fatalf("concurrent SQLite Use returned different DBs: first=%p got[%d]=%p", first, i+1, got)
		}
	}
}

func TestRedisConcurrentFirstUseReturnsSingleClient(t *testing.T) {
	key := "test.redis.concurrent_use"
	viper.Set(key+".addr", "127.0.0.1:6379")

	store := &RedisStore{configKey: key}
	const workers = 32
	results := make([]any, workers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			<-start
			results[i] = store.Use()
		}(i)
	}

	close(start)
	wg.Wait()

	first := results[0]
	if first == nil {
		t.Fatal("Redis first Use returned nil")
	}
	for i, got := range results[1:] {
		if got != first {
			t.Fatalf("concurrent Redis Use returned different clients: first=%p got[%d]=%p", first, i+1, got)
		}
	}
}
