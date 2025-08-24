package migrate

import (
	"embed"
	"fmt"
	"github.com/go-pg/migrations/v8"
	"github.com/go-pg/pg/v10"
	"log"
	"net/http"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func Migrate(db *pg.DB, cmd ...string) error {
	collection := migrations.NewCollection()
	if err := collection.DiscoverSQLMigrationsFromFilesystem(http.FS(migrationsFS), "migrations"); err != nil {
		return fmt.Errorf("DiscoverSQLMigrationsFromFilesystem: %w", err)
	}

	// Init mysteriously fails sometimes during gitlab tests with pg_class_relname_nsp_index violation,
	// probably because Gitlab postgres service has corrupted data.
	// So we swallow the error, because if something really went wrong then collection.Run will also fail.
	_, _, _ = collection.Run(db, "init") //nolint:dogsled

	oldVersion, newVersion, err := collection.Run(db, cmd...)
	if err != nil {
		return fmt.Errorf("collection.Run: %w", err)
	}
	if newVersion != oldVersion {
		log.Printf("migrated from version %d to %d\n", oldVersion, newVersion)
	} else {
		log.Printf("version is %d\n", oldVersion)
	}

	log.Printf("migrate done\n")
	return nil
}
