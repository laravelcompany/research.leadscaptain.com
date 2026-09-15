package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCreatesDatabaseDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "data", "leads.db")

	database, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { database.Close() })

	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Fatalf("database directory was not created: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database file was not created: %v", err)
	}
}

func TestOpenReportsUncreatableDatabaseDirectory(t *testing.T) {
	parentFile := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(parentFile, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Open(filepath.Join(parentFile, "leads.db"))
	if err == nil {
		t.Fatal("Open() expected an error")
	}
}
