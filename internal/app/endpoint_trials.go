package app

import "englishlisten/sdwan/internal/storage/sqlc"

const maxEndpointTrials = 6

// nextDirectEndpoints selects a bounded sequence that tests both sides independently.
func nextDirectEndpoints(client, hub []string, path sqlc.PeerPath) (string, string, bool) {
	if len(client) == 0 || len(hub) == 0 {
		return "", "", false
	}
	type pair struct{ client, hub string }
	pairs := []pair{{client[0], hub[0]}}
	for i := 1; i < len(client) || i < len(hub); i++ {
		if i < len(client) {
			pairs = append(pairs, pair{client[i], hub[0]})
		}
		if i < len(hub) {
			pairs = append(pairs, pair{client[0], hub[i]})
		}
		if i < len(client) && i < len(hub) {
			pairs = append(pairs, pair{client[i], hub[i]})
		}
	}
	current := pair{path.ClientEndpoint, path.HubEndpoint}
	if current == (pair{}) {
		current = pairs[0]
	}
	for i, candidate := range pairs {
		if candidate == current {
			if i+1 == len(pairs) {
				return "", "", false
			}
			next := pairs[i+1]
			return next.client, next.hub, true
		}
	}
	return pairs[0].client, pairs[0].hub, true
}

func preferEndpoint(addresses []string, chosen string) []string {
	for i, address := range addresses {
		if address == chosen && i > 0 {
			return append(append([]string{address}, addresses[:i]...), addresses[i+1:]...)
		}
	}
	return addresses
}
