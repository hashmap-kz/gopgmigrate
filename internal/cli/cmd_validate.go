package cli

import (
	"context"
	"fmt"

	"github.com/hashmap-kz/gopgmigrate/v2/pkg/migrator"
	"github.com/urfave/cli/v3"
)

func CmdValidate() *cli.Command {
	return &cli.Command{
		Name:  "validate",
		Usage: "check manifest integrity and file existence (no DB required)",
		Flags: []cli.Flag{
			flagManifest(),
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			m, err := migrator.NewValidateOnly(migrator.Config{
				ManifestPath: cmd.String("manifest"),
			})
			if err != nil {
				return err
			}
			if err := m.Validate(); err != nil {
				return err
			}
			fmt.Println("manifest OK")
			return nil
		},
	}
}
