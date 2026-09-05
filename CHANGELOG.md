# Changelog

All notable changes to this project are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **`gui/`** — scaffold for a native desktop GUI (Fyne), as a second,
  separate Go module so the CLI's zero-dependency/no-cgo/single-binary
  properties are unaffected. CI, `go.mod` tidiness, `go mod verify` and
  a pinned-version `govulncheck` scan all run in a new `GUI` job; a new
  Dependabot `gomod` entry tracks Fyne's dependency tree. No user-facing
  functionality yet — this is the window/build scaffold only.
- **`gui/internal/app`** — the Store/Env abstraction the GUI's views are
  built against, mirroring `internal/cli`'s pattern so views can be
  tested without touching real files or a real clock.
- **GUI dashboard view.** The first real tab: today's focus time, task
  and habit progress, and portfolio-level project stats — the same
  rollup as `pomodoro status`. The window is now a tab container that
  later views add tabs to, rather than a single placeholder label.
- **GUI tasks view.** Add, complete, reorder (up/down) and delete
  tasks, each optionally filed under a project.
- **GUI projects view.** List, add and complete projects, with health
  (On track/At risk/Overdue/...) shown as a colored label — the same
  surface `pomodoro project` exposes; the CLI has no project
  delete/hold/reopen commands, so neither does this view.
- **GUI habits view.** Add, check off today, reorder (up/down) and
  delete habits, each row showing the same 7-day grid, streak and
  30-day completion rate as `pomodoro habit list`. This is the last of
  the four views; only the pomodoro timer itself remains.
- **GUI pomodoro timer.** Choose work/rest lengths and an optional
  open task, then start/pause/resume a real countdown: work→rest→idle
  transitions, a desktop notification at each transition, and session
  persistence with task credit on work completion — the same behavior
  `pomodoro start` has. `app.Timer` is a plain, synchronous,
  goroutine-free state machine; `internal/ui`'s scheduling loop is the
  only thing that touches a background goroutine (the raw
  `env.AfterFunc` tick), and its callback does nothing but hop onto
  the UI goroutine via `fyne.Do` before calling `Timer.Tick()` — so
  `Timer` itself needed no mutex and stayed trivially unit-testable
  with a fake clock. Verified against the real binary under Xvfb: a
  full real-time work→rest cycle, credited to a task selected through
  the dropdown, correctly persisted and visible via the CLI afterward.
- **GUI menu bar.** A File menu with Quit, bound to Cmd+Q on macOS and
  Ctrl+Q elsewhere via Fyne's own cross-platform shortcut modifier and
  rendered automatically next to the label through the menu item's
  `Shortcut` field. The item's label must be exactly "Quit" — Fyne
  auto-appends its own fallback Quit item to the menu unless it finds
  one already labeled exactly that, which produced two Quit entries
  in the running app when this instead read "Quit (Ctrl+Q)", caught
  by screenshot before it ever merged. Verified against the real
  binary under Xvfb: exactly one Quit item, and both the menu click
  and the raw keyboard shortcut quit the app.
- **GUI release packaging.** `gui/Makefile`'s `release` target and a
  new `GUI Release` CI job build `pomodoro-gui` binaries the way the
  CLI's `Cross-compile` job builds `pomodoro`'s — except this one
  can't cross-compile from a single machine: Fyne's cgo needs a C
  toolchain for the actual target, not just a `GOOS`/`GOARCH` env var.
  Each OS's own runner builds what its own toolchain can reach —
  linux/amd64 on `ubuntu-latest`, darwin/arm64 and darwin/amd64 on
  `macos-latest` via clang's built-in multi-arch support — and
  linux/arm64 stays an explicitly documented gap rather than a silent
  one, pending either a native arm64 runner or a cross-gcc + sysroot.
- **Subprojects.** A project can now be filed under another via
  `pomodoro project parent <id> <parent|->` or, in the GUI, a parent
  picker on the add-project form. `TaskProgress` and `EffortProgress`
  roll a subproject's own progress up into every one of its ancestors,
  at any depth, so a parent's numbers reflect its whole tree, not just
  its direct tasks. `project list` (and the GUI projects view) render
  the tree indented rather than as a flat list. Reassigning a parent
  rejects a cycle (a project becoming its own ancestor); deleting a
  project promotes its direct subprojects up one level instead of
  orphaning them; and a dangling or cyclic `parent_id` read back from a
  hand-edited data file is repaired on load, the same guarantee
  `normalize` already gave every other field.
- **GUI blue/grey theme.** Replaces Fyne's default palette with a
  custom blue-accent, slate-grey theme (light and dark variants),
  verified against the real binary under Xvfb.
- **Task due dates.** A task can carry its own deadline, independent of
  its project's — `pomodoro task add <title> [n] [proj|-] [due]`, or a
  due-date field on the GUI add-task form. `task list`, the GUI tasks
  view, and both `status` outputs flag an open task whose date has
  passed; a done task is never flagged, however late it finished.
  Mirrors `Project.IsOverdue`/`DaysUntilDue`, so the same "overdue"
  concept now applies at both the project and the task level.
- **Focus-time report.** `pomodoro report [n]` (default 7 days) and a
  new GUI Report tab break focused time down by project over a
  trailing window, sorted by minutes so the most-worked-on projects
  show first. Built on a new `Sessions.Report`, grouped by each
  session's own recorded project name rather than a live `Projects`
  lookup, so a report still totals correctly under a project's old
  name even after it has since been renamed or deleted — the same
  reason `Session` already stored that name instead of only the id.
- **Project hold/reopen/rm.** `Project.Hold`/`Reopen` and
  `Projects.Delete` existed and were fully tested from the start, but
  had no command or GUI control — `project list`'s doc comment even
  noted the gap. `pomodoro project hold <id>`, `project reopen <id>`
  and `project rm <id>` (plus matching Hold/Reopen/✕ buttons on the
  GUI projects view) now expose them: pausing/reactivating a project,
  and deleting one without losing its tasks (unfiled, not deleted) or
  orphaning its subprojects (promoted up one level, as `Delete` already
  guaranteed).

### Fixed

- **GUI layout overflow.** Built and ran the actual `pomodoro-gui`
  binary for the first time this session (previously verified only via
  headless widget tests and CI's build-only job) — under Xvfb, with
  screenshots. Found two real problems no unit test could catch:
  `widget.Entry` doesn't size itself to its placeholder text, so every
  add-row's fields were unusably narrow inside a plain `HBox`; and the
  480×320 default window size clipped list rows' delete/reorder
  buttons off the right edge entirely. Fixed by giving each add-row's
  primary field the `Border` layout's stretching center slot instead
  of sharing an `HBox`, wrapping secondary fields at a fixed width via
  `container.NewGridWrap`, and resizing the default window to 820×560.
  Re-verified visually after the fix: all three add-forms are readable
  and every row's buttons stay on screen.
- **Stale post-rename docs.** README and CONTRIBUTING both still said
  `cd pomodoro` after cloning, left over from before the repo was
  renamed to `productivio`. CONTRIBUTING also undercounted the
  required CI checks (nine, not eleven, since the `GUI` jobs were
  added). README's Layout section and supply-chain-security section
  now mention `gui/`.

### Documentation

- **README's "Building a graphical version" rewritten as "How the GUI
  works."** All four views, the timer and the menu bar are done, so
  the section now describes the module split (and why), the
  concurrency rule, a known cross-tab staleness limitation, and what
  building the real binary under Xvfb caught that headless tests
  didn't — rather than a hypothetical options list. CONTRIBUTING gains
  matching rules for the coverage exception, the concurrency rule, and
  reaching for a real-binary check on layout/interaction changes.
- **Post-release-packaging doc pass.** `app.TimerPhase`'s `PhaseIdle`/
  `PhaseWork`/`PhaseRest` constants had no comment directly above their
  `const (` block — every other exported const group in the repo has
  one, caught by re-running the same doc-comment audit this session
  used earlier. CONTRIBUTING's required-check count was still "eleven"
  after the two `GUI Release` jobs joined the branch-protection ruleset
  in the release-packaging PR, making it thirteen. README's GUI section
  had no mention of `make release` at all.

## [1.0.0] - 2026-09-03

Port of the Python/Tkinter application to Go.

### Added

- **`internal/core`** — the full data model: sessions, tasks, habits,
  projects and their rollups, with atomic JSON persistence. No I/O
  beyond files and no interface code, so it is reusable under any front
  end.
- **`internal/platform`** — data directories, desktop notifications and
  shortcut labels for Linux and macOS, with `GOOS`/`GOARCH` shadowed by
  package variables so either platform's branches can be exercised from
  the other.
- **`internal/cli`** — command dispatch for `status`, `start`, `task`,
  `project`, `habit`, `history`, `where` and `version`.
- **Byte-compatible data files.** An existing Python installation's
  `sessions.json`, `tasks.json`, `habits.json` and `projects.json` load
  unchanged, and a legacy `~/.pomodoro_sessions.json` is imported once
  on first run.
- **Static binaries for darwin/arm64, darwin/amd64, linux/amd64 and
  linux/arm64**, cross-compiled from any host because there is no cgo.

### Changed

- **The interface is a command line, not a GUI.** Go has no standard
  library GUI toolkit, and every third-party option requires cgo and a
  graphics toolchain, which would forfeit the static binary and simple
  cross-compilation. The model layer was ported faithfully; the
  interface layer was not. See the README for what a graphical port
  would involve.
- **Errors are returned, not signalled by `nil`.** Lookups that
  returned `None` in Python return `(*T, error)` here, so a caller that
  ignores a failure is visible to `go vet` and to review.
- **Persistence takes a directory.** `core.NewStore(dir)` replaces the
  Python module-level functions that read a global, removing the need
  for tests to monkey-patch anything.
- **`Store` is an interface in `internal/cli`**, declared where it is
  consumed, so tests can substitute a store whose reads succeed and
  whose writes fail.
- **`Project.Complete`, `.Hold` and `.Reopen`** replace `SetStatus` at
  call sites passing a known-good constant, removing an error branch
  that could never be taken and therefore never be tested.

### Removed

- Theme selection, font loading, Tk version checks, HiDPI scaling and
  the macOS Cmd-Q handler. All were Tkinter concerns with no equivalent
  here; they were dropped rather than reimplemented as no-ops.

### Notes

- Statement coverage is 99.9%. One statement — the `Close()` error
  branch in the atomic write — is unreachable and commented as such.
  Note that `go test -cover` measures statements, not branches, so this
  is a weaker guarantee than the Python version's 100% branch coverage,
  not a stronger one.
- `PercentLabel` rounds half away from zero, where Python used banker's
  rounding, so 12.5% renders as 13% here and 12% there. Cosmetic, but
  asserted in the tests so it is not "corrected" by accident.
