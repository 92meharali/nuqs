# What's done

Last updated 19 Nov 2026.

## Research: how Nessus works

All of this lives in the [nessus-dissection](https://github.com/PremadeS/nessus-dissection) repo.

- [x] Unpacked the Nessus 10.12.5 macOS installer with our own small tools (DMG, pkg and universal binary extractors)
- [x] Found that most of Nessus (web server, API, CLI, all checks) is written in its own script language, NASL, and runs inside one big C/C++ program
- [x] Pulled out the database layout, around 280 API routes, about 400 scan settings and the full plugin function list
- [x] Decompiled the small watchdog binary with Ghidra (the main 13 MB engine is still to do)
- [x] Ran Nessus Essentials live and measured two scans: one host took 14 to 17 minutes, and 64% to 72% of that time was spent stuck at "99%"
- [x] Found that Nessus looks at all ~345,000 plugins for every host and skips about 98% of them
- [x] Collected public complaints about Nessus (slow, hard to set up, false positives, paywalls)
- [x] Wrote down 17 weak points and how Nuqs should do better

## Decisions

- [x] Language: Go (see [Decisions](design/decisions.md))
- [x] One core with the CLI now, and the API and web UI later as thin layers on top
- [x] Nmap does the low-level scanning, we don't reinvent it
- [x] Plan for keeping up with new CVEs (see [Keeping up with new CVEs](design/content-pipeline.md))
- [x] 4-month [roadmap](roadmap.md), tightened so each week does a lot more
- [x] Web UI served by the binary, no Qt desktop app (see [Decisions](design/decisions.md))
- [x] Repo moved to [nuqs-org/nuqs](https://github.com/nuqs-org/nuqs), with all commits kept
- [x] Roadmap split into FYP-I (presentation and open house on 5 Dec) and FYP-II

## Nuqs core (milestone 0)

- [x] Target validation (IPs, networks, ranges, hostnames, a safety limit, and blocking Nmap option injection)
- [x] Nmap runner and XML parser
- [x] The `Check` interface, the main way to extend Nuqs
- [x] Scan engine: picks checks per service, runs them in parallel with timeouts, survives errors, hangs and panics
- [x] 3 checks: `telnet-open`, `http-security-headers`, `ftp-anonymous-login`
- [x] Text and JSON reports, every finding has evidence and a fix
- [x] `nuqs scan`, `nuqs checks`, `nuqs version`
- [x] Tests: unit tests for everything, fake servers for each check, fuzz tests for both parsers, an integration test with real Nmap, ~97% coverage
- [x] CI: format, vet, lint (including "everything public is documented"), race detector, coverage gate, fuzzing, integration, govulncheck, build
- [x] Docs website built and published from CI, with API pages generated from code comments
- [x] Architecture page (diagram, package table and file map) generated from the code and updated by CI on every push, with a test that checks it against `go list`
- [x] Every file starts with a one-line description, the file map on the architecture page is built from them, and a test fails if one is missing
- [x] Nicer API pages: one heading per item, the description first, and a link to the code
- [x] `scripts/docs.sh` installs what the docs need and builds or serves them in one command
- [x] Zero dependencies, standard library only

## Next

Week 1 of the [roadmap](roadmap.md): version comparison, confidence on findings, the test lab, 5 checks, UI wireframes, and the first docs (background, survey, scope). Everything builds towards the FYP-I presentation on 5 Dec.
