package agent

import (
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestParseWGDump(t *testing.T) {
	input := "priv\tpub\t41641\toff\npeer-key\tpsk\t198.51.100.1:41641\t100.64.0.1/32\t1710000000\t123\t456\t25\n"
	stats := ParseWGDump(input)
	if len(stats) != 1 || stats[0].PublicKey != "peer-key" || stats[0].RxBytes != 123 || stats[0].TxBytes != 456 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
	if stats[0].LatestHandshake == nil || stats[0].LatestHandshake.Unix() != 1710000000 {
		t.Fatalf("unexpected handshake: %#v", stats[0].LatestHandshake)
	}
}

func TestParseUAPIStats(t *testing.T) {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i)
	}
	hexKey := hex.EncodeToString(raw)
	input := "public_key=" + hexKey + "\nlast_handshake_time_sec=1710000000\nrx_bytes=99\ntx_bytes=101\nerrno=0\n"
	stats := ParseUAPIStats(input)
	if len(stats) != 1 || stats[0].PublicKey != base64.StdEncoding.EncodeToString(raw) || stats[0].RxBytes != 99 || stats[0].TxBytes != 101 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
}

func TestInactivePathHasNoRoutes(t *testing.T) {
	peer := NetmapPeer{VirtualIP: "100.64.0.1", AllowedIPs: []string{}, PathRole: "direct", PathActive: false}
	if routes := peerAllowedIPs(peer); len(routes) != 0 {
		t.Fatalf("inactive path received routes: %v", routes)
	}
}
