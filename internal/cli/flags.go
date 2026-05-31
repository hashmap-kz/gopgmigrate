package cli

import "github.com/urfave/cli/v3"

// Each command calls these to get fresh flag instances.
// Sharing the same *cli.StringFlag pointer across commands
// breaks env var resolution in urfave/cli v3.

func flagDSN() cli.Flag {
	return &cli.StringFlag{
		Name:     "dsn",
		Usage:    "PostgreSQL connection string",
		Required: true,
		Sources:  cli.EnvVars("PGMIGRATE_DSN"),
	}
}

func flagDir() cli.Flag {
	return &cli.StringFlag{
		Name:    "dir",
		Aliases: []string{"d"},
		Usage:   "path to migrations directory",
		Value:   "migrations",
		Sources: cli.EnvVars("PGMIGRATE_DIR"),
	}
}

func flagTable() cli.Flag {
	return &cli.StringFlag{
		Name:    "table",
		Usage:   "tracking table name (overrides manifest)",
		Sources: cli.EnvVars("PGMIGRATE_TABLE"),
	}
}
