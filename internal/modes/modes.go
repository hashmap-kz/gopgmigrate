package modes

import (
	"fmt"

	"gopgmigrate/internal/vers"

	"gopgmigrate/pkg/ds"
)

const (
	// ModeGroup applies all pending migrations as a single "group".
	// This means that all migrations must either be executed within a single transaction (if they are transactional)
	// or all must be non-transactional.
	ModeGroup string = "group"

	// ModeMixed applies all pending migrations in separate transactional and non-transactional groups.
	// Migrations are divided into list of groups: each group contains list of files transactional or non-transactional, and each group is executed separately.
	ModeMixed string = "mixed"

	// ModePlain executes migrations one by one, without grouping.
	// Each migration script is applied individually in sequence.
	ModePlain string = "plain"

	DbmsVendorPostgresql = "postgresql"
	DbmsVendorClickhouse = "clickhouse"
)

type GroupEntry struct {
	Files []vers.MigrationFile
	UseTX bool
}

func ParseFilesGroupMode(input []vers.MigrationFile) (GroupEntry, error) {
	if len(input) == 0 {
		return GroupEntry{}, nil
	}

	var result []vers.MigrationFile
	useTx := vers.IsTx(input[0])

	for _, elem := range input {
		if vers.IsTx(elem) != useTx {
			return GroupEntry{}, fmt.Errorf("in group mode all files should be either all TX either all NO-TX")
		}
		result = append(result, elem)
	}

	return GroupEntry{
		Files: result,
		UseTX: useTx,
	}, nil
}

func ParseFilesMixedMode(input []vers.MigrationFile) ([]GroupEntry, error) {
	stack := ds.NewStack(input)
	result := []GroupEntry{}

	for !stack.IsEmpty() {
		chain, hasElements := cutChain(stack)
		if hasElements {
			result = append(result, chain)
		}
	}

	// Check that we skip nothing
	total := 0
	for _, elem := range result {
		total = total + len(elem.Files)
	}
	if total != len(input) {
		return nil, fmt.Errorf("error splitting files into batches")
	}

	return result, nil
}

func cutChain(stack *ds.Stack[vers.MigrationFile]) (GroupEntry, bool) {
	if stack.IsEmpty() {
		return GroupEntry{}, false
	}

	var tmp []vers.MigrationFile
	for !stack.IsEmpty() {
		cur, _ := stack.Pop()
		tmp = append(tmp, cur)

		nex, _ := stack.Peek()
		if vers.IsTx(cur) != vers.IsTx(nex) {
			break
		}
	}

	if len(tmp) == 0 {
		return GroupEntry{}, false
	}

	return GroupEntry{
		Files: tmp,
		UseTX: vers.IsTx(tmp[0]),
	}, true
}
