package storage

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	*sql.DB
	logger *slog.Logger
}

func New(dsn string, logger *slog.Logger) (*DB, error) {
	connStr := dsn + "?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON"
	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	logger.Info("database connected", "dsn", dsn)
	return &DB{DB: db, logger: logger}, nil
}

func (d *DB) RunMigrations(migrationsPath string) error {
	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		return fmt.Errorf("reading migrations directory: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, f := range files {
		path := filepath.Join(migrationsPath, f)
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", f, err)
		}

		if _, err := d.Exec(string(content)); err != nil {
			return fmt.Errorf("executing migration %s: %w", f, err)
		}
		d.logger.Info("migration applied", "file", f)
	}

	return nil
}

func (d *DB) Close() error {
	d.logger.Info("closing database")
	return d.DB.Close()
}
