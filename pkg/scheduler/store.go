// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wstore"
)

// idDataType is used for scanning database rows
type idDataType struct {
	OId     string
	Version int
	Data    []byte
}

// AddScheduledTask adds a new scheduled task to the database
func AddScheduledTask(ctx context.Context, task *waveobj.ScheduledTask) error {
	if task.OID == "" {
		return fmt.Errorf("cannot add scheduled task with empty ID")
	}
	return wstore.DBInsert(ctx, task)
}

// GetScheduledTask retrieves a scheduled task by ID
func GetScheduledTask(ctx context.Context, id string) (*waveobj.ScheduledTask, error) {
	return wstore.DBMustGet[*waveobj.ScheduledTask](ctx, id)
}

// ListScheduledTasks retrieves all scheduled tasks
func ListScheduledTasks(ctx context.Context) ([]*waveobj.ScheduledTask, error) {
	return wstore.DBGetAllObjsByType[*waveobj.ScheduledTask](ctx, waveobj.OType_ScheduledTask)
}

// GetDueScheduledTasks retrieves all scheduled tasks that are due to run (next_run <= now and status = pending)
func GetDueScheduledTasks(ctx context.Context) ([]*waveobj.ScheduledTask, error) {
	return wstore.WithTxRtn(ctx, func(tx *wstore.TxWrap) ([]*waveobj.ScheduledTask, error) {
		now := time.Now().UnixMilli()
		query := `
			SELECT oid, version, data 
			FROM db_scheduledtask 
			WHERE json_extract(data, '$.nextrun') <= ? 
			  AND json_extract(data, '$.status') = ?
		`
		var rows []idDataType
		tx.Select(&rows, query, now, "pending")

		rtn := make([]*waveobj.ScheduledTask, 0, len(rows))
		for _, row := range rows {
			waveObj, err := waveobj.FromJson(row.Data)
			if err != nil {
				return nil, err
			}
			waveobj.SetVersion(waveObj, row.Version)
			rtn = append(rtn, waveObj.(*waveobj.ScheduledTask))
		}
		return rtn, nil
	})
}

// UpdateScheduledTask updates an existing scheduled task
func UpdateScheduledTask(ctx context.Context, task *waveobj.ScheduledTask) error {
	return wstore.DBUpdate(ctx, task)
}

// UpdateScheduledTaskFn updates a scheduled task using a function
func UpdateScheduledTaskFn(ctx context.Context, id string, updateFn func(*waveobj.ScheduledTask) error) error {
	return wstore.DBUpdateFnErr[*waveobj.ScheduledTask](ctx, id, updateFn)
}

// DeleteScheduledTask deletes a scheduled task by ID
func DeleteScheduledTask(ctx context.Context, id string) error {
	return wstore.DBDelete(ctx, waveobj.OType_ScheduledTask, id)
}

// MarkMissedOnStartup scans for tasks with next_run < now and status=pending, sets status=missed
func MarkMissedOnStartup(ctx context.Context) (int, error) {
	return wstore.WithTxRtn(ctx, func(tx *wstore.TxWrap) (int, error) {
		now := time.Now().UnixMilli()
		// Find all pending tasks that should have run already
		query := `
			SELECT oid, version, data 
			FROM db_scheduledtask 
			WHERE json_extract(data, '$.nextrun') < ? 
			  AND json_extract(data, '$.status') = ?
		`
		var rows []idDataType
		tx.Select(&rows, query, now, "pending")

		markedCount := 0
		for _, row := range rows {
			waveObj, err := waveobj.FromJson(row.Data)
			if err != nil {
				return 0, err
			}
			task := waveObj.(*waveobj.ScheduledTask)
			task.Status = "missed"
			task.Version = row.Version

			jsonData, err := waveobj.ToJson(task)
			if err != nil {
				return 0, err
			}

			updateQuery := `UPDATE db_scheduledtask SET data = ?, version = version+1 WHERE oid = ?`
			tx.Exec(updateQuery, jsonData, task.OID)
			markedCount++
		}

		return markedCount, nil
	})
}
