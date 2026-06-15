package agent

import (
	"bufio"
	"encoding/base64"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

func ParseWGDump(output string) []PeerStat {
	var result []PeerStat
	scanner := bufio.NewScanner(strings.NewReader(output))
	line := 0
	for scanner.Scan() {
		line++
		if line == 1 {
			continue
		}
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) < 8 {
			continue
		}
		hs, _ := strconv.ParseInt(fields[4], 10, 64)
		rx, _ := strconv.ParseInt(fields[5], 10, 64)
		tx, _ := strconv.ParseInt(fields[6], 10, 64)
		var at *time.Time
		if hs > 0 {
			v := time.Unix(hs, 0).UTC()
			at = &v
		}
		result = append(result, PeerStat{PublicKey: fields[0], LatestHandshake: at, RxBytes: rx, TxBytes: tx})
	}
	return result
}

func ParseUAPIStats(output string) []PeerStat {
	var result []PeerStat
	var current *PeerStat
	for _, line := range strings.Split(output, "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, value := parts[0], parts[1]
		switch key {
		case "public_key":
			if current != nil {
				result = append(result, *current)
			}
			raw, err := hex.DecodeString(value)
			if err != nil {
				current = nil
				continue
			}
			current = &PeerStat{PublicKey: base64.StdEncoding.EncodeToString(raw)}
		case "last_handshake_time_sec":
			if current != nil {
				v, _ := strconv.ParseInt(value, 10, 64)
				if v > 0 {
					t := time.Unix(v, 0).UTC()
					current.LatestHandshake = &t
				}
			}
		case "rx_bytes":
			if current != nil {
				current.RxBytes, _ = strconv.ParseInt(value, 10, 64)
			}
		case "tx_bytes":
			if current != nil {
				current.TxBytes, _ = strconv.ParseInt(value, 10, 64)
			}
		}
	}
	if current != nil {
		result = append(result, *current)
	}
	return result
}
