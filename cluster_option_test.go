package aqi

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

type testClusterTransport struct{}

func (testClusterTransport) Subscribe(string) error       { return nil }
func (testClusterTransport) Unsubscribe(string) error     { return nil }
func (testClusterTransport) Publish(string, []byte) error { return nil }
func (testClusterTransport) Close() error                 { return nil }

func TestClusterOptionOnlyEnablesCapability(t *testing.T) {
	config := &AppConfig{}
	if err := WithCluster()(config); err != nil {
		t.Fatal(err)
	}
	if !config.Cluster {
		t.Fatal("WithCluster did not enable cluster mode")
	}
}

func TestWithClusterTransportEnablesClusterAndStoresFactory(t *testing.T) {
	factory := func(handler ClusterMessageHandler) (ClusterTransport, error) {
		if handler == nil {
			t.Fatal("custom cluster transport factory received nil handler")
		}
		return testClusterTransport{}, nil
	}

	config := &AppConfig{}
	if err := WithClusterTransport(factory)(config); err != nil {
		t.Fatal(err)
	}
	if !config.Cluster {
		t.Fatal("WithClusterTransport did not enable cluster mode")
	}
	if config.ClusterTransportFactory == nil {
		t.Fatal("WithClusterTransport did not retain factory")
	}
}

func TestWithClusterTransportRejectsNilFactory(t *testing.T) {
	if err := WithClusterTransport(nil)(&AppConfig{}); err == nil {
		t.Fatal("WithClusterTransport accepted nil factory")
	}
}

func TestClusterBootstrapDisabledRequiresNoRedis(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	if err := bootstrapCluster(&AppConfig{}); err != nil {
		t.Fatalf("disabled cluster bootstrap returned error: %v", err)
	}
}

func TestClusterBootstrapCustomTransportBypassesRedisAQI(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	sentinel := errors.New("custom transport factory invoked")
	config := &AppConfig{
		Cluster: true,
		ClusterTransportFactory: func(handler ClusterMessageHandler) (ClusterTransport, error) {
			if handler == nil {
				t.Fatal("custom cluster transport factory received nil handler")
			}
			return nil, sentinel
		},
	}

	err := bootstrapCluster(config)
	if !errors.Is(err, sentinel) {
		t.Fatalf("custom cluster bootstrap error = %v, want wrapped sentinel", err)
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
