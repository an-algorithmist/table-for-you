package providers

import (
	"net"
	"testing"
)

func TestMenuFileRejectsPrivateAddresses(t *testing.T) {
	for _, raw := range []string{"127.0.0.1", "10.0.0.1", "192.168.0.1", "169.254.169.254", "::1", "fc00::1"} {
		if publicIP(net.ParseIP(raw)) {
			t.Fatal(raw)
		}
	}
	if !publicIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("public address rejected")
	}
}
