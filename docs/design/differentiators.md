# Why people would switch

Being open source gets people to try Nuqs. These features are what make them stay. Each one comes from a real problem we found in Nessus, with the evidence linked.

The first four are the core of the FYP, the rest come later.

## 1. Retro-scan: find new vulnerabilities without scanning again

**Nessus:** to know if yesterday's CVE affects you, you scan every host again. That took 14 to 17 minutes per host in our tests ([evidence](https://github.com/PremadeS/nessus-dissection/blob/main/09-live-observations.md)).

**Nuqs:** every scan saves what's installed on each host (products and versions). When new vulnerability data comes in, we match it against what we saved, instantly, with no network traffic. You get "this host is affected since 14:02" within seconds.

**Why it works:** about 70% of Nessus' 351,782 plugins are just version comparisons. Those don't need a scan, just data. More in [Keeping up with new CVEs](content-pipeline.md).

## 2. Much faster scans with an honest progress bar

**Nessus:** looks at all ~345,000 plugins for every host and skips about 98% of them while scanning. Then it sits at "99%" for most of the scan (64% to 72% of the total time in our tests).

**Nuqs:** Nmap finds the services, then we only run checks that apply to what was found, hundreds instead of hundreds of thousands. Slow checks start first, so nothing drags at the end. Progress shows what's running right now and roughly how long is left, with a "stop and keep what you have" button.

## 3. Fix-first results

**Nessus:** showed 6 separate critical and high findings on our test Mac, all fixed by the same macOS update.

**Nuqs:** the main view is grouped by fix, like "Update OpenSSH to 9.8, fixes 6 findings on 3 hosts". People think in fixes, not CVE numbers, and managers want to know what to do on Monday.

## 4. Evidence and confidence for every finding

**Nessus:** false positives from guessing versions (see the pfSense case in our [weak points](https://github.com/PremadeS/nessus-dissection/blob/main/08-weak-points-and-opportunities.md)), and the "why" is hidden in a log with 337,000 rows per host.

**Nuqs:** every finding says what we actually saw and how sure we are: **confirmed** (we proved it) or **inferred** (based on the version). One click re-runs just that check. This is already in the core, every finding has evidence.

## 5. Every scan shows what changed

Results show **new**, **fixed** and **still open** compared to the last scan, instead of a fresh pile every time.

## 6. Check credentials before scanning

A "test my credentials" step logs into each host first and says exactly what's wrong (wrong key, needs sudo, locked out). Bad credentials are the most common complaint on Nessus forums, and a scan without them quietly misses most findings.

## 7. Checks anyone can write

Most checks will be a short YAML file. Anything harder is a small Go file with one interface (see [Writing a check](../writing-a-check.md)). Every check comes with tests. Later we can import Nuclei templates and Nmap scripts too.

## 8. Built for automation

Everything the UI does, the CLI and API can do too. JSON and SARIF output, `--fail-on high` for CI pipelines, and live updates over a stream instead of the UI asking "are you done yet?" every 4 seconds like Nessus does.

## 9. A real beginner mode

One button: "Scan my network". It finds your network by itself, explains problems in plain words and ranks them by real-world risk (is it being exploited right now?), not only by CVSS score.

## 10. Light, free and private

Runs from one binary, uses little memory, works on a Raspberry Pi. Adding more scanners is free, Nessus charges for that. No telemetry.
