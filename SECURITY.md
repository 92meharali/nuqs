<!-- How to report a security problem in Nuqs. -->

# Security policy

## Supported versions

Nuqs is still before v1.0. Only the latest code on `master` gets security fixes.

## Reporting a problem

Please don't open a public issue for a security bug.

Report it privately here: [Report a vulnerability](https://github.com/nuqs-org/nuqs/security/advisories/new).

Tell us what you found, how to reproduce it, and what version or commit you used.

## What to expect

- We reply within 7 days.
- We fix it, or tell you why we won't.
- Once it's fixed, we publish an advisory and credit you, unless you'd rather stay anonymous.

We're a small student team, so please give us time before talking about it publicly.

## What counts

A security bug is a problem in Nuqs itself, for example:

- a scanned server can crash Nuqs or make it use lots of memory
- a scanned server can run code or write files on the machine running Nuqs
- a check changes something on the target, checks must be read only

A check that misses a problem or reports a false one isn't a security bug. Open a normal issue for that.

## Using Nuqs safely

Only scan networks you own or have permission to scan.
