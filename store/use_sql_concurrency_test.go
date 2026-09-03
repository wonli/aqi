package store

import (
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

func TestSqlServerConcurrentFirstUseIsRaceFree(t *testing.T) {
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

	// SQL Server initialization may still fail when no server is listening.
	// The regression here is concurrent access to retryable initialization;
	// -race is the oracle rather than requiring an offline dial to succeed.
	assertConcurrentUseCompletes(t, func() { _ = store.Use() })
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
