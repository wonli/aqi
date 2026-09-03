package store

import (
	"fmt"
	"sync"
	"testing"

	"github.com/spf13/viper"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

func TestMySQLConcurrentFirstUseIsRaceFree(t *testing.T) {
	key := "test.mysql.concurrent_use"
	viper.Set(key+".enable", 1)
	viper.Set(key+".host", "127.0.0.1")
	viper.Set(key+".port", 3306)
	viper.Set(key+".username", "test")
	viper.Set(key+".password", "test")
	viper.Set(key+".database", "test")

	store := &MySQLStore{configKey: key}
	store.Options(&gorm.Config{
		DisableAutomaticPing: true,
		Logger:               gormLogger.Default.LogMode(gormLogger.Silent),
	})

	// The MySQL dialector still probes server version during initialization,
	// so an offline test may legitimately return nil. The regression here is
	// concurrent access to the store initialization state; -race is the oracle.
	assertConcurrentUseCompletes(t, func() { _ = store.Use() })
}

func TestSqlServerConcurrentFirstUseReturnsSingleDB(t *testing.T) {
	key := "test.sqlserver.concurrent_use"
	viper.Set(key+".host", "127.0.0.1")
	viper.Set(key+".port", 1433)
	viper.Set(key+".username", "test")
	viper.Set(key+".password", "test")
	viper.Set(key+".database", "test")

	store := &SqlServerStore{configKey: key}
	store.Options(&gorm.Config{
		DisableAutomaticPing: true,
		Logger:               gormLogger.Default.LogMode(gormLogger.Silent),
	})

	assertConcurrentFirstUseSingleDB(t, func() *gorm.DB { return store.Use() })
}

func assertConcurrentUseCompletes(t *testing.T, use func()) {
	t.Helper()
	const workers = 32

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			<-start
			use()
		}()
	}

	close(start)
	wg.Wait()
}

func assertConcurrentFirstUseSingleDB(t *testing.T, use func() *gorm.DB) {
	t.Helper()
	const workers = 32

	results := make([]*gorm.DB, workers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			<-start
			results[i] = use()
		}(i)
	}

	close(start)
	wg.Wait()

	first := results[0]
	if first == nil {
		t.Fatal("first Use returned nil")
	}

	for i, got := range results[1:] {
		if got != first {
			t.Fatalf("concurrent first Use returned different DBs: first=%p got[%d]=%p", first, i+1, got)
		}
	}

	for i, db := range results {
		if db == nil {
			t.Fatal(fmt.Sprintf("Use result[%d] is nil", i))
		}
	}
}
