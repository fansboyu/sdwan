# Enterprise onboarding

In the enterprise console, open the deployment wizard. Issue a 30-minute
single-use credential for each device. The credential is displayed only on
creation. Revoke an unused credential if it is no longer needed. Revocation
does not remove a device that already enrolled; use the device page for that.

Linux registration accepts `--enrollment-token`. The existing Windows tray
can paste the same credential via Join from Clipboard. The registration API
still accepts legacy admin credentials for existing automation, but the
enterprise settings no longer display the admin session token.

Set a registered Linux device as the Hub. Enable LAN forwarding there and
approve the announced prefix in the enterprise console. To verify an actual
TCP service from an enrolled Linux employee device, run:

```bash
sudo sdwan-agent verify-lan --target 192.168.50.10:443
```

The command checks that the system routes the target through the SD-WAN
interface, attempts a TCP connection and reports its result with the device
credential. Refresh the result in the wizard. This is a timestamped TCP
observation for one device and target, not proof that the application works
for everyone. Hub probes and Overlay verification are separate signals.
