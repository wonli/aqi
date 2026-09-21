package aqi

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestClusterOptionOnlyEnablesCapability(t *testing.T) {
	config := &AppConfig{}
	if err := WithCluster()(config); err != nil {
		t.Fatal(err)
	}
	if !config.Cluster {
		t.Fatal("WithCluster did not enable cluster mode")
	}
}

func TestClusterBootstrapDisabledRequiresNoRedis(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	if err := bootstrapCluster(&AppConfig{}); err != nil {
		t.Fatalf("disabled cluster bootstrap returned error: %v", err)
	}
}

func TestClusterBootstrapRequiresRedisAQI(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	err := bootstrapCluster(&AppConfig{Cluster: true})
	if err == nil {
		t.Fatal("cluster bootstrap succeeded without redis.aqi")
	}
	if !strings.Contains(err.Error(), "redis.aqi") {
		t.Fatalf("error %q does not mention redis.aqi", err)
	}
}

func TestClusterBootstrapRejectsUnusableRedis(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	viper.Set("redis.aqi.addr", "127.0.0.1:1")
	err := bootstrapCluster(&AppConfig{Cluster: true})
	if err == nil {
		t.Fatal("cluster bootstrap succeeded with unusable redis.aqi")
	}
	if !strings.Contains(err.Error(), "redis.aqi") {
		t.Fatalf("error %q does not mention redis.aqi", err)
	}
}
