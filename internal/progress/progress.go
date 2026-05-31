package progress

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/hashmap-kz/gopgmigrate/v2/internal/x/fmtx"
)

type Status string

const (
	StatusRun    Status = "run"
	StatusOK     Status = "ok"
	StatusSkip   Status = "skip"
	StatusFail   Status = "fail"
	StatusBegin  Status = "begin"
	StatusCommit Status = "commit"
	StatusAbort  Status = "abort"
)

type Mode string

const (
	ModeTx     Mode = "tx"
	ModeNoTx   Mode = "no-tx"
	ModeAtomic Mode = "atomic"
	ModeRepeat Mode = "repeat"
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
	Files      int
	Statements int
	Took       time.Duration
	Descr      string
}

type Table struct {
	w io.Writer
}

func NewTable(w io.Writer) *Table {
	return &Table{w: w}
}

func (t *Table) Header() {
	fmtx.Fprintf(
		t.w,
		"%-7s %-7s %-10s %-8s %-16s %s\n",
		"STATUS",
		"MODE",
		"DONE",
		"TIME",
		"DESCR",
		"TARGET",
	)
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

	descr := r.Descr
	if descr == "" {
		descr = "-"
	}

	fmtx.Fprintf(
		t.w,
		"%-7s %-7s %-10s %-8s %-16s %s\n",
		status,
		mode,
		done,
		took,
		descr,
		r.Target,
	)
}

func (t *Table) Run(target string, mode Mode) {
	t.Row(&Row{
		Status: StatusRun,
		Mode:   mode,
		Target: target,
	})
}

func (t *Table) OK(target string, mode Mode, done Done) {
	t.Row(&Row{
		Status: StatusOK,
		Mode:   mode,
		Done:   formatDone(done),
		Took:   done.Took,
		Descr:  done.Descr,
		Target: target,
	})
}

func (t *Table) Skip(target string, mode Mode, reason string) {
	t.Row(&Row{
		Status: StatusSkip,
		Mode:   mode,
		Descr:  reason,
		Target: target,
	})
}

func (t *Table) Fail(target string, mode Mode, took time.Duration, descr string) {
	t.Row(&Row{
		Status: StatusFail,
		Mode:   mode,
		Took:   took,
		Descr:  descr,
		Target: target,
	})
}

func (t *Table) BeginAtomic(name string) {
	t.Row(&Row{
		Status: StatusBegin,
		Mode:   ModeAtomic,
		Target: atomicTarget(name),
	})
}

func (t *Table) CommitAtomic(name string, done Done) {
	t.Row(&Row{
		Status: StatusCommit,
		Mode:   ModeAtomic,
		Done:   formatDone(Done{Files: done.Files}),
		Took:   done.Took,
		Descr:  formatDescr(done),
		Target: atomicTarget(name),
	})
}

func (t *Table) AbortAtomic(name string, took time.Duration, descr string) {
	t.Row(&Row{
		Status: StatusAbort,
		Mode:   ModeAtomic,
		Took:   took,
		Descr:  descr,
		Target: atomicTarget(name),
	})
}

func (t *Table) Summary(status Status, release string, done Done) {
	t.Row(&Row{
		Status: status,
		Mode:   "",
		Done:   formatDone(Done{Files: done.Files}),
		Took:   done.Took,
		Descr:  formatDescr(done),
		Target: release,
	})
}

func atomicTarget(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "atomic"
	}
	return "atomic/" + name
}

func formatDone(d Done) string {
	switch {
	case d.Files > 0:
		return fmt.Sprintf("%d files", d.Files)
	case d.Statements > 0:
		return fmt.Sprintf("%d stmt%s", d.Statements, plural(d.Statements))
	default:
		return "-"
	}
}

func formatDescr(d Done) string {
	if d.Descr != "" {
		return d.Descr
	}
	if d.Files > 0 && d.Statements > 0 {
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
