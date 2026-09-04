// Package cli implements the command-line interface.
//
// Everything is written against an explicit environment — an output
// writer, a store, a clock — rather than reaching for globals. That is
// what lets the whole surface be tested in-process, without spawning a
// binary or touching the real user's data.
package cli

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/christopherwc/productivio/internal/core"
	"github.com/christopherwc/productivio/internal/platform"
)

// Store is the persistence surface the commands use.
//
// Declared as an interface here, where it is consumed, rather than
// taking the concrete *core.Store. That is the usual Go arrangement,
// and it is what lets a test substitute a store whose reads succeed and
// whose writes fail — a combination no arrangement of real files can
// produce, especially when the suite runs as root and permission bits
// are ignored.
type Store interface {
	Dir() string
	LoadSessions() core.Sessions
	SaveSessions(core.Sessions) error
	LoadTasks() core.Tasks
	SaveTasks(core.Tasks) error
	LoadHabits() core.Habits
	SaveHabits(core.Habits) error
	LoadProjects() core.Projects
	SaveProjects(core.Projects) error
}

// Env is everything a command needs from the outside world.
//
// Collecting these behind one struct means a test constructs an Env
// over a temp directory and a pinned clock, and every command is then
// deterministic.
type Env struct {
	Store Store
	Out   io.Writer
	Err   io.Writer

	// Now returns the current instant. Pinned in tests.
	Now func() time.Time

	// Sleep waits during a countdown. Tests substitute a no-op so the
	// suite does not actually sit for twenty-five minutes.
	Sleep func(time.Duration)

	// Notify raises a desktop notification.
	Notify func(title, message string) bool
}

// Today is the current calendar day according to the environment.
func (e *Env) Today() core.Date { return core.DateOf(e.Now()) }

// Exit codes. Two is used for usage errors, following the convention
// most Unix tools observe.
const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

const usage = `pomodoro — a focus timer with tasks, habits and projects

Usage:
  pomodoro status                     Today's summary
  pomodoro start [flags]              Run a work interval
  pomodoro task list                  List tasks
  pomodoro task add <title> [n] [proj]  Add a task estimated at n pomodoros
  pomodoro task done <id>             Toggle a task complete
  pomodoro task rm <id>               Delete a task
  pomodoro project list               List projects, subprojects indented
  pomodoro project add <name> [due|-] [parent]  Add a project, optionally under parent
  pomodoro project parent <id> <parent|->  File under parent, or - to clear
  pomodoro project done <id>          Mark a project complete
  pomodoro habit list                 List habits with streaks
  pomodoro habit add <name> [sched]   Add a habit (daily|weekdays|weekends)
  pomodoro habit check <id>           Toggle today's completion
  pomodoro history [n]                Show the last n sessions (default 10)
  pomodoro where                      Print the data directory
  pomodoro version                    Print version information

Flags for start:
  -work N     Work interval in minutes (default 25)
  -rest N     Rest interval in minutes (default 5)
  -task ID    Credit the interval to a task, and to its project
`

// Version is set at build time with -ldflags "-X ...Version=v1.0.0".
var Version = "dev"

// Run parses arguments and executes a command against the real
// environment, returning a process exit code.
func Run(args []string, out, errOut io.Writer) int {
	dir, err := platform.DataDir()
	if err != nil {
		fmt.Fprintln(errOut, "pomodoro:", err)
		return exitError
	}
	store, err := core.NewStore(dir)
	if err != nil {
		fmt.Fprintln(errOut, "pomodoro:", err)
		return exitError
	}
	// Import any pre-existing history from the Python versions once.
	if home, err := platform.HomeDir(); err == nil {
		if migrated, err := store.MigrateLegacy(home); err == nil && migrated {
			fmt.Fprintln(out, "Imported your existing session history.")
		}
	}

	env := &Env{
		Store:  store,
		Out:    out,
		Err:    errOut,
		Now:    time.Now,
		Sleep:  time.Sleep,
		Notify: platform.Notify,
	}
	return Dispatch(env, args)
}

// Dispatch routes a parsed command to its implementation.
//
// Separated from Run so tests can drive every command against a
// temporary environment without touching the real data directory.
func Dispatch(env *Env, args []string) int {
	if len(args) == 0 {
		fmt.Fprint(env.Out, usage)
		return exitUsage
	}

	command, rest := args[0], args[1:]
	var err error
	switch command {
	case "status":
		err = cmdStatus(env)
	case "start":
		err = cmdStart(env, rest)
	case "task":
		err = cmdTask(env, rest)
	case "project":
		err = cmdProject(env, rest)
	case "habit":
		err = cmdHabit(env, rest)
	case "history":
		err = cmdHistory(env, rest)
	case "where":
		fmt.Fprintln(env.Out, env.Store.Dir())
	case "version":
		fmt.Fprintf(env.Out, "pomodoro %s\n%s\n", Version, platform.Summary())
	case "help", "-h", "--help":
		fmt.Fprint(env.Out, usage)
	default:
		fmt.Fprintf(env.Err, "pomodoro: unknown command %q\n\n", command)
		fmt.Fprint(env.Err, usage)
		return exitUsage
	}

	if err != nil {
		fmt.Fprintln(env.Err, "pomodoro:", err)
		if errors.Is(err, errUsage) {
			return exitUsage
		}
		return exitError
	}
	return exitOK
}

// errUsage marks an error caused by bad arguments rather than by a
// failure, so Dispatch can return the conventional exit code 2.
var errUsage = errors.New("usage")

func usageErrorf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", errUsage, fmt.Sprintf(format, args...))
}

// --- status ----------------------------------------------------------

func cmdStatus(env *Env) error {
	today := env.Today()
	sessions := env.Store.LoadSessions()
	tasks := env.Store.LoadTasks()
	habits := env.Store.LoadHabits()
	projects := env.Store.LoadProjects()

	count, minutes := sessions.DailySummary(today)
	fmt.Fprintf(env.Out, "Today: %d pomodoros (%s)   All time: %d\n\n",
		count, core.FormatMinutes(minutes), len(sessions))

	open, done, remaining := tasks.TaskStats()
	fmt.Fprintf(env.Out, "Tasks:    %d open, %d done, %d pomodoros remaining\n",
		open, done, remaining)

	scheduled, completed := habits.Stats(today)
	if scheduled == 0 {
		fmt.Fprintln(env.Out, "Habits:   none scheduled today")
	} else {
		fmt.Fprintf(env.Out, "Habits:   %d/%d kept today\n", completed, scheduled)
		for _, h := range habits.Due(today) {
			fmt.Fprintf(env.Out, "            due: %s (streak %d)\n",
				h.Name, h.CurrentStreak(today))
		}
	}

	active, projectsDone, needAttention := projects.PortfolioStats(tasks, today)
	fmt.Fprintf(env.Out, "Projects: %d active, %d completed", active, projectsDone)
	if needAttention > 0 {
		fmt.Fprintf(env.Out, ", %d needing attention", needAttention)
	}
	fmt.Fprintln(env.Out)
	return nil
}

// --- start -----------------------------------------------------------

func cmdStart(env *Env, args []string) error {
	work, rest, taskID := 25, 5, ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-work", "--work":
			value, err := intArg(args, i)
			if err != nil {
				return err
			}
			work, i = value, i+1
		case "-rest", "--rest":
			value, err := intArg(args, i)
			if err != nil {
				return err
			}
			rest, i = value, i+1
		case "-task", "--task":
			if i+1 >= len(args) {
				return usageErrorf("-task needs a task id")
			}
			taskID, i = args[i+1], i+1
		default:
			return usageErrorf("unknown flag %q for start", args[i])
		}
	}
	if work < 1 || rest < 1 {
		return usageErrorf("intervals must be at least one minute")
	}

	tasks := env.Store.LoadTasks()
	projects := env.Store.LoadProjects()

	var task *core.Task
	var project *core.Project
	if taskID != "" {
		found, err := tasks.Find(taskID)
		if err != nil {
			return err
		}
		task = found
		// Focus follows the work: crediting a task also credits its
		// project, without a second argument.
		if task.ProjectID != "" {
			if p, err := projects.Find(task.ProjectID); err == nil {
				project = p
			}
		}
	}

	label := "unattributed work"
	if task != nil {
		label = task.Title
		if project != nil {
			label += " · " + project.Name
		}
	}
	fmt.Fprintf(env.Out, "Working on: %s\n", label)

	start := env.Now()
	countdown(env, "Work", work)

	end := env.Now()
	sessions := append(env.Store.LoadSessions(),
		core.NewSession(start, end, work, task, project))
	if err := env.Store.SaveSessions(sessions); err != nil {
		return err
	}
	if task != nil {
		task.Completed++
		if err := env.Store.SaveTasks(tasks); err != nil {
			return err
		}
	}

	env.Notify("Pomodoro complete", "Time for a break.")
	fmt.Fprintf(env.Out, "\nLogged %s.\n", core.FormatMinutes(work))

	countdown(env, "Rest", rest)
	env.Notify("Break over", "Back to work.")
	fmt.Fprintln(env.Out, "\nBreak over.")
	return nil
}

// countdown prints a ticking clock for the given number of minutes.
//
// It sleeps through env.Sleep so tests can run it instantly; the real
// environment supplies time.Sleep.
func countdown(env *Env, phase string, minutes int) {
	for remaining := minutes * 60; remaining > 0; remaining-- {
		fmt.Fprintf(env.Out, "\r%s  %s  ", phase, core.FormatTime(remaining))
		env.Sleep(time.Second)
	}
	fmt.Fprintf(env.Out, "\r%s  %s  ", phase, core.FormatTime(0))
}

func intArg(args []string, i int) (int, error) {
	if i+1 >= len(args) {
		return 0, usageErrorf("%s needs a number", args[i])
	}
	value, err := strconv.Atoi(args[i+1])
	if err != nil {
		return 0, usageErrorf("%s needs a number, got %q", args[i], args[i+1])
	}
	return value, nil
}

// --- tasks -----------------------------------------------------------

func cmdTask(env *Env, args []string) error {
	if len(args) == 0 {
		return usageErrorf("task needs a subcommand: list, add, done or rm")
	}
	tasks := env.Store.LoadTasks()
	projects := env.Store.LoadProjects()

	switch args[0] {
	case "list":
		if len(tasks) == 0 {
			fmt.Fprintln(env.Out, "No tasks yet.")
			return nil
		}
		w := tabwriter.NewWriter(env.Out, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\t \tTASK\tPROJECT\tPOMODOROS")
		for _, t := range tasks {
			mark := " "
			if t.Done {
				mark = "x"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", t.ID, mark, t.Title,
				projects.NameOf(t.ProjectID, "-"), t.ProgressLabel())
		}
		return w.Flush()

	case "add":
		if len(args) < 2 {
			return usageErrorf("task add needs a title")
		}
		estimate := 1
		if len(args) >= 3 {
			value, err := strconv.Atoi(args[2])
			if err != nil {
				return usageErrorf("estimate must be a number, got %q", args[2])
			}
			estimate = value
		}
		projectID := ""
		if len(args) >= 4 {
			p, err := projects.Find(args[3])
			if err != nil {
				return err
			}
			projectID = p.ID
		}
		task, err := tasks.Add(args[1], estimate, projectID)
		if err != nil {
			return err
		}
		if err := env.Store.SaveTasks(tasks); err != nil {
			return err
		}
		fmt.Fprintf(env.Out, "Added %s  %s (%s)\n", task.ID, task.Title,
			task.ProgressLabel())
		return nil

	case "done":
		if len(args) < 2 {
			return usageErrorf("task done needs a task id")
		}
		task, err := tasks.Find(args[1])
		if err != nil {
			return err
		}
		task.SetDone(!task.Done)
		if err := env.Store.SaveTasks(tasks); err != nil {
			return err
		}
		state := "reopened"
		if task.Done {
			state = "completed"
		}
		fmt.Fprintf(env.Out, "%s %s\n", task.Title, state)
		return nil

	case "rm":
		if len(args) < 2 {
			return usageErrorf("task rm needs a task id")
		}
		if !tasks.Delete(args[1]) {
			return fmt.Errorf("task %q: %w", args[1], core.ErrNotFound)
		}
		if err := env.Store.SaveTasks(tasks); err != nil {
			return err
		}
		fmt.Fprintln(env.Out, "Deleted.")
		return nil

	default:
		return usageErrorf("unknown task subcommand %q", args[0])
	}
}

// --- projects --------------------------------------------------------

func cmdProject(env *Env, args []string) error {
	if len(args) == 0 {
		return usageErrorf("project needs a subcommand: list, add, parent or done")
	}
	projects := env.Store.LoadProjects()
	tasks := env.Store.LoadTasks()
	sessions := env.Store.LoadSessions()
	today := env.Today()

	switch args[0] {
	case "list":
		if len(projects) == 0 {
			fmt.Fprintln(env.Out, "No projects yet.")
			return nil
		}
		w := tabwriter.NewWriter(env.Out, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tPROJECT\tSTATUS\tTASKS\tPOMODOROS\tFOCUSED\tDUE")
		// Roots first, each followed immediately by its subprojects
		// indented one level deeper, recursively — a tree, not a flat
		// list, so a subproject always reads as filed under its parent.
		var printProject func(p *core.Project, depth int)
		printProject = func(p *core.Project, depth int) {
			s := projects.Summarize(p, tasks, sessions, today)
			due := "-"
			if !p.Due.IsZero() {
				due = p.Due.String()
			}
			fmt.Fprintf(w, "%s\t%s%s\t%s\t%d/%d (%s)\t%d/%d\t%s\t%s\n",
				p.ID, strings.Repeat("  ", depth), p.Name, s.Health,
				s.TasksDone, s.TasksTotal, core.PercentLabel(s.TaskFraction),
				s.PomodorosDone, s.PomodorosEstimated,
				core.FormatMinutes(s.Minutes), due)
			for _, child := range projects.Children(p.ID) {
				printProject(child, depth+1)
			}
		}
		for _, p := range projects.Children("") {
			printProject(p, 0)
		}
		return w.Flush()

	case "add":
		if len(args) < 2 {
			return usageErrorf("project add needs a name")
		}
		var due core.Date
		if len(args) >= 3 && args[2] != "-" {
			parsed, err := core.ParseDate(args[2])
			if err != nil {
				return usageErrorf("due date must be YYYY-MM-DD, got %q", args[2])
			}
			due = parsed
		}
		var parent *core.Project
		if len(args) >= 4 {
			p, err := projects.Find(args[3])
			if err != nil {
				return err
			}
			parent = p
		}
		project, err := projects.Add(args[1], "", due, today)
		if err != nil {
			return err
		}
		if parent != nil {
			// Cannot fail: project is brand new and cannot already be an
			// ancestor of parent, so no cycle is possible here.
			_ = projects.SetParent(project.ID, parent.ID)
		}
		if err := env.Store.SaveProjects(projects); err != nil {
			return err
		}
		fmt.Fprintf(env.Out, "Added %s  %s\n", project.ID, project.Name)
		return nil

	case "parent":
		if len(args) < 3 {
			return usageErrorf("project parent needs a project id and a parent id (or - to clear)")
		}
		parentID := args[2]
		if parentID == "-" {
			parentID = ""
		}
		if err := projects.SetParent(args[1], parentID); err != nil {
			return err
		}
		if err := env.Store.SaveProjects(projects); err != nil {
			return err
		}
		project, err := projects.Find(args[1])
		if err != nil {
			return err
		}
		if parentID == "" {
			fmt.Fprintf(env.Out, "%s is now top-level\n", project.Name)
		} else {
			parent, err := projects.Find(parentID)
			if err != nil {
				return err
			}
			fmt.Fprintf(env.Out, "%s filed under %s\n", project.Name, parent.Name)
		}
		return nil

	case "done":
		if len(args) < 2 {
			return usageErrorf("project done needs a project id")
		}
		project, err := projects.Find(args[1])
		if err != nil {
			return err
		}
		project.Complete()
		if err := env.Store.SaveProjects(projects); err != nil {
			return err
		}
		fmt.Fprintf(env.Out, "%s completed\n", project.Name)
		return nil

	default:
		return usageErrorf("unknown project subcommand %q", args[0])
	}
}

// --- habits ----------------------------------------------------------

func cmdHabit(env *Env, args []string) error {
	if len(args) == 0 {
		return usageErrorf("habit needs a subcommand: list, add or check")
	}
	habits := env.Store.LoadHabits()
	today := env.Today()

	switch args[0] {
	case "list":
		if len(habits) == 0 {
			fmt.Fprintln(env.Out, "No habits yet.")
			return nil
		}
		w := tabwriter.NewWriter(env.Out, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tHABIT\tSCHEDULE\tLAST 7\tSTREAK\t30D")
		for _, h := range habits {
			grid := strings.Builder{}
			for _, day := range h.RecentWindow(7, today) {
				switch {
				case day.Done:
					grid.WriteString("#")
				case day.Scheduled:
					grid.WriteString(".")
				default:
					grid.WriteString(" ")
				}
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\n",
				h.ID, h.Name, h.ScheduleLabel(), grid.String(),
				h.CurrentStreak(today),
				core.PercentLabel(h.CompletionRate(30, today)))
		}
		return w.Flush()

	case "add":
		if len(args) < 2 {
			return usageErrorf("habit add needs a name")
		}
		days := core.Daily
		if len(args) >= 3 {
			parsed, err := parseSchedule(args[2])
			if err != nil {
				return err
			}
			days = parsed
		}
		habit, err := habits.Add(args[1], days, today)
		if err != nil {
			return err
		}
		if err := env.Store.SaveHabits(habits); err != nil {
			return err
		}
		fmt.Fprintf(env.Out, "Added %s  %s (%s)\n", habit.ID, habit.Name,
			habit.ScheduleLabel())
		return nil

	case "check":
		if len(args) < 2 {
			return usageErrorf("habit check needs a habit id")
		}
		habit, err := habits.Find(args[1])
		if err != nil {
			return err
		}
		habit.Toggle(today)
		if err := env.Store.SaveHabits(habits); err != nil {
			return err
		}
		state := "cleared"
		if habit.IsDone(today) {
			state = "kept"
		}
		fmt.Fprintf(env.Out, "%s %s today (streak %d)\n",
			habit.Name, state, habit.CurrentStreak(today))
		return nil

	default:
		return usageErrorf("unknown habit subcommand %q", args[0])
	}
}

func parseSchedule(name string) ([]int, error) {
	switch strings.ToLower(name) {
	case "daily", "every-day":
		return core.Daily, nil
	case "weekdays":
		return core.Weekdays, nil
	case "weekends":
		return core.Weekends, nil
	default:
		return nil, usageErrorf("schedule must be daily, weekdays or weekends, got %q", name)
	}
}

// --- history ---------------------------------------------------------

func cmdHistory(env *Env, args []string) error {
	limit := 10
	if len(args) >= 1 {
		value, err := strconv.Atoi(args[0])
		if err != nil || value < 1 {
			return usageErrorf("history needs a positive number, got %q", args[0])
		}
		limit = value
	}

	sessions := env.Store.LoadSessions()
	if len(sessions) == 0 {
		fmt.Fprintln(env.Out, "No sessions recorded yet.")
		return nil
	}
	if limit > len(sessions) {
		limit = len(sessions)
	}

	w := tabwriter.NewWriter(env.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "DATE\tSTART\tMIN\tTASK\tPROJECT")
	// Newest first.
	for i := len(sessions) - 1; i >= len(sessions)-limit; i-- {
		s := sessions[i]
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n",
			s.Day(), s.Start.Time().Format("15:04"), s.WorkMinutes,
			orDash(s.TaskTitle), orDash(s.ProjectName))
	}
	return w.Flush()
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
