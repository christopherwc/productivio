# Pomodoro (Go)

A focus timer with tasks, habits and project tracking. Ported from the
Python/Tkinter original.

[![CI](https://github.com/christopherwc/productivio/actions/workflows/ci.yml/badge.svg)](https://github.com/christopherwc/productivio/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/christopherwc/productivio.svg)](https://pkg.go.dev/github.com/christopherwc/productivio)
[![Go Report Card](https://goreportcard.com/badge/github.com/christopherwc/productivio)](https://goreportcard.com/report/github.com/christopherwc/productivio)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

Single static binary. No runtime dependencies, no cgo, nothing to
install alongside it.

---

## Read this first: the interface changed

The Python version was a Tkinter desktop application with four tabs.
**This port has a command-line interface instead**, and that is not a
stylistic preference.

Go has no GUI toolkit in its standard library. Every option
— [Fyne](https://fyne.io), [Gio](https://gioui.org),
[Wails](https://wails.io) — is a third-party dependency, and the
desktop ones need cgo and an OpenGL or WebKit toolchain. Adopting one
would trade away the properties that make this port worth having: a
2 MB static binary, no dependency tree to audit, and cross-compilation
to Apple Silicon from any machine with one environment variable.

So the model layer was ported faithfully and the interface was not.
What you get instead:

| | Python original | This port |
| --- | --- | --- |
| Interface | Tkinter GUI, four tabs | Command line |
| Dependencies | Standard library only | Standard library only |
| Distribution | Source + a Python with Tk 8.6 | One static binary |
| Apple Silicon | Needs an arm64-native Python | `GOOS=darwin GOARCH=arm64 go build` |
| Data files | Same four JSON files | **Byte-compatible — see below** |

That's still true of `cmd/pomodoro`, the binary this README is mostly
about — but the GUI does now exist, as a second, separate binary with
its own module (Fyne, cgo, a newer Go). See [How the GUI
works](#how-the-gui-works) for why it's kept apart, and how to run it.

## Your existing data still works

The JSON files are byte-compatible with the Python version. Point this
binary at an existing installation and your sessions, tasks, habits and
projects load unchanged; a pre-existing `~/.pomodoro_sessions.json`
from the earliest versions is imported automatically on first run.

That compatibility constrains the time encoding, which is why
`core.Timestamp` and `core.Date` exist rather than plain `time.Time`:
Go marshals `time.Time` as RFC 3339 with a zone offset, and Python
wrote naive local timestamps. Silently changing the format would have
stranded every existing file. There is a test that pins the wire format
for exactly this reason.

## Install

```bash
go install github.com/christopherwc/productivio/cmd/pomodoro@latest
```

Or build from a checkout:

```bash
git clone https://github.com/christopherwc/productivio.git
cd productivio
make build       # ./bin/pomodoro
make install     # into $GOBIN
```

Cross-compiling needs no toolchain setup, because there is no cgo:

```bash
make release     # binaries for darwin/arm64, darwin/amd64, linux/amd64, linux/arm64
```

## Use

```bash
pomodoro status                          # today's summary
pomodoro project add "Ship v1" 2026-09-30
pomodoro project list                    # copy the id
pomodoro project add "Checkout redesign" - <project-id>   # a subproject
pomodoro project parent <id> <parent-id> # file an existing project under another
pomodoro project hold <id>               # pause it; reopen brings it back
pomodoro project rm <id>                 # delete it (tasks unfiled, subprojects promoted)
pomodoro task add "Write docs" 4 <project-id>
pomodoro task add "File taxes" 1 - 2026-10-15   # due date, no project
pomodoro task list                       # DUE column, flagged once overdue
pomodoro start -task <task-id>           # 25/5 by default
pomodoro habit add "Review inbox" weekdays
pomodoro habit check <habit-id>
pomodoro history 20
pomodoro report                          # focus time by project, last 7 days
pomodoro report 30                       # widen the window
pomodoro where                           # where your data lives
```

`start` credits the completed interval to the task, to the task's
project, and to the permanent session history in one step. That is the
integration the whole design is built around: the timer knows what it
is timing, so focus time becomes project-level data instead of generic
minutes.

A project can be filed under another to build a tree of subprojects —
`project list` prints it indented, and a subproject's task and effort
progress rolls up into every one of its ancestors, at any depth. The
tree rejects cycles: a project can never become its own ancestor.
Deleting a project promotes its direct subprojects up one level rather
than orphaning them.

A task can carry its own due date, independent of its project's —
useful since a project's deadline says when the whole thing is due, not
when each piece of it needs to happen. `status` and `task list` flag an
open task whose date has passed; a done task is never flagged, however
late it finished.

`report` breaks focused time down by project over a trailing window
(7 days by default), sorted by minutes so the projects worked on most
show first — the same session records `history` lists individually,
rolled up instead of itemized. A session's project is stored on the
session itself, so a report still totals correctly under a project's
name even after it has since been renamed or deleted.

## How progress is measured

Ported unchanged from the Python version, including the reasoning.

**Task progress** — completed tasks over total. Adding tasks lowers it
even though nothing was undone; the scope grew, and the number saying
so is correct. For a project with subprojects, this counts every
subproject's tasks too, at any depth: filing work under a subproject
does not hide it from the parent's rollup.

**Effort progress** — completed pomodoros over estimated. Deliberately
not capped at 100%. Tasks half done with 90% of the estimate already
spent means the remaining half will hurt, and hiding that throws the
signal away. Also rolled up across subprojects, same as task progress.

**Health** — On track, At risk, Overdue, Not started, On hold,
Completed — from deterministic rules, not heuristics, so it is
reproducible and testable. "At risk" means task progress has fallen
more than 25% behind the elapsed share of the project's timeline.

**Habit streaks** follow two rules that keep the tracker honest:
unscheduled days are skipped rather than counted as misses, so missing
a Saturday cannot break a weekdays-only streak; and today gets grace,
so a daily habit not yet done at 9am still shows yesterday's streak.

## Where your data lives

| Platform | Directory |
| --- | --- |
| Linux | `$XDG_DATA_HOME/pomodoro`, or `~/.local/share/pomodoro` |
| macOS | `~/Library/Application Support/Pomodoro` |

Writes are atomic — temp file, fsync, `os.Rename` — so a crash
mid-save leaves the previous good file intact rather than an
unparseable half-written one.

## Layout

```
cmd/pomodoro/          main; wires the real environment to the CLI
internal/core/         model and persistence; no I/O beyond files
internal/platform/     every Linux/macOS difference, in one place
internal/cli/          command dispatch and output
gui/                    the desktop GUI, a separate Go module — see
                        "How the GUI works" below
```

Three decisions worth knowing before changing anything:

- **`Store` is an interface where it is consumed** (`internal/cli`), not
  where it is defined. That is the usual Go arrangement, and it is what
  lets tests substitute a store whose reads succeed and whose writes
  fail — a combination no arrangement of real files produces, least of
  all in CI running as root where permission bits are ignored.
- **Persistence takes a directory rather than reading a global.** The
  Python suite had to monkey-patch a module function to isolate tests;
  here a test constructs a `Store` over `t.TempDir()`.
- **Platform differences live only in `internal/platform`.** Two
  package variables shadow `runtime.GOOS`/`GOARCH` so a Linux CI run
  exercises the macOS branches and vice versa. A platform branch
  anywhere else becomes a permanent dead spot in the coverage report.

## Testing

```bash
make test        # go test ./...
make cover       # coverage report
make check       # vet, race, lint, coverage — everything CI runs
```

**99.9% statement coverage.** One statement is not covered: the
`Close()` error branch in the atomic write, which can only be reached
after both the write and the fsync have already succeeded — an ordering
no injectable `os` call produces. It is commented as such in the source
rather than deleted, because dropping the check would discard a real if
rare error.

One honest caveat on that number. The Python original reported **100%
branch coverage**; `go test -cover` measures **statements only**. The
Go toolchain has no branch-coverage mode, so 99.9% here is a weaker
guarantee than 100% there, not a stronger one. The suite compensates
where it matters by table-driving both sides of the platform and
schedule branches explicitly.

CI runs the matrix on Ubuntu and macOS across Go 1.21–1.23, with
`go vet`, the race detector, `staticcheck` and `gofmt` enforcement.

### Actions are pinned to a commit, not a tag

Every third-party step in `.github/workflows/ci.yml` is pinned to a
full commit SHA (`actions/checkout@3d3c42e...  # v7`), not a floating
tag like `@v4`. Tags are mutable — whoever controls the upstream repo
can repoint one at a different commit at any time — so a tag-pinned
workflow trusts every future push to that tag, not just the code that
was reviewed when the pin was added. A compromised action is a
supply-chain attack with direct access to `GITHUB_TOKEN` and anything
the runner can reach. Pinning to a SHA makes the dependency immutable:
the workflow keeps running exactly the code that was audited, and
bumping it is a visible, reviewable diff instead of a silent change.
The version is kept as a trailing comment purely for humans; Dependabot
(configured in `.github/dependabot.yml`) reads the SHA, not the
comment, and opens a PR that updates both when a new release ships.

The same instinct applies to `gui/`'s one exception to the
zero-dependency policy above: Fyne is pinned to an exact version in
`gui/go.mod`, `gui/go.sum` is committed and checked with `go mod
verify`, a pinned-version `govulncheck` scan runs in the `GUI` CI job
(not `@latest` — a floating version already broke this CI once, see
the git history), and a second Dependabot entry scoped to `/gui`
tracks Fyne's dependency tree independently of the Actions one above.

## How the GUI works

`gui/` is a native desktop GUI (Fyne) with five tabs — Dashboard,
Tasks, Projects, Habits, Timer — mirroring what the CLI already does,
plus a File menu with Quit. Run it with:

```bash
cd gui
go run ./cmd/pomodoro-gui
```

It reads and writes the exact same data files as the CLI (same
`platform.DataDir()`, same `core.Store`), so the two are interchangeable
day to day — add a task from the CLI, see it in the GUI's Tasks tab, or
the reverse.

### Why a second module

Fyne needs cgo and Go ≥ 1.22, both of which would otherwise force this
module's `go.mod` (`go 1.21`, zero dependencies, no cgo) to change too,
breaking the CLI's single-static-binary property and its CI matrix's
Go 1.21 leg. `gui/go.mod` is a second, separate Go module instead —
`github.com/christopherwc/productivio/gui`, `go 1.25` — with its own
`replace github.com/christopherwc/productivio => ../` pointing back at
this one. That works — `gui/internal/ui` can still import
`internal/core` and `internal/platform` from a different module —
because Go's `internal/` visibility rule is import-path-based, not
module-based: `.../productivio/gui/...` is still inside the
`.../productivio/` tree.

One consequence worth knowing before changing anything: `internal/core`
and `internal/platform` are reusable exactly as they stand — nothing
in `gui/` ever reimplements task/habit/project logic, it only calls
into `core`, matching the pattern `internal/cli` already set. Neither
of those two packages was replaced or forked for the GUI; a third
interface (a web UI, a TUI) could be added the same way, as a third
module, without touching either.

### Concurrency

`*core.Store` and the collection types have no mutex. The rule `gui/`
follows: every Store or UI touch happens on Fyne's UI goroutine. The
timer is the one place this actually matters — `app.Timer` is a plain,
synchronous, goroutine-free state machine, and `internal/ui`'s
scheduling loop re-arms `env.AfterFunc` every second, but its raw
callback does nothing but hop onto the UI goroutine via `fyne.Do`
before touching `Timer` or `Store` at all.

### Building a release binary

```bash
cd gui
make release     # dist/pomodoro-gui-<os>-<arch>, for whatever this host can build
```

Unlike the CLI's `make release`, this can't cross-compile every target
from one machine: Fyne's cgo needs a real C toolchain for the target
platform, not just a `GOOS`/`GOARCH` env var. Each host builds only
what its own toolchain reaches — `linux/amd64` on Linux; `darwin/arm64`
and `darwin/amd64` on Apple Silicon macOS, via clang's built-in
multi-arch support. `linux/arm64` is a known gap, not a silent one: it
needs either a native arm64 runner or a cross-gcc and sysroot, neither
set up yet. CI's `GUI Release` job runs this on both `ubuntu-latest`
and `macos-latest` and uploads whatever each produces as build
artifacts.

### Known limitation

Fyne builds every tab's content once, up front, not lazily per
tab-switch. Add a project in the Projects tab and the Tasks tab's
project dropdown won't see it until the app restarts — each tab only
refreshes itself, after its own actions. Not fixed yet; documented
here rather than silently worked around.

### Testing

`fyne.io/fyne/v2/test`'s headless driver needs no real display, so
`go test ./...` in `gui/` runs the same everywhere CI does. `gui/` has
no hard statement-coverage floor the way `internal/core` does (99.9%)
— widget-wiring code doesn't reach that bar as naturally as domain
logic, and the domain logic itself stays covered by `internal/core`'s
own suite, which `gui/` calls into rather than reimplementing. Each
row-formatting function (`taskRowText`, `projectRowText`,
`habitRowText`, `healthColor`) is still pulled out and unit-tested on
its own, separate from the Fyne wiring around it.

Headless tests only prove the widget tree builds; they don't prove it
renders sanely or that a countdown actually ticks in real time.
Building the real binary and running it under a virtual display
(Xvfb), screenshotting it, and driving it with `xdotool` caught two
bugs no headless test did: an add-form's fields collapsing to a few
unreadable characters (`widget.Entry` doesn't size itself to
placeholder text), and a duplicate "Quit" menu entry (Fyne silently
auto-appends its own unless an item is labeled exactly "Quit"). Worth
reaching for again for anything layout- or interaction-shaped.

## License

MIT — see [LICENSE](LICENSE).

Pomodoro Technique® is a registered trademark of Francesco Cirillo.
This project is not affiliated with or endorsed by him.
