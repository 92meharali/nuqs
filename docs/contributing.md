# Contributing

## The flow

1. Pick a task from the [roadmap](roadmap.md) and open an issue (or comment on one) so nobody does it twice.
2. Make a branch from `main`, like `feat/redis-check` or `fix/ftp-timeout`.
3. Write the code **and** the tests **and** the doc comments.
4. Run `go test -race ./...` and `golangci-lint run` before pushing.
5. Open a PR. Keep it small, one thing per PR.
6. One teammate reviews it. CI must be green before merging.

## Keep it simple

If it can be simple, it should be simple. Before adding a library, a config option or an abstraction, ask "do we need this right now?". Usually the answer is no.

## Docs

- Every public function, type and package needs a doc comment. CI checks this.
- Comments explain **why**, the code already shows what.
- Keep docs short and plain, like you're explaining it to a teammate. Use commas, not semicolons.
- The API pages on the website are generated from these comments, so you never edit them by hand.
- If you change how something works, update the matching page in `docs/` in the same PR.

## Commit messages

Short and clear, like `add redis-no-auth check` or `fix ftp check hanging on silent servers`.
