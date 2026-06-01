package progress

import (
	"fmt"
	"io"
	"time"

	"github.com/hashmap-kz/gopgmigrate/v2/internal/x/fmtx"
)

type Status string

const (
	StatusRun  Status = "run"
	StatusOK   Status = "ok"
	StatusSkip Status = "skip"
	StatusFail Status = "fail"
)

type Mode string

const (
	ModeTx         Mode = "tx"
	ModeNoTx       Mode = "no-tx"
	ModeRepeat     Mode = "repeat"
	ModeRepeatNoTx Mode = "repeat-notx"
)

type Row struct {
	Status Status
	Mode   Mode
	Done   string
	Took   time.Duration
	Descr  string
	Target string
}

type Done struct {
	Statements int
	Took       time.Duration
}

// TotalDone carries the aggregate counters for the Summary line.
type TotalDone struct {
	Applied    int
	Skipped    int
	Errors     int
	Statements int
	Took       time.Duration
}

type Table struct {
	w io.Writer
}

func NewTable(w io.Writer) *Table {
	return &Table{w: w}
}

func (t *Table) prefix(s Status) string {
	switch s {
	case StatusOK:
		return "+"
	case StatusFail:
		return "x"
	case StatusSkip:
		return "~"
	default:
		return "-"
	}
}

const rowFmt = "%s %-7s %-7s %-10s %-8s %s\n"

func (t *Table) Header() {
	fmtx.Fprintf(t.w, "  %-7s %-7s %-10s %-8s %s\n",
		"STATUS", "MODE", "DONE", "TIME", "TARGET")
}

func (t *Table) Blank() {
	fmtx.Fprintln(t.w)
}

func (t *Table) Row(r *Row) {
	status := string(r.Status)
	if status == "" {
		status = "-"
	}
	mode := string(r.Mode)
	if mode == "" {
		mode = "-"
	}
	done := r.Done
	if done == "" {
		done = "-"
	}
	took := "-"
	if r.Took > 0 {
		took = formatDuration(r.Took)
	}
	fmtx.Fprintf(t.w, rowFmt, t.prefix(r.Status), status, mode, done, took, r.Target)
}

func (t *Table) Run(target string, mode Mode) {
	t.Row(&Row{Status: StatusRun, Mode: mode, Target: target})
}

func (t *Table) OK(target string, mode Mode, done Done) {
	t.Row(&Row{
		Status: StatusOK,
		Mode:   mode,
		Done:   formatDone(done),
		Took:   done.Took,
		Target: target,
	})
}

func (t *Table) Skip(target string, mode Mode, reason string) {
	t.Row(&Row{Status: StatusSkip, Mode: mode, Descr: reason, Target: target})
}

func (t *Table) Fail(target string, mode Mode, took time.Duration, descr string) {
	t.Row(&Row{Status: StatusFail, Mode: mode, Took: took, Descr: descr, Target: target})
}

func (t *Table) Summary(d TotalDone) {
	fmtx.Fprintf(t.w, "TOTAL: applied: %d; skipped: %d; errors: %d; time: %s; stmts: %d\n",
		d.Applied, d.Skipped, d.Errors, formatDuration(d.Took), d.Statements)
}

func formatDone(d Done) string {
	if d.Statements > 0 {
		return fmt.Sprintf("%d stmt%s", d.Statements, plural(d.Statements))
	}
	return "-"
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return d.Round(time.Millisecond).String()
	}
	return d.Round(100 * time.Millisecond).String()
}
