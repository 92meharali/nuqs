<!-- The web UI: what it is, why it's built this way, and how it beats Nessus. -->

# Web UI

The web UI is how most people will use Nuqs. It's a web page served by the `nuqs` binary itself: run `nuqs serve`, open the link, and you're in. Nothing else to install.

This section explains:

- **This page:** what the UI does and why it's better than Nessus
- **[Pages](pages.md):** every screen, what's on it and when we build it
- **[How we build it](building.md):** the tech, the folder layout, security and testing

## What it does

Everything the CLI does, from the browser:

1. Start a scan, or press **Scan my network** and let Nuqs find it
2. Watch it live: which hosts are done, what's running right now, time left
3. See results grouped **by fix**, with evidence and confidence on every finding
4. Fix something, scan again, and see what's **new**, **fixed** and **still open**
5. Look back at any old scan and compare any two

## Why it's better than Nessus

We measured Nessus Essentials 10.12.6 before starting, see the [research](https://github.com/PremadeS/nessus-dissection). Each row is a real problem we saw.

| | Nessus | Nuqs |
|---|---|---|
| **Getting started** | Install, register, wait for 351,000 plugins to download and compile before the first scan | One binary, `nuqs serve`, scan in under a minute |
| **Starting a scan** | Pick from 43 templates and about 390 settings | One box for the target and a Scan button. Advanced options are there, but folded away. |
| **Progress** | The page asks the server "are you done?" every 4 seconds and gets back a 9.5 KB answer. The bar sits at 99% for 64% to 72% of the scan. | The server pushes small updates as they happen. Progress shows the phase, what's running and time left, with a "stop and keep what we have" button. |
| **Results** | A long list by plugin. On our test Mac, 6 separate findings were all fixed by one update. | Grouped by fix: "Update OpenSSH to 9.8, fixes 6 findings on 3 hosts". The fix you should do first is at the top. |
| **Trusting a finding** | Version guesses cause false positives, and the "why" is buried in logs | Every finding shows what we saw and **confirmed** or **inferred**. One click re-runs just that check. |
| **Scanning again** | A fresh pile of results each time | **New**, **fixed** and **still open** compared to the last scan |
| **Reports** | Exports are blocked on the free version | Every export is free |
| **Who it's for** | Security professionals | Anyone. Plain-language explanations, a beginner mode, and depth for experts when they want it. |
| **Where it runs** | Heavy, needs a decent machine | Light enough for a Raspberry Pi, works from any device on the network, including phones |

## Rules for the UI

- **One question per screen.** Each page answers one thing: "what should I fix?", "what changed?", "is it done yet?"
- **Fixes, not CVE numbers.** CVE IDs are there, but the headline is always what to do.
- **Show the proof.** Every finding links to its evidence. We never ask people to trust us.
- **Simple by default, deep on demand.** Advanced options are folded away, not removed.
- **Never slow.** Pages load in under a second, even with thousands of findings.
- **The UI adds no logic.** It only calls the same code the CLI uses, so the CLI, the API and the UI always agree.
