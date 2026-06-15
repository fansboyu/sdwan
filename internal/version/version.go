package version

import (
	"fmt"
	"strings"
)

const (
	Version    = "v1.2.0"
	APIVersion = "v1"
)

func Compare(left, right string) int {
	left = strings.TrimPrefix(strings.TrimSpace(left), "v")
	right = strings.TrimPrefix(strings.TrimSpace(right), "v")
	var l, r [3]int
	fmt.Sscanf(left, "%d.%d.%d", &l[0], &l[1], &l[2])
	fmt.Sscanf(right, "%d.%d.%d", &r[0], &r[1], &r[2])
	for i := range l {
		if l[i] < r[i] {
			return -1
		}
		if l[i] > r[i] {
			return 1
		}
	}
	return 0
}
