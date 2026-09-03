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

If you want the GUI back, [Building a graphical
version](#building-a-graphical-version) sets out what that would take.

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
pomodoro task add "Write docs" 4 <project-id>
pomodoro task list
pomodoro start -task <task-id>           # 25/5 by default
pomodoro habit add "Review inbox" weekdays
pomodoro habit check <habit-id>
pomodoro history 20
pomodoro where                           # where your data lives
```

`start` credits the completed interval to the task, to the task's
project, and to the permanent session history in one step. That is the
integration the whole design is built around: the timer knows what it
is timing, so focus time becomes project-level data instead of generic
minutes.

## How progress is measured

Ported unchanged from the Python version, including the reasoning.

**Task progress** — completed tasks over total. Adding tasks lowers it
even though nothing was undone; the scope grew, and the number saying
so is correct.

**Effort progress** — completed pomodoros over estimated. Deliberately
not capped at 100%. Tasks half done with 90% of the estimate already
spent means the remaining half will hurt, and hiding that throws the
signal away.

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
gui/                    the desktop GUI — a separate Go module; see
                        "Building a graphical version" below
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

## Building a graphical version

**Work has started on a native GUI** in `gui/` (Fyne), landing as a
sequence of small PRs. It is a second, separate Go module with its own
`go.mod`/CI job, specifically so the CLI above keeps its zero-dependency,
no-cgo, single-static-binary properties untouched — see `gui/go.mod`'s
header comment for why. This section will be rewritten to describe the
finished GUI once that work lands; until then it still describes the
options as they stood before.

If the GUI matters more than the single-binary property, the realistic
options, in rough order of effort:

- **[Fyne](https://fyne.io)** — closest to the Tkinter original in
  spirit. Widgets, layouts, native-ish look. Needs cgo and OpenGL
  headers; cross-compiling becomes a real chore.
- **[Gio](https://gioui.org)** — immediate-mode, excellent performance,
  a much larger rewrite of the interface layer.
- **[Wails](https://wails.io)** — HTML/CSS front end over this Go
  backend. Uses the system WebView, so no bundled Chromium, but you are
  writing a web front end.
- **A terminal UI** — [Bubble Tea](https://github.com/charmbracelet/bubbletea)
  or [tview](https://github.com/rivo/tview). Keeps the static binary,
  gets back the interactive four-pane layout.

In every case `internal/core` and `internal/platform` are reusable as
they stand; only `internal/cli` would be replaced. That separation was
the point of the split.

## License

MIT — see [LICENSE](LICENSE).

Pomodoro Technique® is a registered trademark of Francesco Cirillo.
This project is not affiliated with or endorsed by him.
