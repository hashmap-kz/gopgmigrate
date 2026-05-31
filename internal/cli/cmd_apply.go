package cli

import (
	"context"
	"os"

	"github.com/hashmap-kz/gopgmigrate/v2/pkg/migrator"
	"github.com/urfave/cli/v3"
)

func CmdApply() *cli.Command {
	return &cli.Command{
		Name:  "apply",
		Usage: "apply all pending migrations",
		Flags: []cli.Flag{
			flagDSN(),
			flagManifest(),
			flagTable(),
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			m, err := migrator.NewWithDSN(cmd.String("dsn"), migrator.Config{
				ManifestPath: cmd.String("manifest"),
				Table:        cmd.String("table"),
				Output:       os.Stdout,
			})
			if err != nil {
				return err
			}
			defer m.Close()
			return m.Run(ctx)
		},
	}
}
