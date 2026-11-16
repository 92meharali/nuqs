# Keeping up with new CVEs

"Will it catch last night's CVE?" is the biggest question users ask about any scanner. Here's how Nuqs stays current.

## Most CVEs are data, not code

There are two kinds of vulnerabilities, and they need different handling.

| Kind | Roughly | Example | How we handle it |
|---|---|---|---|
| **1. Version-based** | 90% | "OpenSSH older than 9.8 is vulnerable" | Vulnerability data plus one matching engine. No code per CVE, no human per CVE. |
| **2. Needs a real test** | 10% | an unauthenticated web exploit, default passwords | A real check, suggested by a bot as a pull request, merged by a human. |

Nessus writes a separate plugin for every version-based advisory (about 70% of its 351,782 plugins), and needs a full rescan to use them. We match data against saved scans instead, which is what makes retro-scan possible.

## Why we can't just use NVD

NVD is the US government's CVE database, and most scanners use it to know which products a CVE affects. But:

- On **15 April 2026** NIST stopped fully analyzing every CVE. Only the most important ones now get product data.
- Estimates say only about 15% to 20% of new CVEs will get full analysis, so around 80% won't have the product info scanners need.
- Even before that, in 2024, only about 41% of CVEs had it.

So a scanner that relies on NVD misses most new CVEs. That's a real opening for us.

## Where we get the data instead (kind 1)

| Source | What it gives us |
|---|---|
| **OSV.dev** | Affected versions for Linux distros (Debian, Ubuntu, Alpine, Red Hat, SUSE, Rocky, Alma and more) and language packages. It's all downloadable, so we keep a local copy. Distro data also knows about backported fixes, which means fewer false positives. |
| **The official CVE list** (CVEProject/cvelistV5) | Affected versions straight from the vendor, even when NVD never looks at the CVE. Also the fastest way to hear about new CVEs. |
| **CISA KEV** | Which CVEs are being exploited right now. |
| **EPSS** | How likely a CVE is to be exploited soon. |

The one part people maintain by hand is the **product mapping table**, which turns what Nmap reports (like `OpenSSH 9.6p1 Ubuntu-3ubuntu13`) into the name used in the vulnerability data. That changes through normal pull requests, a few a week at most.

## The CVE bot (kind 2)

1. **Watch.** A scheduled GitHub Action checks the CVE list and CISA KEV every 15 to 30 minutes.
2. **Filter.** It only acts when we can detect the product over the network, version data alone isn't enough, and the CVE is being exploited or likely to be. That keeps it to a few pull requests a day, not hundreds.
3. **Draft.** It writes a check in our simple YAML format (never free code), plus test cases for a vulnerable and a fixed server. Later an AI can do the drafting, and if a Nuclei template already exists we convert that instead.
4. **Test.** CI checks the format, makes sure the check is safe (read only, no exploits), and runs the test cases. When possible it also runs the check against real vulnerable and fixed Docker images.
5. **Pull request.** The PR explains the CVE, links the sources, shows the test results and how to try it locally.
6. **Human merge.** A person always reviews and merges. The bot never merges by itself.
7. **Ship.** Merged checks go out in the next signed content update, separate from the Nuqs binary, so users get them without upgrading.

## Honest limits

- AI-drafted checks will sometimes be wrong, that's why we have tests and a human review.
- Some CVEs can't be tested safely over the network. Those stay version-based and are marked "inferred".
- AI calls in CI cost money, so the bot gets a daily limit.
- Home routers and other appliances often hide their real version, so matching them is harder.

## Sources

- NVD change of April 2026: [NIST](https://www.nist.gov/node/1910421), [Help Net Security](https://helpnetsecurity.com/2026/04/16/nist-national-vulnerability-database-nvd-enrichment), [lilting.ch](https://lilting.ch/en/articles/nist-nvd-risk-based-cve-enrichment)
- Share of CVEs with product data, and impact on scanners: [Cloud Security Alliance](https://labs.cloudsecurityalliance.org/research/csa-research-note-nist-nvd-enrichment-overhaul-20260429-csa/), [CSA, enterprise impact](https://labs.cloudsecurityalliance.org/research/csa-research-note-nist-nvd-enrichment-changes-enterprise-vul/), [Socket](https://socket.dev/blog/nist-officially-stops-enriching-most-cves)
- OSV coverage and data downloads: [OSV blog](https://osv.dev/blog/posts/announcing-api-queries-for-more-linux-distros), [OSV data sources](https://google.github.io/osv.dev/data)
- EPSS and KEV don't depend on NVD: [Cogent](https://www.cogent.com/academy/nvd-processing-delays-and-what-they-mean-for-your-program)
