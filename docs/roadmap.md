# Roadmap (16 Nov 2026 to 16 Mar 2027)

Four months, 17 weeks, one clear goal per week. Each week builds on the one before, so we do them in order.

Every week follows the same rules:

- Each task ships with its **tests** and **doc comments** in the same PR. No "tests later".
- CI must be green on `main` at the end of every week.
- Friday: a 10 minute demo to each other of what works now.
- If a week slips, we cut scope inside the week, we don't push the whole plan back.

What's already done is in [What's done](progress.md).

---

## The order, at a glance

| # | Week | Dates | What we build |
|---|---|---|---|
| 1 | W1 | 16 to 22 Nov | Team setup, repo, CI and website live |
| 2 | W2 | 23 to 29 Nov | Test lab and two more checks |
| 3 | W3 | 30 Nov to 6 Dec | Version comparison and the first version-based check |
| 4 | W4 | 7 to 13 Dec | Saving scans (SQLite) and scan history |
| 5 | W5 | 14 to 20 Dec | Comparing scans (new, fixed, still open) |
| 6 | W6 | 21 to 27 Dec | Host inventory (light week, holidays) |
| 7 | W7 | 28 Dec to 3 Jan | Vulnerability data sync, part 1 (OSV) |
| 8 | W8 | 4 to 10 Jan | Vulnerability data sync, part 2 (KEV, EPSS, CVE list) + **mid-point demo** |
| 9 | W9 | 11 to 17 Jan | Matching advisories to hosts, and retro-scan |
| 10 | W10 | 18 to 24 Jan | Fix-first grouping and a signed content feed |
| 11 | W11 | 25 to 31 Jan | REST API, live progress and MCP server |
| 12 | W12 | 1 to 7 Feb | Web UI, part 1 (run a scan, watch it, see results) |
| 13 | W13 | 8 to 14 Feb | Web UI, part 2 (fix-first view, beginner mode, defence model) |
| 14 | W14 | 15 to 21 Feb | Credential check and SSH logged-in scanning |
| 15 | W15 | 22 to 28 Feb | Simple YAML checks, the CVE bot and the verifier model |
| 16 | W16 | 1 to 7 Mar | Reports and the benchmark against Nessus |
| 17 | W17 | 8 to 14 Mar | Freeze: hardening, docs, thesis, demo, v1.0 |
| | Buffer | 15 to 16 Mar | Final submission |

---

## Month 1: a solid base

### W1 (16 to 22 Nov): team setup, CI and website live

**Goal:** everyone can build, test and ship to `main`.

- [ ] Push this repo to `github.com/PremadeS/nuqs`
- [ ] Turn on GitHub Pages (Settings, Pages, Source: GitHub Actions) so the website goes live at `premades.github.io/nuqs`
- [ ] Protect `main`: PRs only, one review, CI must pass
- [ ] Pick a license (we suggest Apache 2.0) and add `LICENSE`
- [ ] Every teammate: install Go and Nmap, build Nuqs, run a scan of their own router, read [How it works](how-it-works.md)
- [ ] Each teammate opens one tiny PR (a doc fix is fine) to learn the flow
- [ ] Split ownership: engine and checks, data and storage, API and UI

**Tests:** CI runs on every PR (unit, race, lint, fuzz, integration, docs).
**Done when:** the website is live, CI is green, and everyone has merged one PR.

### W2 (23 to 29 Nov): test lab and two more checks

**Goal:** a repeatable lab of vulnerable targets, so every check is tested against real software.

- [ ] `lab/docker-compose.yml` with real services: vsftpd with anonymous login, a Telnet server, nginx without security headers, an old OpenSSH, a TLS server allowing TLS 1.0
- [ ] `make lab-up` and `make lab-down` (or a short script)
- [ ] New CI job that starts the lab and scans it, and the result must match a saved expected report
- [ ] Check `tls-deprecated-protocols`: server accepts TLS 1.0 or 1.1
- [ ] Check `ssh-password-auth`: SSH server accepts password logins

**Tests:** each new check has fake-server unit tests plus a lab test. The lab scan's expected findings are saved in the repo, any difference fails CI.
**Done when:** `nuqs scan` on the lab finds exactly the 5 expected problems, in CI.

### W3 (30 Nov to 6 Dec): version comparison and the first version-based check

**Goal:** be able to say "this version is older than the fixed one" correctly. This is the base of everything in month 2.

- [ ] `internal/version` package: compare versions the way each ecosystem does (plain semver, Debian like `1:2.3-4ubuntu1`, RPM, and OpenSSH style `9.6p1`)
- [ ] Check `openssh-vulnerable-version`: compares the OpenSSH version Nmap found against a small hard-coded list of known bad ranges
- [ ] Add a `confidence` field to findings: `confirmed` (we proved it) or `inferred` (based on version only)

**Tests:** big table tests for version comparison taken from real Debian and RPM rules, plus a fuzz test (comparison must never crash and must be consistent: if a < b then b > a).
**Done when:** the OpenSSH check correctly flags the old OpenSSH in the lab and is marked `inferred`.

### W4 (7 to 13 Dec): saving scans and scan history

**Goal:** scans are saved, so we can look back and compare later.

- [ ] Add SQLite (`modernc.org/sqlite`, pure Go, so no C compiler needed). This is our first dependency, so note why in [Decisions](design/decisions.md).
- [ ] `internal/store` with tables for scan runs, hosts, services, findings and check errors
- [ ] Simple numbered migrations that run on startup
- [ ] `nuqs scan` saves every run (with `--no-save` to skip)
- [ ] `nuqs history` lists past runs, `nuqs show <run>` prints one

**Tests:** store tests run on a temporary database, migrations are tested from empty and from every older version, and the saved and loaded report must be identical.
**Done when:** you can scan, close everything, and see the same report again with `nuqs show`.

---

## Month 2: knowing about new vulnerabilities

### W5 (14 to 20 Dec): comparing scans

**Goal:** every scan shows what changed since the last one.

- [ ] Use the stable finding IDs to compare two runs: **new**, **fixed**, **still open**
- [ ] Each finding keeps `first_seen` and `last_seen`
- [ ] `nuqs diff <old> <new>`, and the normal scan output shows a short "since last scan" summary
- [ ] `--fail-on high` makes `nuqs scan` exit with code 3 if there's a finding at or above that severity (for CI use)

**Tests:** diff cases (nothing changed, all fixed, all new, mixed, host disappeared), exit code tests.
**Done when:** fixing a problem in the lab and scanning again shows it as "fixed".

### W6 (21 to 27 Dec): host inventory (light week)

**Goal:** turn what Nmap tells us into a clean product list we can match against vulnerability data.

- [ ] Read the CPE values Nmap already gives (`cpe:/a:openbsd:openssh:9.6p1`)
- [ ] A small, hand-written **product mapping table** (YAML file) that maps Nmap's names to the names used in vulnerability databases
- [ ] Save the inventory with each scan

**Tests:** mapping tests using real Nmap XML saved from the lab and from our router scans.
**Done when:** every service in the lab has the right product and version in the inventory.

### W7 (28 Dec to 3 Jan): vulnerability data sync, part 1 (OSV)

**Goal:** a local copy of open vulnerability data that updates itself.

- [ ] `nuqs update` downloads OSV data for the ecosystems we support first (Debian, Ubuntu, Alpine, and the packages behind common network services)
- [ ] Store it in a separate local database, with the time of the last update
- [ ] Only download what changed when possible

**Tests:** import tests on small saved OSV files (no network in tests), a corrupt download must not damage the existing data.
**Done when:** `nuqs update` works offline from a saved file and online from OSV.

### W8 (4 to 10 Jan): vulnerability data sync, part 2, and the mid-point demo

**Goal:** add the data that tells us what's actually dangerous.

- [ ] CISA KEV (known exploited) and EPSS (chance of being exploited) feeds
- [ ] The official CVE list (CVEProject/cvelistV5) for vendor-supplied affected versions
- [ ] **Mid-point demo** to supervisors: scan the lab, save, fix something, rescan, show the diff, show `nuqs update`

**Tests:** import tests for each feed using saved samples.
**Done when:** the demo runs end to end without touching anything by hand.

### W9 (11 to 17 Jan): matching advisories to hosts, and retro-scan

**Goal:** our headline feature. New vulnerabilities are matched against saved inventories, with no rescan.

- [ ] Match inventory products and versions against the advisory data (using `internal/version`)
- [ ] Findings from advisories are marked `inferred` and link to the advisory
- [ ] `nuqs rematch` re-checks every saved inventory against the latest data, and `nuqs update` runs it automatically
- [ ] Show "affected since <time>" for findings found this way

**Tests:** matching tests with hand-made advisories (in range, out of range, fixed exactly at the version, distro backports), plus a test that rematch finds a "new" advisory without any network scan.
**Done when:** adding a fake advisory for the lab's OpenSSH and running `nuqs update` flags it within seconds.

### W10 (18 to 24 Jan): fix-first grouping and a signed content feed

**Goal:** show people what to do, not just what's wrong, and ship checks and data safely.

- [ ] Group findings by fix, like "Upgrade OpenSSH to 9.8, fixes 6 findings on 3 hosts"
- [ ] `nuqs scan --by-fix` and the same grouping in JSON
- [ ] A content feed (checks data, mapping table, advisory snapshot) built and **signed** by CI, separate from the binary
- [ ] `nuqs update` checks the signature before using anything

**Tests:** grouping tests, signature tests (a changed byte must be rejected, a wrong key must be rejected).
**Done when:** CI publishes a signed feed and a tampered feed is refused.

---

## Month 3: API and web UI

### W11 (25 to 31 Jan): REST API, live progress and MCP server

**Goal:** `nuqs serve` so the UI (and other tools) can use Nuqs, and `nuqs mcp` so AI assistants can too.

- [ ] `nuqs serve` with the standard `net/http` server
- [ ] Endpoints: start a scan, list scans, get a scan, get findings, get diff, list checks
- [ ] Live progress with Server-Sent Events, using the engine's existing events
- [ ] Simple token login, the token is printed on first start
- [ ] OpenAPI file, checked in CI against the real routes
- [ ] `internal/tools`: one small set of safe actions (list checks, scan, get findings, get evidence, diff). The API, the MCP server and later the verifier all use these same functions, so they always behave the same.
- [ ] `nuqs mcp`: an MCP server so assistants like Claude or Cursor can use Nuqs. Uses the official Go MCP SDK, our second dependency, so note why in [Decisions](design/decisions.md).
- [ ] **Allowed targets** in the config file. An AI can only scan addresses listed there, and scans ask for confirmation. Read-only by default.

**Tests:** every endpoint tested with `httptest`, including bad input and missing token. Live progress tested end to end with a fake scanner. MCP tested with a test client: every tool works, and scanning outside the allowed targets is always refused.
**Done when:** a scan can be started and followed with `curl` alone, and Claude Desktop can scan the lab through `nuqs mcp`.

### W12 (1 to 7 Feb): web UI, part 1

**Goal:** run a scan from the browser and watch it.

- [ ] Pick the UI approach on Monday. We suggest plain HTML with htmx served by the Go binary (one binary, nothing to install), unless the UI owner prefers Svelte.
- [ ] Pages: new scan, live scan (progress from the API), results list, finding detail with evidence and fix
- [ ] The UI is embedded in the `nuqs` binary

**Tests:** a browser test (Playwright) that starts a scan against the lab and sees the findings appear. Runs in CI.
**Done when:** a teammate who never saw the UI can run a scan without help.

### W13 (8 to 14 Feb): web UI, part 2, and the defence model

**Goal:** the screens that make people switch from Nessus, plus exact fix steps for each host.

- [ ] **Fix-first view** as the main results screen
- [ ] **Changes since last scan** view (new, fixed, still open)
- [ ] **Beginner mode:** one button, "Scan my network", which finds the local network by itself
- [ ] Plain-language explanations for each check
- [ ] **Defence model:** a "How do I fix this?" button. An AI writes the exact fix steps for that host, using the evidence, the detected OS and version, and our fix data (like the exact `apt` command and what to restart).
- [ ] The AI's answer is clearly marked as AI-written, shown next to our own fix text, and Nuqs never runs it.
- [ ] Model setup: a local model with Ollama by default, or any OpenAI-compatible endpoint the user sets in the config. Nuqs works fully without any model.

**Tests:** browser tests for each screen, plus a test for the local network detection. The defence model is tested with a fake model: the right facts go in, the answer is labelled as AI, and nothing is ever executed. Also a test that everything works with no model set up.
**Done when:** someone non-technical can scan their home network, understand the top fix, and get exact steps for their machine.

---

## Month 4: depth, automation and finishing

### W14 (15 to 21 Feb): credential check and SSH logged-in scanning

**Goal:** see installed packages, which is where most real findings come from.

- [ ] **Credential check:** test SSH logins before a scan and say exactly what's wrong (wrong key, needs sudo, locked out)
- [ ] With working SSH credentials, collect the installed package list (dpkg, rpm, apk) into the inventory
- [ ] Advisory matching (from W9) now works on installed packages too
- [ ] Credentials are never written to logs or reports

**Tests:** an SSH server in the lab with known old packages, the credential check against good and bad logins, and a test that credentials never appear in any output.
**Done when:** a logged-in scan of the lab Debian box shows its outdated packages grouped by fix.

### W15 (22 to 28 Feb): simple YAML checks, the CVE bot and the verifier model

**Goal:** add checks without writing Go, get new checks suggested automatically, and let users prove a finding is real with one click.

- [ ] A small YAML check format for the common cases (banner match, HTTP request and response match, version range). Only safe, read-only actions are allowed.
- [ ] **CVE bot:** a scheduled GitHub Action that watches the CVE list and CISA KEV, filters to things we can detect over the network, and opens a PR with a draft YAML check, its tests and how to verify it
- [ ] A human always reviews and merges. The bot never merges.
- [ ] **Verifier model:** a "Verify" button on each finding. The AI plans extra checks, using only the safe tools from W11: confirm the version from a second source, read the package list over SSH, read a config value, or run a harmless test of the vulnerable behaviour.
- [ ] **The AI is never the proof.** A finding only changes from "inferred" to "confirmed" when one of those tools returns hard evidence. If nothing proves it, it stays "inferred" and we say why.
- [ ] Everything the verifier ran and saw is saved with the finding, so the user can read exactly how it was confirmed.
- [ ] Limits: only runs when the user clicks, only on that one host and port, read-only, no exploits, a time and step limit, and the same allowed-targets list as MCP.

**Tests:** YAML loader tests (good files, bad files, unsafe actions rejected), and the bot tested on saved CVE samples so it doesn't need the internet in CI. The verifier is tested with a fake model that tries to break the rules: it must not be able to touch another host, run anything outside the safe tools, go past the step limit, or confirm a finding without real evidence.
**Done when:** the bot opens a real PR for a real recent CVE, and verifying the lab's old OpenSSH turns it into "confirmed" with the proof shown.

### W16 (1 to 7 Mar): reports and the benchmark against Nessus

**Goal:** something to hand to a manager, and numbers for the thesis.

- [ ] HTML report (fix-first, with evidence) and SARIF output for CI tools
- [ ] **Benchmark:** scan the same lab with Nessus Essentials and Nuqs. Compare time, findings, false positives and missed findings. Save the raw results.
- [ ] Measure the verifier: false positives before and after verifying, and how long a verify takes. These real numbers go in the thesis.
- [ ] Fix the slowest parts found by the benchmark

**Tests:** report output tests (HTML is valid, SARIF matches its schema).
**Done when:** the benchmark table is in the thesis and reproducible with one script.

### W17 (8 to 14 Mar): freeze

**Goal:** nothing new, everything solid.

- [ ] Feature freeze on Monday, only fixes after that
- [ ] Long fuzzing runs (a few hours per target) and fix anything found
- [ ] Every doc page reviewed and up to date
- [ ] Final demo script, rehearsed twice
- [ ] Tag and release **v1.0** with binaries for Linux and macOS
- [ ] Thesis chapters on design, testing and results finished

**Done when:** v1.0 is released, the demo works twice in a row, the thesis is ready.

### Buffer (15 to 16 Mar)

Final submission. Anything that slipped gets cut, not rushed.

---

## Later (after the FYP)

Things we know we want but won't fit in 4 months:

- Windows logged-in scanning (WinRM)
- More scan workers on other machines (distributed scanning)
- Importing Nuclei templates and Nmap NSE scripts as checks
- A Windows build
- Letting an AI draft the CVE bot's checks (moved out of W15 to make room for the verifier)
- Scheduled scans and email or webhook alerts
