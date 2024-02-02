package postgres

import (
	"database/sql"

	_ "github.com/lib/pq" // Import the PostgreSQL driver
	"github.com/rs/zerolog/log"

	"github.com/forscht/ddrv/pkg/migrate"
)

// Driver - fot now we only support postgres
const Driver = "postgres"

// NewDb creates a new database connection using the dbUrl
// It returns the *sql.DB object representing the connection.
func NewDb(connStr string, skipMigration bool) *sql.DB {
	// next a new database connection
	db, err := sql.Open(Driver, connStr)
	if err != nil {
		log.Fatal().Err(err).Str("c", "postgres").Msg("could not open postgres connection")
	}
	// Set a limit to the maximum number of open connections to the database.
	// This is to prevent excessive resource use and ensure the database
	// doesn't become overwhelmed with connections, particularly in cases
	// where many small files are being uploaded simultaneously.
	db.SetMaxOpenConns(100)
	// Ping the database to ensure connectivity
	if err = db.Ping(); err != nil {
		log.Fatal().Err(err).Str("c", "postgres").Msg("ping failed")
	}
	// Perform database migrations
	if !skipMigration {
		if err = Migrate(db); err != nil {
			log.Fatal().Err(err).Str("c", "postgres").Msg("failed to execute migration")
		}
	}
	return db
}

// Migrate performs database migrations using the provided *sql.DB connection.
func Migrate(db *sql.DB) error {
	m := migrate.NewMigrator(db)                  // Create a new migrator instance
	m.TransactionMode = migrate.SingleTransaction // Set the transaction mode to single transaction
	return m.Exec(migrate.Up, migrations...)      // Execute the migrations in the "up" direction
}
