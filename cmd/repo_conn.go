package cmd

import (
	"context"
	"database/sql"
	"log/slog"
	"os"

	"gopgmigrate/internal/dbms"
	"gopgmigrate/internal/history"
	"gopgmigrate/internal/history/impl"
)

func getRepoAndConn(ctx context.Context) (history.MigrateHistoryRepository, *sql.DB) {
	var err error
	var repo history.MigrateHistoryRepository
	var conn *sql.DB

	if cliOptions.dbms == dbmsVendorPostgres {
		repo = impl.NewMigrateHistoryPostgresRepository(ctx, cliOptions.historyTableName)
		conn, err = dbms.GetDatabaseConnectionPostgres(cliOptions.connStr)
		if err != nil {
			slog.Error("database connection error", slog.String("err", err.Error()))
			os.Exit(1)
		}
	} else {
		slog.Error("unknown DBMS vendor", slog.String("name", cliOptions.dbms))
		os.Exit(1)
	}

	return repo, conn
}
