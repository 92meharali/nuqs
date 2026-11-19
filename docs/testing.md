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

## Lab reports

With Docker Compose running, run `./scripts/test-http-lab.sh`. The job compares
three CLI reports against the saved `*-report.json` files in
`internal/cli/testdata/`.

| Case | Input | Expected report |
|---|---|---|
| HTTP | A static page from a disposable container | One low-severity finding listing the missing headers |
| Telnet | A saved inventory with two Telnet ports, one open and one closed | One medium-severity finding on the open port (2323/tcp) |
| FTP over TLS | A saved inventory describing an encrypted FTP service | Zero findings and one check error explaining that FTP over TLS is unsupported |

Each case uses a saved Nmap inventory and the corresponding built-in check.
Only the HTTP case contacts a service. Compose assigns a temporary loopback
port and the script removes the container on exit. The integration test above
covers live discovery.

The test validates the report timestamps and gives them fixed values before
comparing the complete JSON. Review expected report changes alongside the
changes that caused them. All three comparisons also run in `go test ./...`,
which uses an in-process HTTP fixture.

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
| Lab reports | an HTTP, Telnet or FTPS report differs from its expected JSON |
| Build | the binary doesn't build or `nuqs version` fails |
| govulncheck | a known vulnerability affects our code or Go version |
| Docs | the website doesn't build (broken links and so on) |
| Architecture | the diagram generator disagrees with `go list` about how packages connect |

## Rules for tests

- Every bug fix comes with a test that fails without the fix.
- Every check has positive, negative and "server misbehaves" tests.
- Tests must be fast and never touch the real network.
- Table-driven tests are preferred, they're easy to extend.
