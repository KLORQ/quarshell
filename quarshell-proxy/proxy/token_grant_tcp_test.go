//go:build !windows

package proxy

import "testing"

func TestSPIFFETokenGrantResolverUsesAcknowledgedTCPWorkloadAPI(t *testing.T) {
	testSPIFFETokenGrantResolverUsesWorkloadSVIDAndRefreshesAfterExpiry(t, true)
}

func TestSPIFFEWorkloadAPIAddressRequiresConcreteTCPAddress(t *testing.T) {
	for _, endpoint := range []string{"tcp:localhost:8081", "tcp:0.0.0.0:8081", "tcp:127.0.0.1:0", "tcp:127.0.0.1"} {
		if _, err := spiffeWorkloadAPIAddress(endpoint); err == nil {
			t.Errorf("spiffeWorkloadAPIAddress(%q) unexpectedly succeeded", endpoint)
		}
	}
	got, err := spiffeWorkloadAPIAddress("tcp:127.0.0.1:8081")
	if err != nil || got != "tcp://127.0.0.1:8081" {
		t.Fatalf("TCP endpoint = (%q, %v)", got, err)
	}
}
