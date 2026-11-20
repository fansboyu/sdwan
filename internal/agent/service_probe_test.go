package agent

import (
	"net"
	"strconv"
	"testing"
)

func TestProbeTCPServicesReportsReachabilityAndRejectsOutsideCIDR(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	_, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}

	results := probeTCPServices("127.0.0.0/8", []string{"127.0.0.1:" + portText, "192.0.2.1:443"})
	if len(results) != 2 {
		t.Fatalf("expected two probe results, got %d", len(results))
	}
	if !results[0].Reachable || results[0].Target != net.JoinHostPort("127.0.0.1", strconv.Itoa(port)) {
		t.Fatalf("local TCP service should be reachable: %#v", results[0])
	}
	if results[1].Reachable || results[1].Error == "" {
		t.Fatalf("target outside advertised subnet should be rejected: %#v", results[1])
	}
}

func TestProbeTCPServicesLimitsTargets(t *testing.T) {
	targets := make([]string, 12)
	for i := range targets {
		targets[i] = "192.0.2.1:443"
	}
	if got := len(probeTCPServices("192.0.2.0/24", targets)); got != 8 {
		t.Fatalf("probe count must be capped at 8, got %d", got)
	}
}
