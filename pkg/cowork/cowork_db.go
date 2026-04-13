// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package cowork

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/wavetermdev/waveterm/pkg/wps"
	"github.com/wavetermdev/waveterm/pkg/wshrpc"
	"github.com/wavetermdev/waveterm/pkg/wstore"
)

func CreateTask(ctx context.Context, task *wshrpc.CoworkTask) error {
	if task.TaskId == "" {
		task.TaskId = uuid.New().String()
	}
	now := time.Now().Unix()
	task.CreatedAt = now
	task.UpdatedAt = now
	if task.Status == "" {
		task.Status = "pending"
	}
	if task.Priority == "" {
		task.Priority = "medium"
	}
	return wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
		tx.Exec(`INSERT INTO cowork_tasks (task_id, title, description, priority, status, assigned_worker, created_at, updated_at, completed_at, result, error, progress)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			task.TaskId, task.Title, task.Description, task.Priority, task.Status, task.AssignedWorker,
			task.CreatedAt, task.UpdatedAt, task.CompletedAt, task.Result, task.Error, task.Progress)
		return nil
	})
}

func GetTask(ctx context.Context, taskId string) (*wshrpc.CoworkTask, error) {
	db := wstore.GetGlobalDB()
	var task wshrpc.CoworkTask
	err := db.Get(&task, `SELECT task_id as taskid, title, description, priority, status, assigned_worker as assignedworker, created_at as createdat, updated_at as updatedat, completed_at as completedat, result, error, progress FROM cowork_tasks WHERE task_id = ?`, taskId)
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func UpdateTask(ctx context.Context, task *wshrpc.CoworkTask) error {
	task.UpdatedAt = time.Now().Unix()
	return wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
		tx.Exec(`UPDATE cowork_tasks SET title=?, description=?, priority=?, status=?, assigned_worker=?, updated_at=?, completed_at=?, result=?, error=?, progress=? WHERE task_id=?`,
			task.Title, task.Description, task.Priority, task.Status, task.AssignedWorker,
			task.UpdatedAt, task.CompletedAt, task.Result, task.Error, task.Progress, task.TaskId)
		return nil
	})
}

func DeleteTask(ctx context.Context, taskId string) error {
	return wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
		tx.Exec(`DELETE FROM cowork_tasks WHERE task_id = ?`, taskId)
		return nil
	})
}

func ListTasks(ctx context.Context, status, priority string) ([]*wshrpc.CoworkTask, error) {
	db := wstore.GetGlobalDB()
	var tasks []*wshrpc.CoworkTask
	var err error
	if status != "" && priority != "" {
		err = db.Select(&tasks, `SELECT task_id as taskid, title, description, priority, status, assigned_worker as assignedworker, created_at as createdat, updated_at as updatedat, completed_at as completedat, result, error, progress FROM cowork_tasks WHERE status = ? AND priority = ? ORDER BY created_at DESC`, status, priority)
	} else if status != "" {
		err = db.Select(&tasks, `SELECT task_id as taskid, title, description, priority, status, assigned_worker as assignedworker, created_at as createdat, updated_at as updatedat, completed_at as completedat, result, error, progress FROM cowork_tasks WHERE status = ? ORDER BY created_at DESC`, status)
	} else if priority != "" {
		err = db.Select(&tasks, `SELECT task_id as taskid, title, description, priority, status, assigned_worker as assignedworker, created_at as createdat, updated_at as updatedat, completed_at as completedat, result, error, progress FROM cowork_tasks WHERE priority = ? ORDER BY created_at DESC`, priority)
	} else {
		err = db.Select(&tasks, `SELECT task_id as taskid, title, description, priority, status, assigned_worker as assignedworker, created_at as createdat, updated_at as updatedat, completed_at as completedat, result, error, progress FROM cowork_tasks ORDER BY created_at DESC`)
	}
	if err != nil {
		return nil, err
	}
	return tasks, nil
}

func RegisterWorker(ctx context.Context, worker *wshrpc.CoworkWorker) error {
	if worker.WorkerId == "" {
		worker.WorkerId = uuid.New().String()
	}
	now := time.Now().Unix()
	worker.CreatedAt = now
	worker.LastActiveAt = now
	if worker.Status == "" {
		worker.Status = "idle"
	}
	return wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
		tx.Exec(`INSERT INTO cowork_workers (worker_id, name, tool, custom_cmd, status, assigned_task, block_id, tab_id, created_at, last_active_at, last_output_hash, error_msg)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			worker.WorkerId, worker.Name, worker.Tool, worker.CustomCmd, worker.Status, worker.AssignedTask,
			worker.BlockId, worker.TabId, worker.CreatedAt, worker.LastActiveAt, worker.LastOutputHash, worker.ErrorMsg)
		return nil
	})
}

func GetWorker(ctx context.Context, workerId string) (*wshrpc.CoworkWorker, error) {
	db := wstore.GetGlobalDB()
	var worker wshrpc.CoworkWorker
	err := db.Get(&worker, `SELECT worker_id as workerid, name, tool, custom_cmd as customcmd, status, assigned_task as assignedtask, block_id as blockid, tab_id as tabid, created_at as createdat, last_active_at as lastactiveat, last_output_hash as lastoutputhash, error_msg as errormsg FROM cowork_workers WHERE worker_id = ?`, workerId)
	if err != nil {
		return nil, err
	}
	return &worker, nil
}

func UpdateWorker(ctx context.Context, worker *wshrpc.CoworkWorker) error {
	worker.LastActiveAt = time.Now().Unix()
	return wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
		tx.Exec(`UPDATE cowork_workers SET name=?, tool=?, custom_cmd=?, status=?, assigned_task=?, block_id=?, tab_id=?, last_active_at=?, last_output_hash=?, error_msg=? WHERE worker_id=?`,
			worker.Name, worker.Tool, worker.CustomCmd, worker.Status, worker.AssignedTask,
			worker.BlockId, worker.TabId, worker.LastActiveAt, worker.LastOutputHash, worker.ErrorMsg, worker.WorkerId)
		return nil
	})
}

func DeleteWorker(ctx context.Context, workerId string) error {
	return wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
		tx.Exec(`DELETE FROM cowork_workers WHERE worker_id = ?`, workerId)
		return nil
	})
}

func ListWorkers(ctx context.Context) ([]*wshrpc.CoworkWorker, error) {
	db := wstore.GetGlobalDB()
	var workers []*wshrpc.CoworkWorker
	err := db.Select(&workers, `SELECT worker_id as workerid, name, tool, custom_cmd as customcmd, status, assigned_task as assignedtask, block_id as blockid, tab_id as tabid, created_at as createdat, last_active_at as lastactiveat, last_output_hash as lastoutputhash, error_msg as errormsg FROM cowork_workers ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	return workers, nil
}

func AddActivity(ctx context.Context, activity *wshrpc.CoworkActivity) error {
	activity.CreatedAt = time.Now().Unix()
	return wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
		tx.Exec(`INSERT INTO cowork_activity (task_id, worker_id, type, description, meta, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
			activity.TaskId, activity.WorkerId, activity.Type, activity.Description, activity.Meta, activity.CreatedAt)
		return nil
	})
}

func ListActivities(ctx context.Context, limit int) ([]*wshrpc.CoworkActivity, error) {
	if limit <= 0 {
		limit = 100
	}
	db := wstore.GetGlobalDB()
	var activities []*wshrpc.CoworkActivity
	err := db.Select(&activities, `SELECT id, task_id as taskid, worker_id as workerid, type, description, meta, created_at as createdat FROM cowork_activity ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	return activities, nil
}

func CleanupOldActivities(ctx context.Context, maxCount int) error {
	return wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
		tx.Exec(`DELETE FROM cowork_activity WHERE id NOT IN (SELECT id FROM cowork_activity ORDER BY created_at DESC LIMIT ?)`, maxCount)
		return nil
	})
}

func GetStatus(ctx context.Context) (*wshrpc.CoworkStatusData, error) {
	db := wstore.GetGlobalDB()
	status := &wshrpc.CoworkStatusData{}

	var pendingTasks sql.NullInt64
	err := db.Get(&pendingTasks, `SELECT COUNT(*) FROM cowork_tasks WHERE status = ?`, "pending")
	if err != nil {
		return nil, fmt.Errorf("error counting pending tasks: %w", err)
	}
	status.PendingTasks = int(pendingTasks.Int64)

	var workingTasks sql.NullInt64
	err = db.Get(&workingTasks, `SELECT COUNT(*) FROM cowork_tasks WHERE status = ?`, "working")
	if err != nil {
		return nil, fmt.Errorf("error counting working tasks: %w", err)
	}
	status.WorkingTasks = int(workingTasks.Int64)

	var doneTasks sql.NullInt64
	err = db.Get(&doneTasks, `SELECT COUNT(*) FROM cowork_tasks WHERE status = ?`, "done")
	if err != nil {
		return nil, fmt.Errorf("error counting done tasks: %w", err)
	}
	status.DoneTasks = int(doneTasks.Int64)

	var failedTasks sql.NullInt64
	err = db.Get(&failedTasks, `SELECT COUNT(*) FROM cowork_tasks WHERE status = ?`, "failed")
	if err != nil {
		return nil, fmt.Errorf("error counting failed tasks: %w", err)
	}
	status.FailedTasks = int(failedTasks.Int64)

	var activeWorkers sql.NullInt64
	err = db.Get(&activeWorkers, `SELECT COUNT(*) FROM cowork_workers WHERE status = ?`, "working")
	if err != nil {
		return nil, fmt.Errorf("error counting active workers: %w", err)
	}
	status.ActiveWorkers = int(activeWorkers.Int64)

	var idleWorkers sql.NullInt64
	err = db.Get(&idleWorkers, `SELECT COUNT(*) FROM cowork_workers WHERE status = ?`, "idle")
	if err != nil {
		return nil, fmt.Errorf("error counting idle workers: %w", err)
	}
	status.IdleWorkers = int(idleWorkers.Int64)

	return status, nil
}

func PublishTaskUpdate() {
	wps.Broker.Publish(wps.WaveEvent{Event: wps.Event_CoworkTaskUpdate})
}

func PublishWorkerUpdate() {
	wps.Broker.Publish(wps.WaveEvent{Event: wps.Event_CoworkWorkerUpdate})
}
