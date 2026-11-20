# Docker SD-WAN Lab

This isolated lab runs the platform services, one customer main Hub, two Linux branch agents, and a small service on the Hub's LAN. The agents use real kernel WireGuard interfaces inside containers.

## Start

Requirements: Docker Compose and a Linux kernel with WireGuard support. On Windows, Docker Desktop with the WSL2 backend is the expected setup.

```powershell
docker compose -p sdwan-lab -f docker-compose.yml -f docker-compose.simulation.yml up -d --build
```

The lab uses its own PostgreSQL volume and Docker networks. It does not reuse or overwrite the normal local Compose database.

## Open the UI

- Web console: <http://localhost:8182>
- Controller readiness: <http://localhost:18181/readyz>
- Demo login: `lab-admin@sdwan.test` / `LabNetwork2026!`

The demo account is created only in the lab database. Do not use these credentials outside the local simulation.

## Topology

```text
Platform services (Controller + Web + PostgreSQL)
                    |
        customer overlay: 100.64.240.0/24
          /             |              \
  main-hub          branch-east      branch-west
     |
     +-- HQ LAN: 192.168.250.0/24 -- hq-service (192.168.250.10:8080)
```

`main-hub` advertises `192.168.250.0/24`. Open **Subnet Routes** in the console and approve that route. The branch agents then install the route through WireGuard. The included `hq-service` has a return route via the Hub, so the test exercises both directions rather than only a one-way route advertisement.

After approval, test from a branch container:

```powershell
docker compose -p sdwan-lab -f docker-compose.yml -f docker-compose.simulation.yml exec branch-east ping -c 3 192.168.250.10
docker compose -p sdwan-lab -f docker-compose.yml -f docker-compose.simulation.yml exec branch-east curl http://192.168.250.10:8080/
```

Inspect a WireGuard session:

```powershell
docker compose -p sdwan-lab -f docker-compose.yml -f docker-compose.simulation.yml exec branch-east wg show sdwan0
```

## Stop and Reset

Stop containers while keeping lab data:

```powershell
docker compose -p sdwan-lab -f docker-compose.yml -f docker-compose.simulation.yml down
```

To also delete the lab database and registered demo devices, add `-v`. This is destructive to lab data; it does not affect the normal local Compose volume.

## Simulation Boundaries

- The platform here means the current Controller, Web console, and PostgreSQL. A separate platform-operator/tenant-admin authorization model is not simulated.
- The containers share a Docker network for direct WireGuard reachability. This validates registration, netmap, actual WireGuard handshakes, route approval, Hub forwarding, and branch-to-LAN connectivity.
- Docker networking does not model real-world NAT/CGNAT, symmetric NAT, public Internet loss, or production Relay fallback quality. Those need separate network-namespace or multi-host tests.
- The lab uses a fixed, intentionally public demo credential and admin token. They are test-only and must not be used in a deployed environment.
