// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package team

import (
	"context"
	"log"
	"os"
	"syscall"
	"time"

	"github.com/wavetermdev/waveterm/pkg/wps"
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

// UpdateWorkerHeartbeat refreshes the last_heartbeat timestamp for a worker.
func UpdateWorkerHeartbeat(ctx context.Context, workerID string) error {
	return wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
		tx.Exec(
			`UPDATE team_workers SET last_heartbeat = ?, updated_at = ? WHERE worker_id = ?`,
			time.Now().Unix(), time.Now().Unix(), workerID,
		)
		return nil
	})
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

// PublishWorkerUpdate sends a WPS event notifying the frontend of worker state changes.
func PublishWorkerUpdate() {
	wps.Broker.Publish(wps.WaveEvent{Event: wps.Event_TeamWorkerUpdate})
}

func markWorkerOffline(ctx context.Context, workerID string) error {
	now := time.Now().Unix()
	return wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
		tx.Exec(
			`UPDATE team_workers SET status = ?, updated_at = ? WHERE worker_id = ?`,
			WorkerStatusOffline, now, workerID,
		)
		return nil
	})
}
