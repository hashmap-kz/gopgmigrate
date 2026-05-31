package cli

import (
	"context"
	"fmt"

	"github.com/hashmap-kz/gopgmigrate/v2/pkg/migrator"
	"github.com/urfave/cli/v3"
)

func CmdPlan() *cli.Command {
	return &cli.Command{
		Name:  "plan",
		Usage: "show pending migrations without applying (exits 2 if any are pending)",
		Flags: []cli.Flag{
			flagDSN(),
			flagDir(),
			flagTable(),
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			m, err := migrator.NewWithDSN(cmd.String("dsn"), migrator.Config{
				Dir:   cmd.String("dir"),
				Table: cmd.String("table"),
			})
			if err != nil {
				return err
			}
			defer m.Close()

			statuses, err := m.Status(ctx)
			if err != nil {
				return err
			}

			var pending []migrator.EntryStatus
			for _, s := range statuses {
				if s.Pending {
					pending = append(pending, s)
				}
			}

			if len(pending) == 0 {
				fmt.Println("nothing to apply.")
				return nil
			}

			pathW := 0
			for _, s := range pending {
				if len(s.Path) > pathW {
					pathW = len(s.Path)
				}
			}

			fmt.Printf("pending migrations (%d):\n\n", len(pending))
			for _, s := range pending {
				fmt.Printf("  %-*s  %s\n", pathW, s.Path, s.Kind)
			}
			fmt.Println("\nrun 'apply' to execute.")
			return cli.Exit("", 2)
		},
	}
}
