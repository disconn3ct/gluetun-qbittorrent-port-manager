# Gluetun qBitTorrent Port Manager

> [!NOTE]  
> This is a fork of [aunefyren/gluetun-qbittorrent-port-manager](https://github.com/aunefyren/gluetun-qbittorrent-port-manager), which is a spiritual successor to [SnoringDragon/gluetun-qbittorrent-port-manager](https://github.com/SnoringDragon/gluetun-qbittorrent-port-manager).

## What?

A simple Go application that syncs your [Gluetun port](https://github.com/qdm12/gluetun-wiki/blob/main/setup/options/port-forwarding.md) to the [qBittorrent](https://github.com/qbittorrent/qBittorrent) listening port. It can run as a standalone executable, or in a Docker container.

## Why?

Gluetun can receive a port from a VPN provider which is port-forwarded. Many want to utilize this port elsewhere, and therefore Gluetun has an API to retrieve this port. It is often desired for qBitTorrent to use this port as a `listen port` when doing peer-to-peer connections. This simple program synchronizes those values, ensuring qBitTorrent stays in sync with Gluetun.

## How?

The Gluetun and qBitTorrent APIs are queried by the program if they are not in sync, qBitTorrent is updated through the API. If Gluetun does not have a forwarded port, nothing is done to qBitTorrent.

## When?

Every `INTERVAL` minutes. (Default is 15 minutes.)

## Configuration?

Environment variables:

| Environment variable         | Type   | Description                                                                   |
| ---------------------------- | ------ | ----------------------------------------------------------------------------- |
| HTTPS                        | bool   | Access qBit over https (default: `false`, uses http)                          |
| IP                           | string | IP qBit listens on (default: `localhost`)                                     |
| PORT                         | int    | Port qBit listens on (default: `8080`)                                        |
| USERNAME                     | string | Username for qBit login (default: `admin`)                                    |
| PASSWORD                     | string | Password for qBit login (no default)                                          |
| WAITFORQBIT                  | bool   | Wait for QBitTorrent to start before manager starts working (default: `true`) |
| GLUETUNIP                    | string | IP Gluetun listens on (default: `localhost`)                                  |
| GLUETUNPORT                  | int    | Port Gluetun listens on (default: `8000`)                                     |
| TZ                           | string | Timezone of the app (default: `Europe/Paris`)                                 |
| ENVIRONMENT                  | string | Defines program behavior (default: `production`)                              |
| LOGLEVEL                     | string | Amount of logs (default: `info`)                                              |
| INTERVAL                     | int    | Minutes between port checks (default: `15`)                                   |
| TIMEOUT                      | int    | HTTP timeout in seconds (default `15`)                                        |

## Docker Compose example?

```yaml
services:
  gluetun-qbittorrent-port-manager:
    container_name: gluetun-qbittorrent-port-manager
    image: ghcr.io/disconn3ct/gluetun-qbittorrent-port-manager:latest
    restart: unless-stopped
    environment:
      # General settings:
      TZ: Europe/Paris
      LOGLEVEL: debug
      INTERVAL: 5 # minutes
      # gluetun connection details
      GLUETUNIP: 127.0.0.1
      GLUETUNPORT: 8000
      # qBit connection details
      HTTPS: False
      IP: localhost
      PORT: 8080
      USERNAME: admin
      PASSWORD: secretpassword
```
