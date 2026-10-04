# 00: Development setup

Status: Done
Depends on: none
Roadmap milestone: none (groundwork for milestone 1)

## Scope

Delivers:

- A Go module at `github.com/FreddieTheObserver/stubborn` with no dependencies and one package, `stubborn`, that holds only its doc comment.
- `make check`, the one command that gates every commit: formatting, `go mod tidy`, a cgo check, `go vet`, staticcheck, and the tests under the race detector.
- staticcheck pinned in a separate modfile, so the library's own `go.mod` lists only what the library imports.
- A GitHub Actions workflow that runs `make check` on every push and pull request.
- `.gitignore`, and a root `CLAUDE.md` for the `/sync` skill to maintain.
- The README's Development section rewritten to describe the targets that exist instead of the planned ones.

Does not deliver (and where it goes):

- `modernc.org/sqlite`: the milestone 1 slice that adds the SQLite store, because `go mod tidy` drops a dependency that nothing imports.
- The `failpoints` build tag and the kill-test harness: the milestone 1 kill-tests slice.
- Postgres in Docker and `make test-pg`: the first slice of milestone 4.
  Docker Desktop's WSL integration is off on this machine and has to be turned on by then.
- `make torture`: milestone 5.
- The directories from the README layout: each one arrives with the slice that puts code in it.

## Done when

- [x] `make check` passes in a fresh clone on a machine with Go 1.26 and gcc (clone into a temporary directory and run `make check` there).
- [x] Every gate in `make check` fails when it should: an unformatted file, an untidy `go.mod`, a dependency that uses cgo, a vet finding, a staticcheck finding, and a data race each make it exit non-zero (run once by hand, results recorded under Changes).
- [x] The library's `go.mod` has no requirements (`go list -m all` prints only `github.com/FreddieTheObserver/stubborn`).
- [x] CI runs `make check` on a push and passes (the first workflow run on GitHub after the slice is pushed).

## Design

| Function | File | New or modified | Author | What | Why |
| -------- | ---- | --------------- | ------ | ---- | --- |
| module | `go.mod` | New | assist | Module `github.com/FreddieTheObserver/stubborn`, `go 1.26.0`, no requirements. | The path is fixed in the README. `go mod init` under Go 1.26 writes `go 1.25.0`, so the directive is raised by hand to the one toolchain the project develops and tests on. |
| package `stubborn` | `doc.go` | New | assist | The package doc comment and nothing else. | `go vet ./...` and `go test ./...` exit 1 when `./...` matches no package, so the gates need one package to run against. |
| `make test` | `Makefile` | New | assist | `go test -race ./...` | From milestone 1 the engine runs a workflow goroutine, a heartbeat loop and a claim loop side by side, so a data race should fail the tests from the first line of engine code. `-race` needs cgo, so gcc is a development requirement even though the library is pure Go. |
| `make vet`, `make lint` | `Makefile` | New | assist | `go vet ./...`, and staticcheck through `go tool -modfile=tools/go.mod staticcheck ./...`. | vet ships with Go. staticcheck adds what vet leaves out (unused code, deprecated APIs, needless complexity) with very few false positives. |
| `make fmt`, `make tidy` | `Makefile` | New | assist | `gofmt -w .` and `go mod tidy`. | The fixers for what `make check` reports. |
| `make check` | `Makefile` | New | assist | Fails if `gofmt -l .` prints anything or `go mod tidy -diff` reports a change, then runs the cgo check, vet, lint and test, cheapest first. | One command for CI and for before a commit, so the two cannot drift. |
| cgo check | `Makefile` | New | assist | `CGO_ENABLED=1 go list -deps -f '{{if and (not .Standard) .CgoFiles}}{{.ImportPath}}{{end}}' ./...` must print nothing. | Holds the README's "pure Go, no cgo" choice for anyone who imports the library. See Alternatives for why `CGO_ENABLED=0 go build` cannot. |
| tool pins | `tools/go.mod`, `tools/go.sum` | New | assist | staticcheck 2026.2.1 pinned with a `tool` directive. | Every machine and CI run the same version with nothing installed globally, and the library's `go.mod` stays limited to what it imports. |
| CI | `.github/workflows/ci.yml` | New | assist | On push and pull request: `ubuntu-latest`, `actions/setup-go` with `go-version-file: go.mod` and `cache-dependency-path: tools/go.sum`, then `make check`. | The Go version comes from `go.mod`, so CI cannot test a different toolchain than the one the module declares. The cache path is explicit because the root module has no `go.sum` until it has a dependency. |
| ignores | `.gitignore` | New | assist | `bin/`, coverage profiles, SQLite files (`*.db`, `*.db-wal`, `*.db-shm`), `.claude/settings.local.json`. | `.claude/skills/` stays tracked, so `/sync` travels with the repository. |
| agent notes | `CLAUDE.md` | New | assist | Generated with `/init`, then trimmed to the make targets, the slice workflow, and pointers into the README. | `/sync` maintains this file and stops if it does not exist. |
| Development section | `README.md` | Modified | assist | The targets that exist, and the requirements: Go 1.26, gcc for `-race`, the repository on the Linux filesystem. | The README is the design, so it must describe the setup as built. |

Author is who writes the code: `user`, `assist`, or `both`.

Invariants touched:

- None.
  This slice adds no engine code.

Alternatives considered:

- golangci-lint instead of staticcheck.
  It runs staticcheck's checks plus dozens more behind a config file, which means more to tune and more noise for little gain at this size.
  Revisit if staticcheck misses something that matters.
- The `tool` directive in the library's own `go.mod`.
  Simpler, but staticcheck's dependencies (`golang.org/x/tools`, `golang.org/x/mod` and others) would become requirements of the library, and minimum version selection would raise them in every program that imports stubborn.
- `go install` on each machine.
  Unpinned, so two machines can disagree about lint results.
- `CGO_ENABLED=0 go build ./...` as the cgo check.
  It passes even with `github.com/mattn/go-sqlite3` imported, because that driver compiles to a stub without cgo and fails only at run time (checked in a scratch module).
  Running the whole test suite with cgo disabled would catch it, but costs a second test run, because `-race` needs cgo.
- `go 1.25.0`, which is what `go mod init` writes.
  It still has `testing/synctest.Test` and admits one older toolchain.
  Nobody imports stubborn yet, and supporting 1.25 for real would need a second toolchain in CI, so one version everywhere wins.
- Postgres in Docker now.
  Nothing would use it until milestone 4, and setup that nothing exercises breaks without anyone noticing.
- Stub directories for the whole README layout.
  Empty packages prove nothing, and the layout may still move during milestone 1.
- just, mage or Taskfile instead of Make.
  Make is already installed, warden uses it, and the README names make targets.

Open questions:

- None.

## Changes

Commits:

- `908a654` Add slice template and slice 00 design
- `57d64ab` Set up module, make check, and CI (slice 00)

Planned vs actual:

Every row of the Design table was built as planned.
`make check` passes, and `go list -m all` prints only the module itself.
After `57d64ab`, `make check` passed in a `git clone` with an empty `GOPATH`, module cache and build cache.

Each gate was then broken in its own fresh copy:

| Injected defect | `make check` exit | Failed at |
| --------------- | ----------------- | --------- |
| none | 0 | |
| `var  X = 1` in a new file | 2 | `fmt-check`, naming the file |
| unused `require golang.org/x/sync` | 2 | `tidy-check`, printing the diff |
| `import _ "github.com/mattn/go-sqlite3"`, tidied | 2 | `cgo-check`, naming `github.com/mattn/go-sqlite3` |
| `fmt.Printf("%d", "s")` | 2 | `vet`, printf check |
| unused unexported function | 2 | `lint`, staticcheck U1000 |
| unsynchronized write from a goroutine in a test | 2 | `test`, with `WARNING: DATA RACE` |

The first CI run, on the push of `57d64ab`, passed in 38 seconds on Go 1.26.0 read from `go.mod`, with every gate in its log ([run 37210984269](https://github.com/FreddieTheObserver/stubborn/actions/runs/37210984269)).

Deviations from the design, and why:

- The format, tidy and cgo checks are their own targets (`fmt-check`, `tidy-check`, `cgo-check`), which `check` runs as prerequisites in order, instead of steps inside `check`'s recipe.
  Each gate can then be run alone, and a failure names the gate.
- The workflow sets `permissions: contents: read`, so its token can do nothing but check out the code.
  The actions are `actions/checkout@v7` and `actions/setup-go@v7`, the current majors.

README or invariant updates this caused:

- Development section rewritten as planned.
- The status line said "No code yet", which is no longer true, so it says "No engine code yet".
- The Layout lists `doc.go` and `tools/`.

What surprised me:

- `go mod tidy -modfile=tools/go.mod` copies the library's dependencies into the tools file, because tidy still reads the root module's imports (checked with `mattn/go-sqlite3` in a scratch copy).
  Tools are changed only with `go get -tool -modfile=tools/go.mod`, and `CLAUDE.md` says so.
