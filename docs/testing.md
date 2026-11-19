# Testing

We don't compromise on tests. Everything has to work from the start, and CI blocks any PR that breaks something.

## Run the tests

```bash
go test ./...                 # all unit tests
go test -race ./...           # same, with the race detector (CI does this)
go test -cover ./...          # with coverage per package
```

Unit tests never need Nmap or the internet. Checks are tested against small fake servers that run inside the test.

## Integration test

This one runs the real Nmap against two local fake servers with known problems, and makes sure we find exactly those problems.

```bash
go test -tags integration -v ./test/integration
```

It skips itself if Nmap isn't installed. CI always runs it.

## End-to-end lab report

With Go, Nmap and Docker Compose installed, run `./scripts/test-http-lab.sh`.
It builds the Nuqs binary, starts nginx on `127.0.0.1:18080`, waits for its
static page, and runs the binary against that port. Keep port 18080 free for
this test. Nuqs uses Nmap to discover the service and selects its checks from
the built-in registry.

Expect one HTTP header finding, no check errors, and a message confirming the
report matches `lab/expected-report.json`. The comparison validates the scan
timestamps and substitutes two fixed values. Changes to the inventory,
findings or errors produce a unified diff. Review that diff before updating
the expected report. The script removes the lab container on exit.

## Fuzz tests

Anything that reads untrusted input has a fuzz test. Fuzzing throws millions of random inputs at it to find crashes.

```bash
go test ./internal/target -run='^$' -fuzz=FuzzParse -fuzztime=30s
go test ./internal/nmap   -run='^$' -fuzz=FuzzParse -fuzztime=30s
```

If fuzzing finds a crash, Go saves the input under `testdata/fuzz/`. Commit that file with the fix so it's tested forever.

## What CI checks

Every push and PR runs:

| Check | Fails when |
|---|---|
| `gofmt` | code isn't formatted |
| `go vet` and `golangci-lint` | common bugs, or a public function, type or package has no doc comment |
| Unit tests with `-race` | any test fails or there's a data race |
| Coverage | total coverage drops below **85%** |
| Fuzz (short) | a fuzz target crashes in 30 seconds |
| Integration | the real Nmap scan doesn't find what it should |
| Lab report | the built CLI’s report for nginx differs from the saved JSON |
| Build | the binary doesn't build or `nuqs version` fails |
| govulncheck | a known vulnerability affects our code or Go version |
| Docs | the website doesn't build (broken links and so on) |
| Architecture | the diagram generator disagrees with `go list` about how packages connect |

## Rules for tests

- Every bug fix comes with a test that fails without the fix.
- Every check has positive, negative and "server misbehaves" tests.
- Tests must be fast and never touch the real network.
- Table-driven tests are preferred, they're easy to extend.
