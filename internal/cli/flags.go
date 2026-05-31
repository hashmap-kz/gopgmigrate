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

func flagManifest() cli.Flag {
	return &cli.StringFlag{
		Name:    "manifest",
		Aliases: []string{"m"},
		Usage:   "path to manifest YAML file",
		Value:   "migrations/manifest.yaml",
		Sources: cli.EnvVars("PGMIGRATE_MANIFEST"),
	}
}

func flagTable() cli.Flag {
	return &cli.StringFlag{
		Name:    "table",
		Usage:   "tracking table name (overrides manifest)",
		Sources: cli.EnvVars("PGMIGRATE_TABLE"),
	}
}
