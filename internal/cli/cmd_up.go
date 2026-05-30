package cli

import (
	"context"

	"github.com/hashmap-kz/gopgmigrate/v2/pkg/migrator"
	"github.com/urfave/cli/v3"
)

func CmdUp() *cli.Command {
	return &cli.Command{
		Name:  "up",
		Usage: "apply all pending migrations",
		Flags: []cli.Flag{
			flagDSN(),
			flagManifest(),
			flagTable(),
			flagDryRun(),
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			m, err := migrator.NewWithDSN(cmd.String("dsn"), migrator.Config{
				ManifestPath: cmd.String("manifest"),
				Table:        cmd.String("table"),
				DryRun:       cmd.Bool("dry-run"),
			})
			if err != nil {
				return err
			}
			defer m.Close()
			return m.Run(ctx)
		},
	}
}
