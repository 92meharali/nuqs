# Nuqs

Nuqs is a simple, open-source network vulnerability scanner. It uses Nmap to find hosts and open ports, then runs small checks on each service to find real problems.

We're building it as our final year project, as a simpler and more trustworthy alternative to Nessus.

## What it does today

- Finds hosts and open services with Nmap
- Runs 3 checks: open Telnet, missing web security headers, anonymous FTP login
- Shows every finding with **evidence** (what we actually saw) and a **fix**
- Prints a readable report or JSON

It's small on purpose. This is the core, and everything else gets built on top of it. See the [roadmap](roadmap.md) for what's coming.

## Quick start

```bash
go build -o nuqs ./cmd/nuqs
./nuqs scan 192.168.1.1
```

More in [Getting started](getting-started.md).

## Where to go next

| I want to... | Read |
|---|---|
| Run a scan | [Getting started](getting-started.md) |
| Understand the code | [How it works](how-it-works.md) and [Architecture](architecture.md) |
| Add a new check | [Writing a check](writing-a-check.md) |
| Run or write tests | [Testing](testing.md) |
| See the plan | [Roadmap](roadmap.md) and [What's done](progress.md) |
| Know why we made a choice | [Decisions](design/decisions.md) |

Our research on how Nessus works lives in a separate repo, [nessus-dissection](https://github.com/PremadeS/nessus-dissection).
