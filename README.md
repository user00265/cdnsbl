# cdnsbl

A DNS server that returns 127.0.0.1 when an IP belongs to a specific country, and NXDOMAIN otherwise. Useful for country-based filtering in mail servers and firewalls.

## How it works

Query format: `<reversed-ip>.<country-code>.<zone>`

Example: `9.48.252.210.jp.cbl.home.lan`
- Checks if IP 210.252.48.9 is in Japan
- Returns 127.0.0.1 if yes, NXDOMAIN if no

## Backends

Five GeoIP backends available:

- **geoip** - MaxMind GeoLite2 (requires free account and license key)
  - Updates: Weekly (every 7 days)
  - Accuracy: High
  
- **dbip** - DB-IP Lite (no registration, auto-downloads)
  - Updates: Monthly (every 30 days)
  - Accuracy: Good
  
- **ipapi** - IP-API.com (no registration, HTTP API, rate-limited)
  - Updates: Real-time (no database to update)
  - Rate limit: 45 requests/minute

- **ipinfo** - IPInfo.io (requires free token, MMDB or API modes)
  - Updates: Monthly (MMDB mode) or real-time (API mode)
  - Rate limit: 50,000 requests/month (free tier)
  - Supports both database and HTTP API

- **ip2location** - IP2Location LITE (requires free registration)
  - Updates: Monthly (every 30 days)
  - Format: BIN (proprietary)
  - Accuracy: Good

Database backends (geoip, dbip, ipinfo-MMDB, ip2location) automatically download and update their databases in the background without service interruption.

## Quick Start

### Using Docker

```bash
docker-compose up -d
```

Test it:
```bash
dig @localhost -p 5353 8.8.8.8.us.cbl.home.lan A
```

### Using Go

```bash
# Build
go build -o cdnsbl .

# Configure
cp .env.example .env
# Edit .env with your settings

# Run
./cdnsbl
```

## Configuration

Create a `.env` file:

```env
BACKEND=dbip
DNS_LISTEN_ADDR=:5353
DNS_ZONE=cbl.home.lan

# Only needed for geoip backend
GEOIP_LICENSE_KEY=your_key
GEOIP_ACCOUNT_ID=your_id
```

For production on port 53, use `docker-compose.prod.yml` or run as root.

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `BACKEND` | `dbip` | Backend type: `geoip`, `dbip`, `ipapi`, `ipinfo`, or `ip2location` |
| `DATA_DIR` | `./data` | Directory for database files (use `/data` in Docker, `/var/lib/cdnsbl` for system-wide) |
| `DNS_LISTEN_ADDR` | `:5353` | DNS server listen address (use `:53` for production) |
| `DNS_ZONE` | `cbl.home.lan` | DNS zone to serve |
| `GEOIP_LICENSE_KEY` | - | MaxMind license key (required for geoip backend) |
| `GEOIP_ACCOUNT_ID` | - | MaxMind account ID (required for geoip backend) |
| `IPINFO_TOKEN` | - | IPInfo.io API token (required for ipinfo backend) |
| `IPINFO_USE_API` | `false` | Force API mode for IPInfo (skips database) |
| `IP2LOCATION_TOKEN` | - | IP2Location download token (optional, for auto-download) |

Database files stored in `DATA_DIR`:
- **geoip**: `GeoLite2-Country.mmdb`
- **dbip**: `dbip-country-lite.mmdb`
- **ipinfo**: `ipinfo-country.mmdb` (MMDB mode)
- **ip2location**: `ip2location-country.bin`

## Use Cases

**Postfix** - Block countries in `/etc/postfix/main.cf`:
```
smtpd_client_restrictions =
    reject_rbl_client cn.cbl.home.lan,
    reject_rbl_client ru.cbl.home.lan,
    permit
```

**SpamAssassin** - Add to `local.cf`:
```
header RCVD_IN_CBL_CN eval:check_rbl('cbl-cn', 'cn.cbl.home.lan.')
describe RCVD_IN_CBL_CN Received from China
score RCVD_IN_CBL_CN 2.0
```

## Features

- Single binary deployment
- Multiple backend support (MaxMind, DB-IP, IP-API)
- Automatic database updates in the background
- Hot-reload of databases without downtime
- Docker support with compose files
- Low resource usage

## Help

Issues and questions: https://github.com/user00265/cdnsbl

## License

Copyright (c) 2025 Elisamuel Resto Donate <sam@samresto.dev>

Licensed under the MIT License. See LICENSE file for details.
