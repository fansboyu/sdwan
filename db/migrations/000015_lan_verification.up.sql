CREATE TABLE employee_lan_verifications (
 device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
 route_id TEXT NOT NULL REFERENCES subnet_routes(id) ON DELETE CASCADE,
 target TEXT NOT NULL,
 reachable BOOLEAN NOT NULL,
 checked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY (device_id, route_id, target)
);
