// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package team

import (
	"context"
	"database/sql"
	"testing"
	"time"

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

// insertTestMember creates a member row so FK constraints on team_workers are satisfied.
func insertTestMember(t *testing.T, db *sqlx.DB, memberID, name string) {
	t.Helper()
	now := time.Now().Unix()
	_, err := db.Exec(
		`INSERT INTO team_members (member_id, name, tool, created_at, updated_at) VALUES (?, ?, 'claude', ?, ?)`,
		memberID, name, now, now,
	)
	if err != nil {
		t.Fatalf("failed to insert test member: %v", err)
	}
}

// insertTestWorker creates a worker row with the given status and heartbeat.
func insertTestWorker(t *testing.T, db *sqlx.DB, workerID, memberID, status string, pid int, lastHeartbeat int64) {
	t.Helper()
	now := time.Now().Unix()
	_, err := db.Exec(
		`INSERT INTO team_workers (worker_id, member_id, name, status, pid, created_at, updated_at, last_heartbeat)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		workerID, memberID, "test-worker-"+workerID, status, pid, now, now, lastHeartbeat,
	)
	if err != nil {
		t.Fatalf("failed to insert test worker: %v", err)
	}
}

func getWorkerStatus(t *testing.T, db *sqlx.DB, workerID string) string {
	t.Helper()
	var status string
	err := db.Get(&status, `SELECT status FROM team_workers WHERE worker_id = ?`, workerID)
	if err != nil && err != sql.ErrNoRows {
		t.Fatalf("failed to get worker status: %v", err)
	}
	return status
}

func TestCheckWorkerHealth_NoWorkers(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	err := CheckWorkerHealth(ctx)
	if err != nil {
		t.Fatalf("CheckWorkerHealth should not error with no workers: %v", err)
	}
}

func TestCheckWorkerHealth_ExpiredHeartbeat(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	insertTestMember(t, db, "member-1", "Test Member")

	oldHeartbeat := time.Now().Add(-3 * time.Minute).Unix()
	insertTestWorker(t, db, "worker-1", "member-1", WorkerStatusWorking, 1, oldHeartbeat)

	err := CheckWorkerHealth(ctx)
	if err != nil {
		t.Fatalf("CheckWorkerHealth failed: %v", err)
	}

	status := getWorkerStatus(t, db, "worker-1")
	if status != WorkerStatusOffline {
		t.Errorf("expected worker status 'offline', got '%s'", status)
	}
}

func TestCheckWorkerHealth_RecentHeartbeat(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	insertTestMember(t, db, "member-1", "Test Member")

	recentHeartbeat := time.Now().Unix()
	insertTestWorker(t, db, "worker-1", "member-1", WorkerStatusWorking, 0, recentHeartbeat)

	err := CheckWorkerHealth(ctx)
	if err != nil {
		t.Fatalf("CheckWorkerHealth failed: %v", err)
	}

	status := getWorkerStatus(t, db, "worker-1")
	if status != WorkerStatusWorking {
		t.Errorf("expected worker status 'working', got '%s'", status)
	}
}

func TestCheckWorkerHealth_OfflineWorkersSkipped(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	insertTestMember(t, db, "member-1", "Test Member")

	oldHeartbeat := time.Now().Add(-10 * time.Minute).Unix()
	insertTestWorker(t, db, "worker-1", "member-1", WorkerStatusOffline, 1, oldHeartbeat)

	err := CheckWorkerHealth(ctx)
	if err != nil {
		t.Fatalf("CheckWorkerHealth failed: %v", err)
	}

	// Should remain offline (query filters out offline workers, so no change attempted)
	status := getWorkerStatus(t, db, "worker-1")
	if status != WorkerStatusOffline {
		t.Errorf("expected offline worker to remain 'offline', got '%s'", status)
	}
}

func TestCheckWorkerHealth_DeadPID(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	insertTestMember(t, db, "member-1", "Test Member")

	// Use a very high PID that definitely doesn't exist
	recentHeartbeat := time.Now().Unix()
	insertTestWorker(t, db, "worker-1", "member-1", WorkerStatusWorking, 9999999, recentHeartbeat)

	err := CheckWorkerHealth(ctx)
	if err != nil {
		t.Fatalf("CheckWorkerHealth failed: %v", err)
	}

	status := getWorkerStatus(t, db, "worker-1")
	if status != WorkerStatusOffline {
		t.Errorf("expected worker with dead PID to be 'offline', got '%s'", status)
	}
}

func TestCheckWorkerHealth_PIDZeroSkipped(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	insertTestMember(t, db, "member-1", "Test Member")

	recentHeartbeat := time.Now().Unix()
	insertTestWorker(t, db, "worker-1", "member-1", WorkerStatusIdle, 0, recentHeartbeat)

	err := CheckWorkerHealth(ctx)
	if err != nil {
		t.Fatalf("CheckWorkerHealth failed: %v", err)
	}

	status := getWorkerStatus(t, db, "worker-1")
	if status != WorkerStatusIdle {
		t.Errorf("expected worker with PID=0 to remain 'idle', got '%s'", status)
	}
}

func TestCheckWorkerHealth_MixedWorkers(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	insertTestMember(t, db, "member-1", "Test Member")

	now := time.Now().Unix()
	oldHeartbeat := time.Now().Add(-5 * time.Minute).Unix()

	// Worker 1: expired heartbeat → offline
	insertTestWorker(t, db, "worker-1", "member-1", WorkerStatusWorking, 1, oldHeartbeat)
	// Worker 2: recent heartbeat, valid PID (current process) → stays working
	insertTestWorker(t, db, "worker-2", "member-1", WorkerStatusWorking, 0, now)
	// Worker 3: already offline → skipped
	insertTestWorker(t, db, "worker-3", "member-1", WorkerStatusOffline, 1, oldHeartbeat)
	// Worker 4: recent heartbeat, dead PID → offline
	insertTestWorker(t, db, "worker-4", "member-1", WorkerStatusIdle, 9999999, now)
	// Worker 5: idle, recent heartbeat, PID=0 → stays idle
	insertTestWorker(t, db, "worker-5", "member-1", WorkerStatusIdle, 0, now)

	err := CheckWorkerHealth(ctx)
	if err != nil {
		t.Fatalf("CheckWorkerHealth failed: %v", err)
	}

	tests := []struct {
		workerID    string
		wantStatus  string
		description string
	}{
		{"worker-1", WorkerStatusOffline, "expired heartbeat should go offline"},
		{"worker-2", WorkerStatusWorking, "recent heartbeat with no PID check should stay working"},
		{"worker-3", WorkerStatusOffline, "already offline should stay offline"},
		{"worker-4", WorkerStatusOffline, "dead PID should go offline"},
		{"worker-5", WorkerStatusIdle, "idle with PID=0 should stay idle"},
	}

	for _, tt := range tests {
		status := getWorkerStatus(t, db, tt.workerID)
		if status != tt.wantStatus {
			t.Errorf("%s: worker %s expected '%s', got '%s'", tt.description, tt.workerID, tt.wantStatus, status)
		}
	}
}

func TestUpdateWorkerHeartbeat(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	insertTestMember(t, db, "member-1", "Test Member")

	initialHB := time.Now().Add(-1 * time.Minute).Unix()
	insertTestWorker(t, db, "worker-1", "member-1", WorkerStatusWorking, 0, initialHB)

	time.Sleep(10 * time.Millisecond)
	err := UpdateWorkerHeartbeat(ctx, "worker-1")
	if err != nil {
		t.Fatalf("UpdateWorkerHeartbeat failed: %v", err)
	}

	var lastHeartbeat int64
	err = db.Get(&lastHeartbeat, `SELECT last_heartbeat FROM team_workers WHERE worker_id = ?`, "worker-1")
	if err != nil {
		t.Fatalf("failed to get last_heartbeat: %v", err)
	}

	if lastHeartbeat <= initialHB {
		t.Errorf("last_heartbeat should be updated: was %d, now %d", initialHB, lastHeartbeat)
	}
}

func TestUpdateWorkerHeartbeat_NonExistent(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	// Should not error even if worker doesn't exist (UPDATE affects 0 rows)
	err := UpdateWorkerHeartbeat(ctx, "non-existent-worker")
	if err != nil {
		t.Fatalf("UpdateWorkerHeartbeat should not error for non-existent worker: %v", err)
	}
}

func TestStartHeartbeatLoop_StopsOnCancel(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		StartHeartbeatLoop(ctx)
		close(done)
	}()

	// Give the loop a moment to start, then cancel
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Loop stopped as expected
	case <-time.After(2 * time.Second):
		t.Fatal("StartHeartbeatLoop did not stop after context cancellation")
	}
}

func TestHeartbeatConstants(t *testing.T) {
	if HeartbeatInterval <= 0 {
		t.Errorf("HeartbeatInterval should be positive, got %v", HeartbeatInterval)
	}
	if HeartbeatTimeout <= HeartbeatInterval {
		t.Errorf("HeartbeatTimeout (%v) should be greater than HeartbeatInterval (%v)", HeartbeatTimeout, HeartbeatInterval)
	}
	if HeartbeatInterval != 30*time.Second {
		t.Errorf("expected HeartbeatInterval=30s, got %v", HeartbeatInterval)
	}
	if HeartbeatTimeout != 2*time.Minute {
		t.Errorf("expected HeartbeatTimeout=2m, got %v", HeartbeatTimeout)
	}
}
