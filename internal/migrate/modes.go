package migrate

import (
	"fmt"

	"gopgmigrate/pkg/ds"
)

type GroupEntry struct {
	Files []MigrationFile
	UseTX bool
}

func ParseFilesGroupMode(input []MigrationFile) (GroupEntry, error) {
	if len(input) == 0 {
		return GroupEntry{}, nil
	}

	var result []MigrationFile
	useTx := isTx(input[0])

	for _, elem := range input {
		if isTx(elem) != useTx {
			return GroupEntry{}, fmt.Errorf("in group mode all files should be either all TX either all NO-TX")
		}
		result = append(result, elem)
	}

	return GroupEntry{
		Files: result,
		UseTX: useTx,
	}, nil
}

func ParseFilesMixedMode(input []MigrationFile) ([]GroupEntry, error) {
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

func cutChain(stack *ds.Stack[MigrationFile]) (GroupEntry, bool) {
	if stack.IsEmpty() {
		return GroupEntry{}, false
	}

	var tmp []MigrationFile
	for !stack.IsEmpty() {
		cur, _ := stack.Pop()
		tmp = append(tmp, cur)

		nex, _ := stack.Peek()
		if isTx(cur) != isTx(nex) {
			break
		}
	}

	if len(tmp) == 0 {
		return GroupEntry{}, false
	}

	return GroupEntry{
		Files: tmp,
		UseTX: isTx(tmp[0]),
	}, true
}
