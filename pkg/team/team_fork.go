// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package team

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/wavetermdev/waveterm/pkg/wstore"
)

// memberInfo holds the minimal member data needed by fork/recycle operations.
type memberInfo struct {
	Name           string `db:"name"`
	MaxConcurrency int    `db:"max_concurrency"`
	ProjectID      string `db:"project_id"`
}

// workerInfo holds the minimal worker data needed by recycle operations.
type workerInfo struct {
	WorkerID string `db:"worker_id"`
	MemberID string `db:"member_id"`
	Name     string `db:"name"`
	Status   string `db:"status"`
}

// ForkWorker creates a new Worker instance for a Member.
// It first tries to reuse an existing offline worker with the same name,
// falling back to creating a new one. Checks MaxConcurrency and records Activity.
func ForkWorker(ctx context.Context, memberID string) (*TeamWorker, error) {
	member, err := getMemberInfo(ctx, memberID)
	if err != nil {
		return nil, fmt.Errorf("fork worker: %w", err)
	}

	db := wstore.GetGlobalDB()
	var existingWorkers []TeamWorker
	err = db.Select(&existingWorkers,
		`SELECT worker_id, member_id, name, status FROM team_workers WHERE member_id = ? AND name = ? AND status = ?`,
		memberID, member.Name, WorkerStatusOffline)
	if err == nil && len(existingWorkers) > 0 {
		reused := existingWorkers[0]
		now := time.Now().Unix()
		err = wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
			tx.Exec(`UPDATE team_workers SET status=?, assigned_task_id='', block_id='', tab_id='', pid=0, project_id=?, updated_at=?, last_heartbeat=? WHERE worker_id=?`,
				WorkerStatusIdle, member.ProjectID, now, now, reused.WorkerID)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("fork worker: failed to reuse offline worker: %w", err)
		}

		reused.Status = WorkerStatusIdle
		reused.ProjectID = member.ProjectID
		reused.AssignedTaskID = ""
		reused.BlockID = ""
		reused.UpdatedAt = now
		reused.LastHeartbeat = now

		addActivity(ctx, &TeamActivity{
			MemberID:    memberID,
			WorkerID:    reused.WorkerID,
			Type:        "reused",
			Description: fmt.Sprintf("reused offline worker %q from member %q", reused.Name, member.Name),
		})

		return &reused, nil
	}

	activeCount, err := countActiveWorkers(ctx, memberID)
	if err != nil {
		return nil, fmt.Errorf("fork worker: %w", err)
	}
	if activeCount >= member.MaxConcurrency {
		return nil, fmt.Errorf("fork worker: member %q has reached max concurrency (%d/%d)",
			memberID, activeCount, member.MaxConcurrency)
	}

	now := time.Now().Unix()
	workerName := member.Name
	workerID := generateWorkerID(workerName)
	worker := &TeamWorker{
		WorkerID:      workerID,
		MemberID:      memberID,
		Name:          workerName,
		Status:        WorkerStatusIdle,
		ProjectID:     member.ProjectID,
		CreatedAt:     now,
		UpdatedAt:     now,
		LastHeartbeat: now,
	}

	err = wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
		tx.Exec(`INSERT INTO team_workers (worker_id, member_id, name, status, assigned_task_id, block_id, tab_id, pid, project_id, session_id, created_at, updated_at, last_heartbeat)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			worker.WorkerID, worker.MemberID, worker.Name, worker.Status,
			worker.AssignedTaskID, worker.BlockID, worker.TabID, worker.PID,
			worker.ProjectID, worker.SessionID, worker.CreatedAt, worker.UpdatedAt, worker.LastHeartbeat)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("fork worker: failed to create worker: %w", err)
	}

	addActivity(ctx, &TeamActivity{
		MemberID:    memberID,
		WorkerID:    worker.WorkerID,
		Type:        "forked",
		Description: fmt.Sprintf("forked worker %q from member %q", worker.Name, member.Name),
	})

	return worker, nil
}

// RecycleWorker marks a Worker as offline, clears its runtime fields,
// and records Activity. This releases the terminal block binding.
func RecycleWorker(ctx context.Context, workerID string) error {
	worker, err := getWorkerInfo(ctx, workerID)
	if err != nil {
		return fmt.Errorf("recycle worker: %w", err)
	}

	if err := ValidateWorkerTransition(worker.Status, WorkerStatusOffline); err != nil {
		return fmt.Errorf("recycle worker: %w", err)
	}

	now := time.Now().Unix()
	err = wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
		var assignedTaskID string
		tx.Get(&assignedTaskID, `SELECT assigned_task_id FROM team_workers WHERE worker_id=?`, workerID)
		if assignedTaskID != "" {
			tx.Exec(`UPDATE team_tasks SET status=?, error=?, updated_at=?, completed_at=? WHERE task_id=? AND status=?`,
				TaskStatusFailed, "worker recycled", now, now, assignedTaskID, TaskStatusWorking)
		}
		tx.Exec(`DELETE FROM team_workers WHERE worker_id=?`, workerID)
		return nil
	})
	if err != nil {
		return fmt.Errorf("recycle worker: failed to delete worker: %w", err)
	}

	PublishWorkerUpdate()
	PublishTaskUpdate()

	member, _ := GetMember(ctx, worker.MemberID)
	CleanupWorkerConfig(member)

	addActivity(ctx, &TeamActivity{
		MemberID:    worker.MemberID,
		WorkerID:    workerID,
		Type:        "recycled",
		Description: fmt.Sprintf("recycled worker %q", worker.Name),
	})

	return nil
}

// GetNextWorkerNumber returns the next sequential number for a new Worker
// of the given Member. It parses existing worker names (pattern: MemberName-N)
// and returns max(N) + 1, or 1 if no workers exist.
func GetNextWorkerNumber(ctx context.Context, memberID string) (int, error) {
	member, err := getMemberInfo(ctx, memberID)
	if err != nil {
		return 0, err
	}
	return getNextWorkerNumber(ctx, memberID, member.Name)
}

func getNextWorkerNumber(ctx context.Context, memberID string, memberName string) (int, error) {
	db := wstore.GetGlobalDB()
	var names []string
	err := db.Select(&names, `SELECT name FROM team_workers WHERE member_id = ?`, memberID)
	if err != nil {
		return 0, fmt.Errorf("get next worker number: %w", err)
	}

	prefix := memberName + "-"
	maxNum := 0
	for _, name := range names {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		suffix := strings.TrimPrefix(name, prefix)
		num, err := strconv.Atoi(suffix)
		if err != nil {
			continue
		}
		if num > maxNum {
			maxNum = num
		}
	}
	return maxNum + 1, nil
}

func countActiveWorkers(ctx context.Context, memberID string) (int, error) {
	db := wstore.GetGlobalDB()
	var count sql.NullInt64
	err := db.Get(&count, `SELECT COUNT(*) FROM team_workers WHERE member_id = ? AND status != ?`,
		memberID, WorkerStatusOffline)
	if err != nil {
		return 0, fmt.Errorf("count active workers: %w", err)
	}
	return int(count.Int64), nil
}

func getMemberInfo(ctx context.Context, memberID string) (*memberInfo, error) {
	db := wstore.GetGlobalDB()
	var info memberInfo
	err := db.Get(&info, `SELECT name, max_concurrency, project_id FROM team_members WHERE member_id = ?`, memberID)
	if err != nil {
		return nil, fmt.Errorf("member %q not found: %w", memberID, err)
	}
	return &info, nil
}

func getWorkerInfo(ctx context.Context, workerID string) (*workerInfo, error) {
	db := wstore.GetGlobalDB()
	var info workerInfo
	err := db.Get(&info, `SELECT worker_id, member_id, name, status FROM team_workers WHERE worker_id = ?`, workerID)
	if err != nil {
		return nil, fmt.Errorf("worker %q not found: %w", workerID, err)
	}
	return &info, nil
}

func addActivity(ctx context.Context, activity *TeamActivity) {
	activity.CreatedAt = time.Now().Unix()
	wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
		tx.Exec(`INSERT INTO team_activity (task_id, worker_id, member_id, type, description, meta, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			activity.TaskID, activity.WorkerID, activity.MemberID,
			activity.Type, activity.Description, activity.Meta, activity.CreatedAt)
		return nil
	})
}

func base64Encode(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

func generateWorkerID(name string) string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%s-%x", base64Encode(name), b)
}
