package db

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := Migrate(db); err != nil {
		return nil, err
	}
	return db, nil
}
func Migrate(db *sql.DB) error {
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return err
	}
	var files []string
	for _, e := range entries {
		files = append(files, e.Name())
	}
	sort.Strings(files)
	for _, f := range files {
		b, _ := fs.ReadFile(migrationFS, "migrations/"+f)
		if _, err := db.Exec(string(b)); err != nil {
			msg := err.Error()
			if contains(msg, "duplicate column") || contains(msg, "already exists") {
				continue
			}
			return fmt.Errorf("migration %s: %w", f, err)
		}
	}
	return nil
}
func contains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
