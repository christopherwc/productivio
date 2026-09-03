# Contributing

## Setup

```bash
git clone https://github.com/christopherwc/productivio.git
cd pomodoro
make check     # fmt, vet, race, coverage, lint
```

No dependencies to install: the module requires nothing beyond the
standard library, and `go.mod` should stay that way. A pull request
that adds a dependency needs to argue for it in the description.

`main` is protected: nobody, including the repo owner, can push to it
directly. Every change lands through a pull request, and GitHub blocks
merging until all nine CI jobs — Lint, the six Test combinations,
Coverage and Cross-compile — report success on the PR's latest commit.
Push a branch, open the PR, and wait for it to go green before merging.

## The rules that matter here

**Platform differences go in `internal/platform`, nowhere else.** The
package shadows `runtime.GOOS` and `GOARCH` in variables so tests can
exercise either platform from the other. A `runtime.GOOS` check
anywhere else becomes a permanent dead spot in the coverage report.

**Accept interfaces where you consume them.** `internal/cli` declares
its own `Store` interface rather than importing `*core.Store`. That is
what allows a test double whose reads succeed and whose writes fail —
a combination no arrangement of real files produces, least of all in CI
running as root where permission bits are ignored.

**Take the clock as a parameter.** Every function that needs "today"
receives it. `core.Today()` exists for the entry point to call once.
Tests pin the date so they cannot behave differently on a Tuesday.

**Prefer methods that cannot fail over unreachable error branches.**
`Project.Complete()` exists because `SetStatus(StatusCompleted)` forced
callers to write an error check that could never trigger and could
never be covered.

**Don't break the wire format.** The JSON is byte-compatible with the
Python implementation, which is what lets someone upgrade without
losing history. `TestPythonWireFormat` pins it. If you genuinely need
to change it, write a migration.

**Table-driven tests, `t.TempDir()` for anything on disk.** No test may
touch a real data directory.

## Before opening a pull request

1. `make check` passes.
2. Coverage has not dropped. One statement is documented as
   unreachable; there should not be a second without an argument.
3. `CHANGELOG.md` has a line under **Unreleased**.
4. Say which platform you tested on.

## Commit messages

Conventional Commits, matching the changelog's organisation:

```
feat(core): add per-project effort estimates
fix(habits): count backfilled completions before the created date
test(cli): cover the failed task-credit path
docs(readme): explain why the GUI did not port
```
