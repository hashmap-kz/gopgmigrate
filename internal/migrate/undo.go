package migrate

func FindCorrespondingUndoScript(undoScripts []MigrationFile, doScript MigrationFile) (*MigrationFile, bool, error) {
	versionDo, err := parseVersionDo(doScript.Base)
	if err != nil {
		return nil, false, err
	}
	for _, elem := range undoScripts {
		versionUndo, err := parseVersionUndo(elem.Base)
		if err != nil {
			return nil, false, err
		}
		if versionUndo == versionDo {
			return &elem, true, nil
		}
	}
	return nil, false, nil
}
