# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

stubborn is a Go library for durable workflows: every step's result is checkpointed in SQLite or Postgres, and recovery is replaying the workflow function from the top.
`README.md` is the design and the source of truth for behavior.
Read its Guarantees, Invariants, Rules for workflow code, and Testing sections before changing engine code.
There is no engine code yet.

## Commands

- `make check` runs everything CI runs: gofmt, `go mod tidy -diff`, the cgo check, `go vet`, staticcheck, and `go test -race`.
  It must pass before a commit.
- `make fmt` and `make tidy` fix what it reports about formatting and `go.mod`.
- One test: `go test -race -run '^TestName$' ./path/to/pkg`.

staticcheck is pinned in `tools/go.mod` and runs as `go tool -modfile=tools/go.mod staticcheck`.
Change tool versions with `go get -tool -modfile=tools/go.mod <pkg>@<version>`, never in the root `go.mod`, which lists only what the library imports.
Never run `go mod tidy -modfile=tools/go.mod`: it copies the library's dependencies into the tools file.
No package in the build may use cgo, but gcc must be installed because `-race` needs it.

## Slices and branches

Work is planned as slices in `docs/slices/NN-short-name.md`, copied from `docs/slices/TEMPLATE.md`, which defines the status lifecycle and the branch step that goes with each status.
Write code for a slice only once its doc is Ready.
From In progress on, its Scope, Done when and Design sections are frozen, and deviations are recorded under Changes.
The Author column of each Design row (`user`, `assist`, `both`) says who writes that code.

A ruleset on `main` blocks direct pushes.
Every change, slice or not, reaches `main` through a pull request that passes the `check` job and is merged with a merge commit.
A slice's branch is `slice/NN-short-name`, and other changes use a short descriptive branch name.
Squash and rebase merges are disabled because slice docs list commit hashes, so do not rewrite a branch's history after its slice doc lists them.
The user merges pull requests.
