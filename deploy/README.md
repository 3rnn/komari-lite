# Native deployment template

This directory intentionally contains no host-specific domain, IP address, machine ID, account file, certificate, or production database.

## Model

- Run `komari.service` as a dedicated unprivileged user, bound to `127.0.0.1:25774`.
- Put an independently configured TLS reverse proxy (for example Caddy) in front of it.
- Keep runtime state under `/opt/komari/data/` with restrictive ownership; it is excluded from Git. The `/opt/komari` parent must be `root:komari` mode `0775` so a restore can atomically exchange the data directory; keep the binary `root:root` mode `0755`.
- Build on a separate machine, upload the resulting binary as `/opt/komari/komari.new`, then call `update-native.sh` on the host.

## Example systemd unit

```ini
[Unit]
Description=Komari Lite Monitor
After=network-online.target
Wants=network-online.target

[Service]
User=komari
Group=komari
WorkingDirectory=/opt/komari
ExecStart=/opt/komari/komari server --listen 127.0.0.1:25774
Restart=always
RestartSec=3
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ReadWritePaths=/opt/komari

[Install]
WantedBy=multi-user.target
```

The restore endpoint intentionally exits the process with status 0 after it has staged a validated archive. `Restart=always` is therefore required to run the restore on the next startup.

Set host checks explicitly before updating a production machine:

```bash
sudo install -m 0755 deploy/update-native.sh /opt/komari/update.sh
export EXPECTED_HOSTNAME="your-hostname"
export EXPECTED_MACHINE_ID="$(cat /etc/machine-id)"
/opt/komari/update.sh /opt/komari/komari.new
```

Do not place credentials, certificates, databases, backup archives, or actual deployment values in this repository.
