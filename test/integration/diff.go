//go:build integration

package integration

import (
	"fmt"
	"strings"
)

// SnapshotDiff describes what changed between two snapshots.
type SnapshotDiff struct {
	TablesAdded      []string
	TablesRemoved    []string
	TableChanges     map[string]TableDiff
	FunctionsAdded   []string
	FunctionsRemoved []string
	ViewsAdded       []string
	ViewsRemoved     []string
	ViewsChanged     []string
}

type TableDiff struct {
	ColumnsAdded       []string
	ColumnsRemoved     []string
	ColumnsChanged     []string
	ConstraintsAdded   []string
	ConstraintsRemoved []string
	IndexesAdded       []string
	IndexesRemoved     []string
	RowCountBefore     int
	RowCountAfter      int
}

func (d *TableDiff) RowsAdded() int {
	return d.RowCountAfter - d.RowCountBefore
}

func Diff(before, after *DBSnapshot) *SnapshotDiff {
	d := &SnapshotDiff{
		TableChanges: make(map[string]TableDiff),
	}

	// tables
	for name := range after.Tables {
		if _, ok := before.Tables[name]; !ok {
			d.TablesAdded = append(d.TablesAdded, name)
		}
	}
	for name := range before.Tables {
		if _, ok := after.Tables[name]; !ok {
			d.TablesRemoved = append(d.TablesRemoved, name)
		}
	}

	// column / constraint / index / row changes within existing tables
	for name, afterTable := range after.Tables {
		beforeTable, existed := before.Tables[name]
		if !existed {
			continue
		}
		td := diffTable(beforeTable, afterTable)
		if !tableDiffEmpty(td) {
			d.TableChanges[name] = td
		}
	}

	// functions
	for key := range after.Functions {
		if _, ok := before.Functions[key]; !ok {
			d.FunctionsAdded = append(d.FunctionsAdded, key)
		}
	}
	for key := range before.Functions {
		if _, ok := after.Functions[key]; !ok {
			d.FunctionsRemoved = append(d.FunctionsRemoved, key)
		}
	}

	// views
	for name, afterDef := range after.Views {
		if beforeDef, ok := before.Views[name]; !ok {
			d.ViewsAdded = append(d.ViewsAdded, name)
		} else if afterDef != beforeDef {
			d.ViewsChanged = append(d.ViewsChanged, name)
		}
	}
	for name := range before.Views {
		if _, ok := after.Views[name]; !ok {
			d.ViewsRemoved = append(d.ViewsRemoved, name)
		}
	}

	return d
}

func diffTable(before, after TableSnapshot) TableDiff {
	td := TableDiff{
		RowCountBefore: before.RowCount,
		RowCountAfter:  after.RowCount,
	}

	beforeCols := indexByName(before.Columns)
	afterCols := indexByName(after.Columns)

	for name, afterCol := range afterCols {
		if beforeCol, ok := beforeCols[name]; !ok {
			td.ColumnsAdded = append(td.ColumnsAdded, name)
		} else if beforeCol != afterCol {
			td.ColumnsChanged = append(td.ColumnsChanged, name)
		}
	}
	for name := range beforeCols {
		if _, ok := afterCols[name]; !ok {
			td.ColumnsRemoved = append(td.ColumnsRemoved, name)
		}
	}

	beforeCons := indexConstraintsByName(before.Constraints)
	afterCons := indexConstraintsByName(after.Constraints)
	for name := range afterCons {
		if _, ok := beforeCons[name]; !ok {
			td.ConstraintsAdded = append(td.ConstraintsAdded, name)
		}
	}
	for name := range beforeCons {
		if _, ok := afterCons[name]; !ok {
			td.ConstraintsRemoved = append(td.ConstraintsRemoved, name)
		}
	}

	beforeIdxs := indexIndexesByName(before.Indexes)
	afterIdxs := indexIndexesByName(after.Indexes)
	for name := range afterIdxs {
		if _, ok := beforeIdxs[name]; !ok {
			td.IndexesAdded = append(td.IndexesAdded, name)
		}
	}
	for name := range beforeIdxs {
		if _, ok := afterIdxs[name]; !ok {
			td.IndexesRemoved = append(td.IndexesRemoved, name)
		}
	}

	return td
}

// Pretty prints the diff for test failure messages.
func (d *SnapshotDiff) String() string {
	var sb strings.Builder
	for _, t := range d.TablesAdded {
		fmt.Fprintf(&sb, "  + table:    %s\n", t)
	}
	for _, t := range d.TablesRemoved {
		fmt.Fprintf(&sb, "  - table:    %s\n", t)
	}
	for _, f := range d.FunctionsAdded {
		fmt.Fprintf(&sb, "  + function: %s\n", f)
	}
	for _, f := range d.FunctionsRemoved {
		fmt.Fprintf(&sb, "  - function: %s\n", f)
	}
	for _, v := range d.ViewsAdded {
		fmt.Fprintf(&sb, "  + view:     %s\n", v)
	}
	for _, v := range d.ViewsRemoved {
		fmt.Fprintf(&sb, "  - view:     %s\n", v)
	}
	for _, v := range d.ViewsChanged {
		fmt.Fprintf(&sb, "  ~ view:     %s\n", v)
	}
	for table, td := range d.TableChanges {
		for _, c := range td.ColumnsAdded {
			fmt.Fprintf(&sb, "  + column:   %s.%s\n", table, c)
		}
		for _, c := range td.ColumnsRemoved {
			fmt.Fprintf(&sb, "  - column:   %s.%s\n", table, c)
		}
		for _, c := range td.ColumnsChanged {
			fmt.Fprintf(&sb, "  ~ column:   %s.%s\n", table, c)
		}
		if td.RowsAdded() != 0 {
			fmt.Fprintf(&sb, "  ~ rows:     %s %+d (%d → %d)\n",
				table, td.RowsAdded(), td.RowCountBefore, td.RowCountAfter)
		}
	}
	return sb.String()
}

func tableDiffEmpty(td TableDiff) bool {
	return len(td.ColumnsAdded) == 0 &&
		len(td.ColumnsRemoved) == 0 &&
		len(td.ColumnsChanged) == 0 &&
		len(td.ConstraintsAdded) == 0 &&
		len(td.ConstraintsRemoved) == 0 &&
		len(td.IndexesAdded) == 0 &&
		len(td.IndexesRemoved) == 0 &&
		td.RowCountBefore == td.RowCountAfter
}

func indexByName(cols []ColumnSnapshot) map[string]ColumnSnapshot {
	m := make(map[string]ColumnSnapshot, len(cols))
	for _, c := range cols {
		m[c.Name] = c
	}
	return m
}

func indexConstraintsByName(cs []ConstraintSnapshot) map[string]ConstraintSnapshot {
	m := make(map[string]ConstraintSnapshot, len(cs))
	for _, c := range cs {
		m[c.Name] = c
	}
	return m
}

func indexIndexesByName(idxs []IndexSnapshot) map[string]IndexSnapshot {
	m := make(map[string]IndexSnapshot, len(idxs))
	for _, i := range idxs {
		m[i.Name] = i
	}
	return m
}
