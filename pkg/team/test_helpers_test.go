// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package team

import (
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/wavetermdev/waveterm/pkg/wstore"
	dbfs "github.com/wavetermdev/waveterm/db"
	"github.com/wavetermdev/waveterm/pkg/util/migrateutil"
)

func initTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	db.Exec("PRAGMA foreign_keys = ON")
	err = migrateutil.Migrate("wstore", db.DB, dbfs.WStoreMigrationFS, "migrations-wstore")
	if err != nil {
		db.Close()
		t.Fatalf("failed to apply migrations: %v", err)
	}
	wstore.SetGlobalDBForTest(db)
	return db
}

func cleanupTestDB(t *testing.T, db *sqlx.DB) {
	t.Helper()
	wstore.SetGlobalDBForTest(nil)
	db.Close()
}
