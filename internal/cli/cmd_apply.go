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
		Description: `Examples:
   # apply using a connection string from an environment variable
   gopgmigrate apply --dsn $DSN

   # apply using standard PG* environment variables (no --dsn needed)
   PGHOST=db PGDATABASE=mydb PGUSER=app gopgmigrate apply

   # apply from a custom directory to a custom history table
   gopgmigrate apply --dsn $DSN --dir ./db/migrations --table myschema.migrations`,
		Flags: []cli.Flag{
			flagDSN(),
			flagDir(),
			flagTable(),
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			m, err := migrator.NewWithDSN(cmd.String("dsn"), migrator.Config{
				Dir:    cmd.String("dir"),
				Table:  cmd.String("table"),
				Output: os.Stdout,
			})
			if err != nil {
				return err
			}
			defer m.Close()
			return m.Run(ctx)
		},
	}
}
