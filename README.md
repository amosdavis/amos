# AMOS — Amos MIB Operating System

A cross-platform GUI MIB browser for SNMP agents, built with Go + Fyne.

## Features
- Browse the full MIB object tree (loaded from bundled + custom MIB files)
- SNMP v1, v2c, and v3 support
- Operations: GET, GETNEXT, GETBULK, WALK, SET
- Auto-scan your local subnet for responding SNMP agents
- Load additional MIB files at runtime
- OID detail view (name, syntax, access level, description)

## Public Test Agents
| Host                                    | Version | Community | Notes              |
|-----------------------------------------|---------|-----------|--------------------|
| `demo.snmplabs.com`                     | v2c     | public    | snmpsim simulator  |
| `demo.pysnmp.com`                       | v2c     | public    | PySNMP demo agent  |
| `snmp.live.gambitcommunications.com`    | v2c     | public    | Cisco device sim   |

## Build

```bash
go build ./cmd/amos
```

## Test

```bash
go test ./...
```

## BDD Tests

```bash
go test ./features/...
```

## Design

See [FAILURE_MODES.md](FAILURE_MODES.md) and [TENETS.md](TENETS.md) for the design authority of this project.
