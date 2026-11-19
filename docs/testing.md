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

## HTTP lab report

With Docker Compose running, run `./scripts/test-http-lab.sh`. It serves one
static page on a temporary loopback port, runs the CLI report test, and removes
the container when the script exits. Expect one low-severity HTTP header
finding and an empty error list.

The test supplies a saved Nmap inventory with one HTTP service. The header
check reads the response from the container, and the CLI writes its JSON
report. Using the same service address in the inventory keeps finding IDs and
evidence reproducible. The integration test above covers live discovery.

The expected report lives in `internal/cli/testdata/http-report.json`. The
test validates the start and finish times and gives them fixed values for the
comparison. Review changes to the expected JSON alongside the change that
caused them. The same comparison runs with an in-process HTTP fixture during
`go test ./...`.

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
| HTTP lab report | the Compose fixture produces a different JSON report |
| Build | the binary doesn't build or `nuqs version` fails |
| govulncheck | a known vulnerability affects our code or Go version |
| Docs | the website doesn't build (broken links and so on) |
| Architecture | the diagram generator disagrees with `go list` about how packages connect |

## Rules for tests

- Every bug fix comes with a test that fails without the fix.
- Every check has positive, negative and "server misbehaves" tests.
- Tests must be fast and never touch the real network.
- Table-driven tests are preferred, they're easy to extend.
