# Path Application And Verification

Path changes have separate controller intent, agent application, and reachability verification states.

```text
Controller increments path generation and publishes the desired path
  -> both Linux/Windows agents apply their effective WireGuard AllowedIPs
  -> each agent identifies which peer owns the other site's overlay IP
  -> each agent probes that overlay IP and reports generation/path/result
  -> Controller completes the transition only when both reports agree and pass
```

`peer_paths.current_path` remains the last path confirmed by both endpoints. `desired_path` is the controller's target. `verification_state` reports `pending`, `verifying`, `verified`, `failed`, or `path_mismatch`; the last confirmed `verified_path`, generation, timestamp, and diagnostic are retained for the console.

The endpoint-level `device_path_applied` record stores the generation applied by that device, the path that actually owns the target overlay IP in the local WireGuard config, and the result of its probe. Reports from older generations cannot overwrite newer state.

The probe is an ICMP ping to the other site's overlay IP, not an application/LAN service check. A local ping success proves the selected overlay route is reachable from that agent at probe time; it does not prove every advertised LAN route or application is healthy. Those need explicit service probes.

If either probe fails or the endpoints report different paths, the controller does not mark the transition complete. It preserves the desired generation and exposes the failure for diagnosis; retry occurs on subsequent agent polls. There is no automatic rollback in this verification layer.

Local validation covers route-owner selection, generation coordination decisions, probe failure reporting, Go tests, and the Docker-hosted controller/UI. It does not replace a real multi-node WireGuard test, because the local Compose stack does not create separate network namespaces or remote endpoints.
