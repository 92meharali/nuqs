# Nuqs

[![CI](https://github.com/PremadeS/nuqs/actions/workflows/ci.yml/badge.svg)](https://github.com/PremadeS/nuqs/actions/workflows/ci.yml)
[![Docs](https://github.com/PremadeS/nuqs/actions/workflows/docs.yml/badge.svg)](https://premades.github.io/nuqs/)

A simple, open-source network vulnerability scanner built on Nmap. Our final year project, aiming to be a simpler and more trustworthy alternative to Nessus.

**Docs:** https://premades.github.io/nuqs/

## Quick start

You need Go 1.24+ and Nmap.

```bash
go build -o nuqs ./cmd/nuqs
./nuqs scan 192.168.1.1
./nuqs scan --json -p 21,23,80,443 192.168.1.0/24 > report.json
```

Only scan networks you own or have permission to scan.

## What's here

The full, always up to date map (diagram plus every file) is in [docs/architecture.md](docs/architecture.md). It's rebuilt from the code on every push.

| Folder | What |
|---|---|
| `cmd/nuqs` | the `nuqs` binary |
| `internal/` | the core: targets, Nmap, engine, checks interface, output, CLI |
| `checks/` | built-in checks, one folder each |
| `test/integration` | full scan test with the real Nmap |
| `docs/` | the website, including the [roadmap](docs/roadmap.md) and [what's done](docs/progress.md) |

## Develop

```bash
go test -race ./...                                  # unit tests
go test -tags integration ./test/integration         # needs nmap
golangci-lint run                                    # lint, also checks every public thing has a doc comment
go run ./tools/archdiagram                           # rebuild the architecture page (CI does this on every push)
./scripts/docs.sh                                    # docs website at http://127.0.0.1:8000 (installs what it needs)
```

Adding a check? See [Writing a check](docs/writing-a-check.md). Everything else: [Contributing](docs/contributing.md).

## Research

How Nessus works under the hood, measured and written up before we started: [PremadeS/nessus-dissection](https://github.com/PremadeS/nessus-dissection).
