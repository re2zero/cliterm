// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package team

import (
	"context"
	"log"
	"os"
	"syscall"
	"time"

	"github.com/wavetermdev/waveterm/pkg/wstore"
)

// CheckWorkerHealth queries all non-offline workers, checks their PID via
// os.FindProcess, and marks them offline if the process is dead or the
// last heartbeat exceeds HeartbeatTimeout.
func CheckWorkerHealth(ctx context.Context) error {
	db := wstore.GetGlobalDB()
	var workers []TeamWorker
	err := db.Select(&workers,
		`SELECT worker_id as workerid, member_id as memberid, name, status,
		pid, last_heartbeat as lastheartbeat
		FROM team_workers WHERE status != ?`, WorkerStatusOffline)
	if err != nil {
		return err
	}

	now := time.Now().Unix()
	timeoutCutoff := now - int64(HeartbeatTimeout.Seconds())
	changed := false

	for _, w := range workers {
		if w.LastHeartbeat < timeoutCutoff {
			if err := markWorkerOffline(ctx, w.WorkerID); err != nil {
				log.Printf("[team] heartbeat: failed to mark worker %s offline: %v", w.WorkerID, err)
				continue
			}
			log.Printf("[team] heartbeat: worker %s marked offline (heartbeat expired)", w.WorkerID)
			changed = true
			continue
		}

		if w.PID > 0 {
			proc, err := os.FindProcess(w.PID)
			if err != nil {
				if err := markWorkerOffline(ctx, w.WorkerID); err != nil {
					log.Printf("[team] heartbeat: failed to mark worker %s offline: %v", w.WorkerID, err)
					continue
				}
				log.Printf("[team] heartbeat: worker %s marked offline (PID %d not found)", w.WorkerID, w.PID)
				changed = true
				continue
			}
			if err := proc.Signal(syscall.Signal(0)); err != nil {
				if err := markWorkerOffline(ctx, w.WorkerID); err != nil {
					log.Printf("[team] heartbeat: failed to mark worker %s offline: %v", w.WorkerID, err)
					continue
				}
				log.Printf("[team] heartbeat: worker %s marked offline (PID %d dead)", w.WorkerID, w.PID)
				changed = true
			}
		}
	}

	if changed {
		PublishWorkerUpdate()
	}
	return nil
}

// StartHeartbeatLoop runs periodic worker health checks until ctx is cancelled.
func StartHeartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := CheckWorkerHealth(ctx); err != nil {
				log.Printf("[team] heartbeat check error: %v", err)
			}
		}
	}
}

func markWorkerOffline(ctx context.Context, workerID string) error {
	now := time.Now().Unix()
	return wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
		tx.Exec(
			`UPDATE team_workers SET status = ?, updated_at = ? WHERE worker_id = ?`,
			WorkerStatusOffline, now, workerID,
		)
		var assignedTaskID string
		tx.Get(&assignedTaskID, `SELECT assigned_task_id FROM team_workers WHERE worker_id = ?`, workerID)
		if assignedTaskID != "" {
			tx.Exec(
				`UPDATE team_tasks SET status = ?, error = ?, updated_at = ?, completed_at = ? WHERE task_id = ? AND status = ?`,
				TaskStatusFailed, "worker went offline", now, now, assignedTaskID, TaskStatusWorking,
			)
		}
		return nil
	})
}

func CleanupWorkerByBlockId(ctx context.Context, blockId string) {
	if blockId == "" {
		return
	}
	db := wstore.GetGlobalDB()
	var workerID string
	err := db.Get(&workerID, `SELECT worker_id FROM team_workers WHERE block_id = ? AND status != ?`, blockId, WorkerStatusOffline)
	if err != nil {
		return
	}
	log.Printf("[team] block closed: cleaning up worker %s (block %s)", workerID, blockId)
	if err := markWorkerOffline(ctx, workerID); err != nil {
		log.Printf("[team] failed to cleanup worker %s on block close: %v", workerID, err)
		return
	}
	PublishWorkerUpdate()
	PublishTaskUpdate()
}
