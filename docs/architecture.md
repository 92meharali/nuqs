# Architecture

!!! note "This page is generated"
    `tools/archdiagram` builds it from the code on every push, so it always matches what's in the repo. Don't edit it by hand. Every description comes from the file itself: the package doc comment for folders, and the one-line comment at the top of each file.

## How the packages connect

An arrow means "uses". For example `internal/cli --> internal/engine` means the CLI calls the engine.

```mermaid
flowchart LR
  subgraph n_g_checks ["checks"]
    n_checks["checks"]
    n_checks__["checks/*<br/><small>ftpanon, httpheaders, telnet</small>"]
  end
  subgraph n_g_cmd ["cmd"]
    n_cmd_nuqs["cmd/nuqs"]
  end
  subgraph n_g_internal ["internal"]
    n_internal_check["internal/check"]
    n_internal_cli["internal/cli"]
    n_internal_engine["internal/engine"]
    n_internal_model["internal/model"]
    n_internal_nmap["internal/nmap"]
    n_internal_output["internal/output"]
    n_internal_target["internal/target"]
  end
  subgraph n_g_tools ["tools"]
    n_tools_apidocs["tools/apidocs"]
    n_tools_archdiagram["tools/archdiagram"]
  end
  n_checks --> n_checks__
  n_checks --> n_internal_check
  n_checks__ --> n_internal_check
  n_checks__ --> n_internal_model
  n_cmd_nuqs --> n_checks
  n_cmd_nuqs --> n_internal_cli
  n_cmd_nuqs --> n_internal_nmap
  n_internal_check --> n_internal_model
  n_internal_cli --> n_internal_check
  n_internal_cli --> n_internal_engine
  n_internal_cli --> n_internal_nmap
  n_internal_cli --> n_internal_output
  n_internal_cli --> n_internal_target
  n_internal_engine --> n_internal_check
  n_internal_engine --> n_internal_model
  n_internal_nmap --> n_internal_model
  n_internal_output --> n_internal_engine
```

## Packages

| Package | What it does | Uses |
|---|---|---|
| `checks` | Lists every check that ships with Nuqs. | `checks/ftpanon`, `checks/httpheaders`, `checks/telnet`, `internal/check` |
| `checks/ftpanon` | Checks if an FTP server lets anyone log in as "anonymous". | `internal/check`, `internal/model` |
| `checks/httpheaders` | Checks web servers for missing security headers. | `internal/check`, `internal/model` |
| `checks/telnet` | Flags open Telnet services. | `internal/check`, `internal/model` |
| `cmd/nuqs` | The Nuqs vulnerability assessment tool. | `checks`, `internal/cli`, `internal/nmap` |
| `internal/check` | Defines what a check is. | `internal/model` |
| `internal/cli` | The "nuqs" command line. | `internal/check`, `internal/engine`, `internal/nmap`, `internal/output`, `internal/target` |
| `internal/engine` | Runs a scan from start to end. | `internal/check`, `internal/model` |
| `internal/model` | Holds the small set of types that every part of Nuqs shares. | nothing else in Nuqs |
| `internal/nmap` | Runs Nmap and turns its XML output into model.Host values. | `internal/model` |
| `internal/output` | Prints a scan report, either as a table for people or as JSON for tools. | `internal/engine` |
| `internal/target` | Checks what the user asked us to scan before it ever reaches Nmap. | nothing else in Nuqs |
| `tools/apidocs` | Writes the API reference pages of the website, one page per package, from the doc comments in the code. | nothing else in Nuqs |
| `tools/archdiagram` | Writes docs/architecture.md, a diagram and a map of the repo built straight from the code. | nothing else in Nuqs |

## What's where

Every file in the repo, as git sees it, except the `docs/` and `test/` folders. Click a folder to open or close it.

<div class="file-tree">
<ul>
<li><details><summary><code>.github/</code> <span class="note">GitHub settings.</span></summary>
<ul>
<li><details><summary><code>workflows/</code> <span class="note">CI jobs. They run on every push and pull request.</span></summary>
<ul>
<li><code>architecture.yml</code> <span class="note">Rebuilds docs/architecture.md on every push to master and commits it if it changed.</span></li>
<li><code>ci.yml</code> <span class="note">Tests, lint, fuzzing, the real Nmap test and builds, on every push and pull request.</span></li>
<li><code>docs.yml</code> <span class="note">Builds the docs website on every pull request and publishes it from the default branch.</span></li>
</ul>
</details></li>
</ul>
</details></li>
<li><details><summary><code>checks/</code> <span class="note">Lists every check that ships with Nuqs.</span></summary>
<ul>
<li><details><summary><code>ftpanon/</code> <span class="note">Checks if an FTP server lets anyone log in as &#34;anonymous&#34;.</span></summary>
<ul>
<li><code>ftpanon.go</code></li>
<li><code>ftpanon_test.go</code> <span class="note">Tests the FTP check against a small fake FTP server.</span></li>
</ul>
</details></li>
<li><details><summary><code>httpheaders/</code> <span class="note">Checks web servers for missing security headers.</span></summary>
<ul>
<li><code>httpheaders.go</code></li>
<li><code>httpheaders_test.go</code> <span class="note">Tests the web headers check against local test servers.</span></li>
</ul>
</details></li>
<li><details><summary><code>telnet/</code> <span class="note">Flags open Telnet services.</span></summary>
<ul>
<li><code>telnet.go</code></li>
<li><code>telnet_test.go</code> <span class="note">Tests the Telnet check.</span></li>
</ul>
</details></li>
<li><code>checks.go</code></li>
<li><code>checks_test.go</code> <span class="note">Makes sure every built-in check is valid, tested and listed.</span></li>
</ul>
</details></li>
<li><details><summary><code>cmd/</code> <span class="note">Programs you can run. Each folder here is one binary.</span></summary>
<ul>
<li><details><summary><code>nuqs/</code> <span class="note">The Nuqs vulnerability assessment tool.</span></summary>
<ul>
<li><code>main.go</code></li>
</ul>
</details></li>
</ul>
</details></li>
<li><details><summary><code>internal/</code> <span class="note">The core of Nuqs. Only code in this repo can import these.</span></summary>
<ul>
<li><details><summary><code>check/</code> <span class="note">Defines what a check is.</span></summary>
<ul>
<li><code>check.go</code></li>
<li><code>check_test.go</code> <span class="note">Tests when checks apply and how check lists are validated.</span></li>
</ul>
</details></li>
<li><details><summary><code>cli/</code> <span class="note">The &#34;nuqs&#34; command line.</span></summary>
<ul>
<li class="folder"><code>testdata/</code> <span class="note">Sample files used by the tests next to it (1 file).</span></li>
<li><code>cli.go</code></li>
<li><code>cli_test.go</code> <span class="note">Runs the whole CLI with a fake Nmap, so no network is needed.</span></li>
</ul>
</details></li>
<li><details><summary><code>engine/</code> <span class="note">Runs a scan from start to end.</span></summary>
<ul>
<li><code>engine.go</code></li>
<li><code>engine_test.go</code> <span class="note">Tests the engine: timeouts, panics, cancelling, the worker limit and sorting.</span></li>
</ul>
</details></li>
<li><details><summary><code>model/</code> <span class="note">Holds the small set of types that every part of Nuqs shares.</span></summary>
<ul>
<li><code>model.go</code></li>
<li><code>model_test.go</code> <span class="note">Tests severities, finding IDs and the JSON shape.</span></li>
</ul>
</details></li>
<li><details><summary><code>nmap/</code> <span class="note">Runs Nmap and turns its XML output into model.Host values.</span></summary>
<ul>
<li class="folder"><code>testdata/</code> <span class="note">Sample files used by the tests next to it (1 file).</span></li>
<li><code>nmap.go</code></li>
<li><code>nmap_test.go</code> <span class="note">Tests building the Nmap command, reading its XML and running it.</span></li>
</ul>
</details></li>
<li><details><summary><code>output/</code> <span class="note">Prints a scan report, either as a table for people or as JSON for tools.</span></summary>
<ul>
<li><code>output.go</code></li>
<li><code>output_test.go</code> <span class="note">Tests the text and JSON reports.</span></li>
</ul>
</details></li>
<li><details><summary><code>target/</code> <span class="note">Checks what the user asked us to scan before it ever reaches Nmap.</span></summary>
<ul>
<li><code>target.go</code></li>
<li><code>target_test.go</code> <span class="note">Tests target parsing and the safety limit, and fuzzes it with random input.</span></li>
</ul>
</details></li>
</ul>
</details></li>
<li><details><summary><code>scripts/</code> <span class="note">Small helper scripts used by CI.</span></summary>
<ul>
<li><code>docs.sh</code> <span class="note">Builds the docs website.</span></li>
</ul>
</details></li>
<li><details><summary><code>tools/</code> <span class="note">Helper programs for developing Nuqs. Not part of the scanner.</span></summary>
<ul>
<li><details><summary><code>apidocs/</code> <span class="note">Writes the API reference pages of the website, one page per package, from the doc comments in the code.</span></summary>
<ul>
<li class="folder"><code>testdata/</code> <span class="note">Sample files used by the tests next to it (7 files).</span></li>
<li><code>apidocs.go</code> <span class="note">Finds the packages, reads their docs and writes one Markdown page each.</span></li>
<li><code>apidocs_test.go</code> <span class="note">Tests for the API page generator, run against a small fake module.</span></li>
<li><code>main.go</code></li>
</ul>
</details></li>
<li><details><summary><code>archdiagram/</code> <span class="note">Writes docs/architecture.md, a diagram and a map of the repo built straight from the code.</span></summary>
<ul>
<li class="folder"><code>testdata/</code> <span class="note">Sample files used by the tests next to it (11 files).</span></li>
<li><code>arch.go</code> <span class="note">Reads the repo: its packages, what they import, and its files.</span></li>
<li><code>arch_test.go</code> <span class="note">Tests the generator, and checks it against go list so the diagram is always right.</span></li>
<li><code>main.go</code></li>
<li><code>notes.go</code> <span class="note">Reads the one-line description at the top of each file.</span></li>
<li><code>notes_test.go</code> <span class="note">Tests reading the one-line description at the top of each kind of file.</span></li>
<li><code>render.go</code> <span class="note">Turns what we read into the architecture page: diagram, table and file tree.</span></li>
</ul>
</details></li>
</ul>
</details></li>
<li><code>.gitignore</code> <span class="note">Files git should ignore, like build output and generated docs.</span></li>
<li><code>.golangci.yml</code> <span class="note">Lint settings.</span></li>
<li><code>LICENSE</code></li>
<li><code>README.md</code> <span class="note">The front page of the repo: what Nuqs is and how to start.</span></li>
<li><code>SECURITY.md</code> <span class="note">How to report a security problem in Nuqs.</span></li>
<li><code>go.mod</code> <span class="note">The Go module for Nuqs.</span></li>
<li><code>mkdocs.yml</code> <span class="note">Settings for the docs website (MkDocs Material).</span></li>
</ul>
</div>
