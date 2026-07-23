# VoHive

A unified management & proxy platform for Qualcomm 4G/LTE/5G modems — modem hot-plug management, SOCKS5/HTTP proxy, SMS messaging, VoWiFi/IMS voice, and eSIM lifecycle control in a single Docker image.

## Quick Start

### 1. Create directories

```bash
mkdir -p vohive/{config,data,logs}
cd vohive
```

### 2. Create a minimal config

```bash
cat > config/config.yaml << 'EOF'
server:
  port: 7575
  debug: false

web:
  username: admin
  password: admin
EOF
```

> Change the default password after first login.

### 3. Start with Docker Compose

Create `docker-compose.yml`:

```yaml
services:
  vohive:
    image: voorz/vohive:latest
    container_name: vohive
    restart: unless-stopped
    network_mode: host
    privileged: true
    volumes:
      - ./config:/app/config
      - ./data:/app/data
      - ./logs:/app/logs
      - /dev:/dev
    environment:
      - TZ=Asia/Shanghai
      - CONFIG_PATH=/app/config/config.yaml
```

```bash
docker compose up -d
```

### 4. Access the web UI

Open `http://YOUR_IP:7575` in a browser.

Default credentials: `admin` / `admin`

## Image Tags

| Tag | Description |
| :--- | :--- |
| `latest` | Latest stable release |
| `vX.Y.Z` | Pinned semantic version |

## Supported Architectures

| Architecture | Tag suffix |
| :--- | :--- |
| linux/amd64 | — |
| linux/arm64 | — |
| linux/arm/v7 | — |

Multi-arch manifests are published automatically. Simply `docker pull voorz/vohive:latest` and Docker selects the correct variant.

## Environment Variables

| Variable | Default | Description |
| :--- | :--- | :--- |
| `TZ` | `UTC` | Timezone (`Asia/Shanghai`, `America/New_York`, …) |
| `CONFIG_PATH` | `/app/config/config.yaml` | Path to the YAML config file |
| `HTTPS_PROXY` | — | Outbound proxy for Telegram API (optional) |

## Volumes

| Path | Description |
| :--- | :--- |
| `/app/config` | Configuration files (`config.yaml`) |
| `/app/data` | SQLite database & persistent state |
| `/app/logs` | Log files |

## Ports

| Port | Protocol | Description |
| :--- | :--- | :--- |
| 7575 | TCP | Web UI & REST API |

> When using `network_mode: host` (recommended), the port is exposed directly on the host. Otherwise map it with `-p 7575:7575`.

## Device Access

VoHive requires direct access to USB modems and network interfaces. The container must run with:

- `privileged: true` — for USB device, network management, and `/dev/net/tun` access
- `/dev:/dev` volume mount — for USB modem passthrough (ttyUSB, etc.)
- `network_mode: host` — for per-device outbound binding (`SO_BINDTODEVICE`) and interface management

## Configuration

The minimal config above is enough to get started. VoHive auto-detects connected modems on startup. Advanced features (proxy instances, VoWiFi, notifications, eSIM) can be configured through the web UI or by extending `config.yaml`.

## Telegram Bot (Optional)

VoHive supports remote management via Telegram Bot. Configure the Bot Token and Chat ID in the web UI under **Settings → Notifications**.

If the server cannot reach Telegram API directly (e.g. in mainland China), set the `HTTPS_PROXY` environment variable or use a Cloudflare Worker URL in the TG API Proxy field.

| Command | Description |
| :--- | :--- |
| `/list` | List devices |
| `/rotate <device>` | Rotate device IP |
| `/sms <device>` | Show recent SMS |
| `/send <device> <number> <content>` | Send SMS |

## Links

- **Source**: [github.com/voorz/vohive-next](https://github.com/voorz/vohive-next)
- **Releases**: [github.com/voorz/vohive-next/releases](https://github.com/voorz/vohive-next/releases)
- **License**: PolyForm Noncommercial 1.0.0 — non-commercial use only
