package ip

import (
	"net"
	"net/http/httptest"
	"testing"
)

func TestIsPrivateSubnetIncludesRangeBoundaries(t *testing.T) {
	private := []string{
		"10.0.0.0", "10.255.255.255",
		"100.64.0.0", "100.127.255.255",
		"172.16.0.0", "172.31.255.255",
		"192.168.0.0", "192.168.255.255",
		"198.18.0.0", "198.19.255.255",
	}

	for _, raw := range private {
		if !isPrivateSubnet(net.ParseIP(raw)) {
			t.Fatalf("isPrivateSubnet(%s) = false, want true", raw)
		}
	}

	public := []string{"8.8.8.8", "1.1.1.1", "172.32.0.1"}
	for _, raw := range public {
		if isPrivateSubnet(net.ParseIP(raw)) {
			t.Fatalf("isPrivateSubnet(%s) = true, want false", raw)
		}
	}
}

func TestGetIPAddress(t *testing.T) {
	t.Run("prefers rightmost public forwarded address", func(t *testing.T) {
		r := httptest.NewRequest("GET", "http://example.com", nil)
		r.RemoteAddr = "10.0.0.2:1234"
		r.Header.Set("X-Forwarded-For", "203.0.113.10, 8.8.8.8, 10.0.0.1")

		if got := GetIPAddress(r); got != "8.8.8.8" {
			t.Fatalf("GetIPAddress() = %q, want %q", got, "8.8.8.8")
		}
	})

	t.Run("falls back to x-real-ip", func(t *testing.T) {
		r := httptest.NewRequest("GET", "http://example.com", nil)
		r.RemoteAddr = "10.0.0.2:1234"
		r.Header.Set("X-Forwarded-For", "10.0.0.1")
		r.Header.Set("X-Real-Ip", "8.8.4.4")

		if got := GetIPAddress(r); got != "8.8.4.4" {
			t.Fatalf("GetIPAddress() = %q, want %q", got, "8.8.4.4")
		}
	})

	t.Run("falls back to remote address", func(t *testing.T) {
		r := httptest.NewRequest("GET", "http://example.com", nil)
		r.RemoteAddr = "192.0.2.10:4321"

		if got := GetIPAddress(r); got != "192.0.2.10" {
			t.Fatalf("GetIPAddress() = %q, want %q", got, "192.0.2.10")
		}
	})
}
