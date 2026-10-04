# NN: Slice title

Status: Aligning
Depends on: none
Roadmap milestone: M

<!--
Copy this file to docs/slices/NN-short-name.md.
Delete any section that does not apply instead of leaving it empty.

Each slice lives on its own branch, slice/NN-short-name, created from main when the doc is started.

Status lifecycle:
- Aligning: the scope and design are being worked out.
- Ready: the design is reviewed and there are no open questions, so coding may start. The doc at Ready is the first commit on the branch.
- In progress: code is being written. The Scope, Done when and Design sections are now frozen, and any change is recorded under Changes. Once the Done when items pass locally, push the branch and open a pull request.
- Done: every Done when item passes, CI on the pull request included, and Changes is filled in. That update is the last commit on the branch, and the pull request is then merged with a merge commit.
- Dropped: abandoned, with the reason in Changes.
-->

## Scope

Delivers:

- What this slice provides, end to end.

Does not deliver (and where it goes):

- What is left out, with the slice or milestone that takes it, for example "graceful shutdown: slice 07".

## Done when

Each item names the invariant or guarantee it proves and the test that proves it.

- [ ] Observable behaviour (invariant N, `TestName`)
- [ ] 3 to 6 items in total, each one runnable

## Design

| Function | File | New or modified | Author | What | Why |
| -------- | ---- | --------------- | ------ | ---- | --- |

Author is who writes the code: `user`, `assist`, or `both`.

Invariants touched:

- Which of the README invariants this slice relies on or could break.

Crash points:

- Where a `SIGKILL` can land in the new code, and what recovery does in each case.

Flows:

Use a Mermaid `sequenceDiagram` for interactions between a worker and the database, and a `stateDiagram-v2` for status changes.

Alternatives considered:

- Option, and why it was not chosen. Link an ADR if one exists.

Open questions:

- Anything unresolved. The slice cannot be Ready while this list is non-empty.

## Changes

Filled in at the end.

Pull request: #N

Commits:

- `abc1234` message

Planned vs actual:

Deviations from the design, and why:

README or invariant updates this caused:

What surprised me (optional, only when there is something worth recording):
