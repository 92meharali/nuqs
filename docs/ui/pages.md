<!-- Every screen in the web UI, what's on it, and the week we build it. -->

# Pages

Every screen in the UI. The week says when it's built, see the [roadmap](../roadmap.md).

Wireframes for each page are made in W1 and saved in `docs/ui/wireframes/`.

## For FYP-I (by 5 Dec)

### Login (W2)

Paste the token that `nuqs serve` prints on first start. Remembered in a cookie after that.

### Home (W2)

- The last scan: when it ran, how many hosts, findings by severity
- **Top 3 fixes** from that scan, each with one line on why it matters
- A big **New scan** button

### New scan (W2)

- One box for the target: an IP, a network like `192.168.1.0/24`, a range or a hostname
- A **Scan** button
- **Advanced** (folded): ports, which checks to run, speed
- If the target isn't in the allowed list, it says so before the scan starts, not after

### Scan results (W2)

The main screen. Three tabs:

- **By fix** (default): one row per fix, like "Update OpenSSH to 9.8, fixes 6 findings on 3 hosts", sorted by risk
- **By host:** each host, its open ports and its findings
- **All findings:** a plain table you can sort and filter by severity, host, check and confidence

A short **since last scan** line at the top: "2 new, 3 fixed, 10 still open".

### Finding detail (W2)

- Title, severity and **confirmed** or **inferred**
- **What we saw:** the evidence, exactly as the server sent it
- **How to fix it:** the fix text, and the CVE links if there are any
- First seen and last seen
- A **Check again** button that re-runs just this check on just this host

### History (W2)

Every past scan: when, target, how long it took, findings. Tick any two to compare them.

### Compare (W2)

Two scans side by side: **new**, **fixed** and **still open**, each as its own list.

## For FYP-II

| Page | Week | What it adds |
|---|---|---|
| Live scan | W6 | Hosts filling in as they finish, what's running right now, time left, a stop button |
| Beginner mode | W6 | A **Scan my network** button on Home, finds the local network by itself |
| Checks | W6 | Every check, what it looks for in plain words, and its fix |
| Data feeds | W6 | How fresh each vulnerability source is, an **Update now** button |
| Credentials | W7 | Add SSH logins, with a **Test** button that says exactly what's wrong |
| Verify | W9 | **Verify** and **How do I fix this?** buttons on a finding, every AI step shown |
| Reports | W10 | Download HTML, PDF, CSV or SARIF, summary or technical |
| Schedules and alerts | W10 | Run scans daily or weekly, send email or webhook alerts |
| Windows credentials | W11 | WinRM logins, with the same **Test** button |
| Workers | W12 | Scan workers on other networks, online or offline |
| Users | W12 | Admins, scanners and viewers, and the audit log |

## The look

- Clean and quiet, lots of white space, like the logo: dark ink, one teal accent
- Severity colours used only for severity: critical, high, medium, low, info
- Light and dark mode (W6)
- Works on a phone, the tables turn into cards on small screens
- System fonts, nothing to download
