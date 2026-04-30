// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package team

import (
	"context"
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

func createTestMember(ctx context.Context, t *testing.T, member *TeamMember) {
	t.Helper()
	if member.MemberID == "" {
		panic("member must have a MemberID")
	}
	now := time.Now().Unix()
	member.CreatedAt = now
	member.UpdatedAt = now
	wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
		tx.Exec(`INSERT INTO team_members (member_id, name, tool, custom_cmd, description, persona,
			persona_path, skills, mcp_servers, capabilities, model, max_concurrency, max_retries,
			memory, color, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			member.MemberID, member.Name, member.Tool, member.CustomCmd, member.Description,
			member.Persona, member.PersonaPath, "[]", "[]", "[]",
			member.Model, member.MaxConcurrency, member.MaxRetries,
			member.Memory, member.Color, member.CreatedAt, member.UpdatedAt)
		return nil
	})
}

// --- ForkWorker tests ---

func TestForkWorker(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	createTestMember(ctx, t, &TeamMember{
		MemberID: "member-1", Name: "GoDev", Tool: ToolClaude,
		MaxConcurrency: 3, MaxRetries: 3, Memory: MemorySession,
	})

	worker, err := ForkWorker(ctx, "member-1")
	if err != nil {
		t.Fatalf("ForkWorker failed: %v", err)
	}
	if worker.WorkerID == "" {
		t.Error("WorkerID should be auto-generated")
	}
	if worker.MemberID != "member-1" {
		t.Errorf("expected MemberID 'member-1', got '%s'", worker.MemberID)
	}
	if worker.Name != "GoDev-1" {
		t.Errorf("expected name 'GoDev-1', got '%s'", worker.Name)
	}
	if worker.Status != WorkerStatusIdle {
		t.Errorf("expected status '%s', got '%s'", WorkerStatusIdle, worker.Status)
	}
	if worker.CreatedAt == 0 {
		t.Error("CreatedAt should be set")
	}
	if worker.LastHeartbeat == 0 {
		t.Error("LastHeartbeat should be set")
	}
}

func TestForkWorkerIncrementalNaming(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	createTestMember(ctx, t, &TeamMember{
		MemberID: "member-1", Name: "GoDev", Tool: ToolClaude,
		MaxConcurrency: 5, MaxRetries: 3, Memory: MemorySession,
	})

	w1, _ := ForkWorker(ctx, "member-1")
	w2, _ := ForkWorker(ctx, "member-1")
	w3, _ := ForkWorker(ctx, "member-1")

	if w1.Name != "GoDev-1" {
		t.Errorf("expected 'GoDev-1', got '%s'", w1.Name)
	}
	if w2.Name != "GoDev-2" {
		t.Errorf("expected 'GoDev-2', got '%s'", w2.Name)
	}
	if w3.Name != "GoDev-3" {
		t.Errorf("expected 'GoDev-3', got '%s'", w3.Name)
	}
}

func TestForkWorkerMaxConcurrency(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	createTestMember(ctx, t, &TeamMember{
		MemberID: "member-1", Name: "GoDev", Tool: ToolClaude,
		MaxConcurrency: 2, MaxRetries: 3, Memory: MemorySession,
	})

	_, err := ForkWorker(ctx, "member-1")
	if err != nil {
		t.Fatalf("first ForkWorker failed: %v", err)
	}
	_, err = ForkWorker(ctx, "member-1")
	if err != nil {
		t.Fatalf("second ForkWorker failed: %v", err)
	}
	_, err = ForkWorker(ctx, "member-1")
	if err == nil {
		t.Error("expected error when exceeding MaxConcurrency, got nil")
	}
}

func TestForkWorkerMemberNotFound(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	_, err := ForkWorker(ctx, "non-existent-member")
	if err == nil {
		t.Error("expected error for non-existent member, got nil")
	}
}

func TestForkWorkerRecordsActivity(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	createTestMember(ctx, t, &TeamMember{
		MemberID: "member-1", Name: "GoDev", Tool: ToolClaude,
		MaxConcurrency: 3, MaxRetries: 3, Memory: MemorySession,
	})

	worker, _ := ForkWorker(ctx, "member-1")

	var activities []TeamActivity
	wdb := wstore.GetGlobalDB()
	wdb.Select(&activities, `SELECT id, task_id as taskid, worker_id as workerid,
		member_id as memberid, type, description, meta, created_at as createdat
		FROM team_activity WHERE worker_id = ? ORDER BY created_at DESC`, worker.WorkerID)

	if len(activities) != 1 {
		t.Fatalf("expected 1 activity, got %d", len(activities))
	}
	if activities[0].Type != "forked" {
		t.Errorf("expected type 'forked', got '%s'", activities[0].Type)
	}
	if activities[0].MemberID != "member-1" {
		t.Errorf("expected MemberID 'member-1', got '%s'", activities[0].MemberID)
	}
}

// --- RecycleWorker tests ---

func TestRecycleWorker(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	createTestMember(ctx, t, &TeamMember{
		MemberID: "member-1", Name: "GoDev", Tool: ToolClaude,
		MaxConcurrency: 3, MaxRetries: 3, Memory: MemorySession,
	})

	worker, _ := ForkWorker(ctx, "member-1")

	err := RecycleWorker(ctx, worker.WorkerID)
	if err != nil {
		t.Fatalf("RecycleWorker failed: %v", err)
	}

	got, err := getWorkerInfo(ctx, worker.WorkerID)
	if err != nil {
		t.Fatalf("getWorkerInfo failed: %v", err)
	}
	if got.Status != WorkerStatusOffline {
		t.Errorf("expected status '%s', got '%s'", WorkerStatusOffline, got.Status)
	}

	// Verify runtime fields were cleared by querying DB directly
	var assignedTaskID, blockID, tabID string
	var pid int
	wdb := wstore.GetGlobalDB()
	wdb.Get(&assignedTaskID, `SELECT assigned_task_id FROM team_workers WHERE worker_id = ?`, worker.WorkerID)
	wdb.Get(&blockID, `SELECT block_id FROM team_workers WHERE worker_id = ?`, worker.WorkerID)
	wdb.Get(&tabID, `SELECT tab_id FROM team_workers WHERE worker_id = ?`, worker.WorkerID)
	wdb.Get(&pid, `SELECT pid FROM team_workers WHERE worker_id = ?`, worker.WorkerID)
	if assignedTaskID != "" {
		t.Errorf("expected empty AssignedTaskID, got '%s'", assignedTaskID)
	}
	if blockID != "" {
		t.Errorf("expected empty BlockID, got '%s'", blockID)
	}
	if tabID != "" {
		t.Errorf("expected empty TabID, got '%s'", tabID)
	}
	if pid != 0 {
		t.Errorf("expected PID 0, got %d", pid)
	}
}

func TestRecycleWorkerInvalidTransition(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	createTestMember(ctx, t, &TeamMember{
		MemberID: "member-1", Name: "GoDev", Tool: ToolClaude,
		MaxConcurrency: 3, MaxRetries: 3, Memory: MemorySession,
	})

	worker, _ := ForkWorker(ctx, "member-1")

	// Manually set worker to offline
	wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
		tx.Exec(`UPDATE team_workers SET status = ? WHERE worker_id = ?`, WorkerStatusOffline, worker.WorkerID)
		return nil
	})

	err := RecycleWorker(ctx, worker.WorkerID)
	if err == nil {
		t.Error("expected error when recycling already-offline worker, got nil")
	}
}

func TestRecycleWorkerNotFound(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	err := RecycleWorker(ctx, "non-existent-worker")
	if err == nil {
		t.Error("expected error for non-existent worker, got nil")
	}
}

func TestRecycleWorkerRecordsActivity(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	createTestMember(ctx, t, &TeamMember{
		MemberID: "member-1", Name: "GoDev", Tool: ToolClaude,
		MaxConcurrency: 3, MaxRetries: 3, Memory: MemorySession,
	})

	worker, _ := ForkWorker(ctx, "member-1")
	RecycleWorker(ctx, worker.WorkerID)

	var activities []TeamActivity
	wdb := wstore.GetGlobalDB()
	wdb.Select(&activities, `SELECT id, task_id as taskid, worker_id as workerid,
		member_id as memberid, type, description, meta, created_at as createdat
		FROM team_activity WHERE worker_id = ? ORDER BY created_at ASC`, worker.WorkerID)

	if len(activities) != 2 {
		t.Fatalf("expected 2 activities (forked + recycled), got %d", len(activities))
	}
	if activities[0].Type != "forked" {
		t.Errorf("expected first activity type 'forked', got '%s'", activities[0].Type)
	}
	if activities[1].Type != "recycled" {
		t.Errorf("expected second activity type 'recycled', got '%s'", activities[1].Type)
	}
}

func TestRecycleThenForkReusesNumber(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	createTestMember(ctx, t, &TeamMember{
		MemberID: "member-1", Name: "GoDev", Tool: ToolClaude,
		MaxConcurrency: 3, MaxRetries: 3, Memory: MemorySession,
	})

	w1, _ := ForkWorker(ctx, "member-1") // GoDev-1
	RecycleWorker(ctx, w1.WorkerID)

	// After recycle, worker-1 still exists in DB with offline status.
	// Next fork should be GoDev-2 (max existing number + 1).
	w2, _ := ForkWorker(ctx, "member-1")
	if w2.Name != "GoDev-2" {
		t.Errorf("expected 'GoDev-2' after recycle, got '%s'", w2.Name)
	}
}

// --- GetNextWorkerNumber tests ---

func TestGetNextWorkerNumberNoWorkers(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	createTestMember(ctx, t, &TeamMember{
		MemberID: "member-1", Name: "GoDev", Tool: ToolClaude,
		MaxConcurrency: 3, MaxRetries: 3, Memory: MemorySession,
	})

	num, err := GetNextWorkerNumber(ctx, "member-1")
	if err != nil {
		t.Fatalf("getNextWorkerNumber failed: %v", err)
	}
	if num != 1 {
		t.Errorf("expected 1 for no existing workers, got %d", num)
	}
}

func TestGetNextWorkerNumberWithExisting(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	createTestMember(ctx, t, &TeamMember{
		MemberID: "member-1", Name: "GoDev", Tool: ToolClaude,
		MaxConcurrency: 5, MaxRetries: 3, Memory: MemorySession,
	})

	ForkWorker(ctx, "member-1") // GoDev-1
	ForkWorker(ctx, "member-1") // GoDev-2

	num, err := GetNextWorkerNumber(ctx, "member-1")
	if err != nil {
		t.Fatalf("getNextWorkerNumber failed: %v", err)
	}
	if num != 3 {
		t.Errorf("expected 3, got %d", num)
	}
}

func TestGetNextWorkerNumberSkipsNonMatching(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	createTestMember(ctx, t, &TeamMember{
		MemberID: "member-1", Name: "GoDev", Tool: ToolClaude,
		MaxConcurrency: 3, MaxRetries: 3, Memory: MemorySession,
	})

	// Create a worker with a name that doesn't match the MemberName-N pattern
	now := time.Now().Unix()
	wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
		tx.Exec(`INSERT INTO team_workers (worker_id, member_id, name, status, created_at, updated_at, last_heartbeat)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			"worker-misc", "member-1", "some-random-name", WorkerStatusIdle, now, now, now)
		return nil
	})

	num, err := GetNextWorkerNumber(ctx, "member-1")
	if err != nil {
		t.Fatalf("getNextWorkerNumber failed: %v", err)
	}
	if num != 1 {
		t.Errorf("expected 1 when no matching names found, got %d", num)
	}
}

// --- CountActiveWorkers tests ---

func TestCountActiveWorkers(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	createTestMember(ctx, t, &TeamMember{
		MemberID: "member-1", Name: "GoDev", Tool: ToolClaude,
		MaxConcurrency: 5, MaxRetries: 3, Memory: MemorySession,
	})

	count, err := countActiveWorkers(ctx, "member-1")
	if err != nil {
		t.Fatalf("countActiveWorkers failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 active workers, got %d", count)
	}

	ForkWorker(ctx, "member-1") // idle
	count, _ = countActiveWorkers(ctx, "member-1")
	if count != 1 {
		t.Errorf("expected 1 active worker, got %d", count)
	}
}

// --- ValidateWorkerTransition tests ---

func TestValidateWorkerTransition(t *testing.T) {
	tests := []struct {
		from    string
		to      string
		wantErr bool
	}{
		{WorkerStatusIdle, WorkerStatusWorking, false},
		{WorkerStatusIdle, WorkerStatusOffline, false},
		{WorkerStatusWorking, WorkerStatusIdle, false},
		{WorkerStatusWorking, WorkerStatusError, false},
		{WorkerStatusWorking, WorkerStatusOffline, false},
		{WorkerStatusError, WorkerStatusIdle, false},
		{WorkerStatusError, WorkerStatusOffline, false},
		{WorkerStatusOffline, WorkerStatusIdle, false},
		{WorkerStatusIdle, WorkerStatusError, true},
		{WorkerStatusOffline, WorkerStatusWorking, true},
		{"unknown", WorkerStatusWorking, true},
		{WorkerStatusIdle, "unknown", true},
	}
	for _, tt := range tests {
		t.Run(tt.from+"->"+tt.to, func(t *testing.T) {
			err := ValidateWorkerTransition(tt.from, tt.to)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateWorkerTransition(%q, %q) error = %v, wantErr %v", tt.from, tt.to, err, tt.wantErr)
			}
		})
	}
}

// --- ValidateTaskTransition tests ---

func TestValidateTaskTransition(t *testing.T) {
	tests := []struct {
		from    string
		to      string
		wantErr bool
	}{
		{TaskStatusPending, TaskStatusAssigned, false},
		{TaskStatusPending, "cancelled", false},
		{TaskStatusAssigned, TaskStatusWorking, false},
		{TaskStatusAssigned, TaskStatusPending, false},
		{TaskStatusWorking, TaskStatusDone, false},
		{TaskStatusWorking, TaskStatusFailed, false},
		{TaskStatusWorking, TaskStatusPaused, false},
		{TaskStatusFailed, TaskStatusWorking, false},
		{TaskStatusPaused, TaskStatusWorking, false},
		{TaskStatusDone, TaskStatusWorking, true},
		{TaskStatusPending, TaskStatusDone, true},
	}
	for _, tt := range tests {
		t.Run(tt.from+"->"+tt.to, func(t *testing.T) {
			err := ValidateTaskTransition(tt.from, tt.to)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTaskTransition(%q, %q) error = %v, wantErr %v", tt.from, tt.to, err, tt.wantErr)
			}
		})
	}
}
