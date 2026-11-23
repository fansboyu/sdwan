# Local Relay failover lab

Use a dedicated Docker Desktop Linux environment. This exercise is for the local
`sdwan-lab` demo account only. It changes the demo account path mode, drops
UDP between branch-east and the Hub inside the branch container, and stops
the lab Relay temporarily. Do not point it at a shared or production network.

From the repository root, start the lab:

```powershell
docker compose -p sdwan-lab -f docker-compose.yml -f docker-compose.simulation.yml -f docker-compose.simulation-relay.yml up -d --build
```

Wait for the Relay to sync peers and for the Hub LAN route
`192.168.250.0/24` to be approved in the enterprise console. Then run:

```powershell
./simulation/relay-failover.ps1
```

Each JSON line reports a phase, HTTP samples and failures, the longest observed
run of failed requests, and the Controller, branch, and Hub generations.
A phase passes only when HTTP succeeds and both devices have verified the target
generation. The longest observed failure is an estimate based on request start
times and is bounded by the sampling cadence. A zero does not establish
zero packet loss. The script removes its UDP fault rules, restarts a Relay it
stopped, and restores the previous path mode on ordinary exit or timeout.

## Reference run

One Docker Desktop run on 2026-11-23, with the Hub and branch running Linux:

| Scenario | HTTP failures / samples | Longest observed failure | Verified convergence | Branch / Hub generation |
| --- | ---: | ---: | ---: | ---: |
| Direct blocked, switch to Relay | 33 / 48 | 52.08 s | 60.10 s | 18 / 18 |
| Direct restored, switch back | 3 / 192 | 4.67 s | 110.52 s | 20 / 20 |
| Relay stopped, direct available | 10 / 20 | 15.81 s | 20.91 s | 23 / 23 |

These are single-run lab measurements, not production latency guarantees.

The Relay key and token live in the lab-only named volume, not the repository.
An existing volume assumes the corresponding Relay record still exists in the
lab database. If rebuilding the lab state, recreate the Relay volume with it.
