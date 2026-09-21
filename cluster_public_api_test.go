package aqi_test

import (
	"testing"

	"github.com/wonli/aqi"
)

type customClusterTransport struct{}

func (*customClusterTransport) Subscribe(string) error       { return nil }
func (*customClusterTransport) Unsubscribe(string) error     { return nil }
func (*customClusterTransport) Publish(string, []byte) error { return nil }
func (*customClusterTransport) Close() error                 { return nil }

var _ aqi.ClusterTransport = (*customClusterTransport)(nil)

func TestCustomClusterTransportPublicAPI(t *testing.T) {
	config := &aqi.AppConfig{}
	err := aqi.WithClusterTransport(func(handler aqi.ClusterMessageHandler) (aqi.ClusterTransport, error) {
		if handler == nil {
			t.Fatal("custom cluster transport received nil inbound handler")
		}
		return &customClusterTransport{}, nil
	})(config)
	if err != nil {
		t.Fatal(err)
	}
	if !config.Cluster {
		t.Fatal("custom cluster transport did not enable cluster mode")
	}
}
