// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package team

import (
  "context"
  "database/sql"
  "encoding/json"
  "fmt"
  "time"

  "github.com/google/uuid"
  "github.com/jmoiron/sqlx"
  "github.com/wavetermdev/waveterm/pkg/wps"
  "github.com/wavetermdev/waveterm/pkg/wstore"
)

// --- Member CRUD ---

func CreateMember(ctx context.Context, m *TeamMember) error {
  if m.MemberID == "" {
    m.MemberID = uuid.New().String()
  }
  now := time.Now().Unix()
  m.CreatedAt = now
  m.UpdatedAt = now
  if m.Tool == "" {
    m.Tool = ToolClaude
  }
  if m.Memory == "" {
    m.Memory = MemorySession
  }
  if m.MaxConcurrency == 0 {
    m.MaxConcurrency = 3
  }
  if m.MaxRetries == 0 {
    m.MaxRetries = 3
  }
  skillsJson, _ := json.Marshal(m.Skills)
  mcpJson, _ := json.Marshal(m.McpServers)
  capsJson, _ := json.Marshal(m.Capabilities)
  return wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
    tx.Exec(`INSERT INTO team_members (member_id, name, tool, custom_cmd, description, persona, persona_path, skills, mcp_servers, capabilities, model, max_concurrency, max_retries, memory, color, project_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
      m.MemberID, m.Name, m.Tool, m.CustomCmd, m.Description, m.Persona, m.PersonaPath,
      string(skillsJson), string(mcpJson), string(capsJson),
      m.Model, m.MaxConcurrency, m.MaxRetries, m.Memory, m.Color, m.ProjectID, m.CreatedAt, m.UpdatedAt)
    return nil
  })
}

func scanMember(row interface{ Scan(dest ...any) error }) (*TeamMember, error) {
  var m TeamMember
  var skillsStr, mcpStr, capsStr string
  err := row.Scan(&m.MemberID, &m.Name, &m.Tool, &m.CustomCmd, &m.Description, &m.Persona, &m.PersonaPath, &skillsStr, &mcpStr, &capsStr, &m.Model, &m.MaxConcurrency, &m.MaxRetries, &m.Memory, &m.Color, &m.ProjectID, &m.CreatedAt, &m.UpdatedAt)
  if err != nil {
    return nil, err
  }
  json.Unmarshal([]byte(skillsStr), &m.Skills)
  json.Unmarshal([]byte(mcpStr), &m.McpServers)
  json.Unmarshal([]byte(capsStr), &m.Capabilities)
  return &m, nil
}

func GetMember(ctx context.Context, memberID string) (*TeamMember, error) {
  db := wstore.GetGlobalDB()
  row := db.QueryRowx(`SELECT member_id, name, tool, custom_cmd, description, persona, persona_path, skills, mcp_servers, capabilities, model, max_concurrency, max_retries, memory, color, project_id, created_at, updated_at FROM team_members WHERE member_id = ?`, memberID)
  return scanMember(row)
}

func UpdateMember(ctx context.Context, m *TeamMember) error {
  m.UpdatedAt = time.Now().Unix()
  skillsJson, _ := json.Marshal(m.Skills)
  mcpJson, _ := json.Marshal(m.McpServers)
  capsJson, _ := json.Marshal(m.Capabilities)
  return wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
    tx.Exec(`UPDATE team_members SET name=?, tool=?, custom_cmd=?, description=?, persona=?, persona_path=?, skills=?, mcp_servers=?, capabilities=?, model=?, max_concurrency=?, max_retries=?, memory=?, color=?, project_id=?, updated_at=? WHERE member_id=?`,
      m.Name, m.Tool, m.CustomCmd, m.Description, m.Persona, m.PersonaPath,
      string(skillsJson), string(mcpJson), string(capsJson),
      m.Model, m.MaxConcurrency, m.MaxRetries, m.Memory, m.Color, m.ProjectID, m.UpdatedAt, m.MemberID)
    return nil
  })
}

func DeleteMember(ctx context.Context, memberID string) error {
  return wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
    tx.Exec(`DELETE FROM team_members WHERE member_id = ?`, memberID)
    return nil
  })
}

func ListMembers(ctx context.Context) ([]*TeamMember, error) {
  db := wstore.GetGlobalDB()
  rows, err := db.Queryx(`SELECT member_id, name, tool, custom_cmd, description, persona, persona_path, skills, mcp_servers, capabilities, model, max_concurrency, max_retries, memory, color, project_id, created_at, updated_at FROM team_members ORDER BY created_at DESC`)
  if err != nil {
    return nil, err
  }
  defer rows.Close()
  var members []*TeamMember
  for rows.Next() {
    m, err := scanMember(rows)
    if err != nil {
      return nil, err
    }
    members = append(members, m)
  }
  return members, rows.Err()
}

// --- Worker CRUD ---

func CreateWorker(ctx context.Context, w *TeamWorker) error {
  if w.WorkerID == "" {
    w.WorkerID = uuid.New().String()
  }
  now := time.Now().Unix()
  w.CreatedAt = now
  w.UpdatedAt = now
  w.LastHeartbeat = now
  if w.Status == "" {
    w.Status = WorkerStatusIdle
  }
  return wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
    tx.Exec(`INSERT INTO team_workers (worker_id, member_id, name, status, assigned_task_id, block_id, tab_id, pid, project_id, session_id, created_at, updated_at, last_heartbeat) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
      w.WorkerID, w.MemberID, w.Name, w.Status, w.AssignedTaskID, w.BlockID, w.TabID, w.PID, w.ProjectID, w.SessionID, w.CreatedAt, w.UpdatedAt, w.LastHeartbeat)
    return nil
  })
}

func scanWorker(row interface{ Scan(dest ...any) error }) (*TeamWorker, error) {
  var w TeamWorker
  err := row.Scan(&w.WorkerID, &w.MemberID, &w.Name, &w.Status, &w.AssignedTaskID, &w.BlockID, &w.TabID, &w.PID, &w.ProjectID, &w.SessionID, &w.CreatedAt, &w.UpdatedAt, &w.LastHeartbeat)
  if err != nil {
    return nil, err
  }
  return &w, nil
}

func GetWorker(ctx context.Context, workerID string) (*TeamWorker, error) {
  db := wstore.GetGlobalDB()
  row := db.QueryRowx(`SELECT worker_id, member_id, name, status, assigned_task_id, block_id, tab_id, pid, project_id, session_id, created_at, updated_at, last_heartbeat FROM team_workers WHERE worker_id = ?`, workerID)
  return scanWorker(row)
}

func UpdateWorker(ctx context.Context, w *TeamWorker) error {
  w.UpdatedAt = time.Now().Unix()
  return wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
    tx.Exec(`UPDATE team_workers SET member_id=?, name=?, status=?, assigned_task_id=?, block_id=?, tab_id=?, pid=?, project_id=?, session_id=?, updated_at=? WHERE worker_id=?`,
      w.MemberID, w.Name, w.Status, w.AssignedTaskID, w.BlockID, w.TabID, w.PID, w.ProjectID, w.SessionID, w.UpdatedAt, w.WorkerID)
    return nil
  })
}

func UpdateWorkerHeartbeat(ctx context.Context, workerID string) error {
  now := time.Now().Unix()
  return wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
    tx.Exec(`UPDATE team_workers SET last_heartbeat=?, updated_at=? WHERE worker_id=?`, now, now, workerID)
    return nil
  })
}

func DeleteWorker(ctx context.Context, workerID string) error {
  return wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
    tx.Exec(`DELETE FROM team_workers WHERE worker_id = ?`, workerID)
    return nil
  })
}

func ListWorkers(ctx context.Context, memberID string) ([]*TeamWorker, error) {
  db := wstore.GetGlobalDB()
  var rows *sqlx.Rows
  var err error
  if memberID != "" {
    rows, err = db.Queryx(`SELECT worker_id, member_id, name, status, assigned_task_id, block_id, tab_id, pid, project_id, session_id, created_at, updated_at, last_heartbeat FROM team_workers WHERE member_id = ? ORDER BY created_at DESC`, memberID)
  } else {
    rows, err = db.Queryx(`SELECT worker_id, member_id, name, status, assigned_task_id, block_id, tab_id, pid, project_id, session_id, created_at, updated_at, last_heartbeat FROM team_workers ORDER BY created_at DESC`)
  }
  if err != nil {
    return nil, err
  }
  defer rows.Close()
  var workers []*TeamWorker
  for rows.Next() {
    w, err := scanWorker(rows)
    if err != nil {
      return nil, err
    }
    workers = append(workers, w)
  }
  return workers, rows.Err()
}

// --- Task CRUD ---

func CreateTask(ctx context.Context, t *TeamTask) error {
  if t.TaskID == "" {
    t.TaskID = uuid.New().String()
  }
  now := time.Now().Unix()
  t.CreatedAt = now
  t.UpdatedAt = now
  t.OldUpdatedAt = now
  if t.Status == "" {
    t.Status = TaskStatusPending
  }
  if t.Priority == "" {
    t.Priority = PriorityMedium
  }
  if t.MaxRetries == 0 {
    t.MaxRetries = 3
  }
  depsJson, _ := json.Marshal(t.DependsOn)
  outputJson, _ := json.Marshal(t.OutputHistory)
  var nextRetry interface{}
  if t.NextRetryAt != 0 {
    nextRetry = t.NextRetryAt
  }
  var completedAt interface{}
  if t.CompletedAt != 0 {
    completedAt = t.CompletedAt
  }
  return wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
    tx.Exec(`INSERT INTO team_tasks (task_id, title, description, priority, status, assigned_member_id, assigned_worker_id, depends_on, result, error, output_history, progress, retry_count, max_retries, next_retry_at, created_at, updated_at, completed_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
      t.TaskID, t.Title, t.Description, t.Priority, t.Status, t.AssignedMemberID, t.AssignedWorkerID,
      string(depsJson), t.Result, t.Error, string(outputJson),
      t.Progress, t.RetryCount, t.MaxRetries, nextRetry, t.CreatedAt, t.UpdatedAt, completedAt)
    return nil
  })
}

func scanTask(row interface{ Scan(dest ...any) error }) (*TeamTask, error) {
  var t TeamTask
  var depsStr, outputStr string
  var nextRetry, completedAt sql.NullInt64
  err := row.Scan(&t.TaskID, &t.Title, &t.Description, &t.Priority, &t.Status, &t.AssignedMemberID, &t.AssignedWorkerID, &depsStr, &t.Result, &t.Error, &outputStr, &t.Progress, &t.RetryCount, &t.MaxRetries, &nextRetry, &t.CreatedAt, &t.UpdatedAt, &completedAt)
  if err != nil {
    return nil, err
  }
  json.Unmarshal([]byte(depsStr), &t.DependsOn)
  json.Unmarshal([]byte(outputStr), &t.OutputHistory)
  if nextRetry.Valid {
    t.NextRetryAt = nextRetry.Int64
  }
  if completedAt.Valid {
    t.CompletedAt = completedAt.Int64
  }
  t.OldUpdatedAt = t.UpdatedAt
  return &t, nil
}

func GetTask(ctx context.Context, taskID string) (*TeamTask, error) {
  db := wstore.GetGlobalDB()
  row := db.QueryRowx(`SELECT task_id, title, description, priority, status, assigned_member_id, assigned_worker_id, depends_on, result, error, output_history, progress, retry_count, max_retries, next_retry_at, created_at, updated_at, completed_at FROM team_tasks WHERE task_id = ?`, taskID)
  return scanTask(row)
}

func UpdateTask(ctx context.Context, t *TeamTask) error {
  t.UpdatedAt = time.Now().Unix()
  depsJson, _ := json.Marshal(t.DependsOn)
  outputJson, _ := json.Marshal(t.OutputHistory)
  var nextRetry interface{}
  if t.NextRetryAt != 0 {
    nextRetry = t.NextRetryAt
  } else {
    nextRetry = nil
  }
  var completedAt interface{}
  if t.CompletedAt != 0 {
    completedAt = t.CompletedAt
  } else {
    completedAt = nil
  }
  return wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
    result := tx.Exec(`UPDATE team_tasks SET title=?, description=?, priority=?, status=?, assigned_member_id=?, assigned_worker_id=?, depends_on=?, result=?, error=?, output_history=?, progress=?, retry_count=?, max_retries=?, next_retry_at=?, updated_at=?, completed_at=? WHERE task_id=? AND updated_at=?`,
      t.Title, t.Description, t.Priority, t.Status, t.AssignedMemberID, t.AssignedWorkerID,
      string(depsJson), t.Result, t.Error, string(outputJson),
      t.Progress, t.RetryCount, t.MaxRetries, nextRetry, t.UpdatedAt, completedAt, t.TaskID, t.OldUpdatedAt)
    rowsAffected, _ := result.RowsAffected()
    if rowsAffected == 0 {
      return fmt.Errorf("task %s was modified concurrently (optimistic lock)", t.TaskID)
    }
    return nil
  })
}

func UpdateTaskAtomic(ctx context.Context, t *TeamTask, releaseWorker bool, workerID string) error {
  t.UpdatedAt = time.Now().Unix()
  depsJson, _ := json.Marshal(t.DependsOn)
  outputJson, _ := json.Marshal(t.OutputHistory)
  var nextRetry interface{}
  if t.NextRetryAt != 0 {
    nextRetry = t.NextRetryAt
  } else {
    nextRetry = nil
  }
  var completedAt interface{}
  if t.CompletedAt != 0 {
    completedAt = t.CompletedAt
  } else {
    completedAt = nil
  }
  return wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
    result := tx.Exec(`UPDATE team_tasks SET title=?, description=?, priority=?, status=?, assigned_member_id=?, assigned_worker_id=?, depends_on=?, result=?, error=?, output_history=?, progress=?, retry_count=?, max_retries=?, next_retry_at=?, updated_at=?, completed_at=? WHERE task_id=? AND updated_at=?`,
      t.Title, t.Description, t.Priority, t.Status, t.AssignedMemberID, t.AssignedWorkerID,
      string(depsJson), t.Result, t.Error, string(outputJson),
      t.Progress, t.RetryCount, t.MaxRetries, nextRetry, t.UpdatedAt, completedAt, t.TaskID, t.OldUpdatedAt)
    rowsAffected, _ := result.RowsAffected()
    if rowsAffected == 0 {
      return fmt.Errorf("task %s was modified concurrently (optimistic lock)", t.TaskID)
    }
    if releaseWorker && workerID != "" {
      tx.Exec(`UPDATE team_workers SET status=?, assigned_task_id='', updated_at=? WHERE worker_id=? AND status=?`,
        WorkerStatusIdle, t.UpdatedAt, workerID, WorkerStatusWorking)
    }
    return nil
  })
}

func DeleteTask(ctx context.Context, taskID string) error {
  return wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
    tx.Exec(`DELETE FROM team_tasks WHERE task_id = ?`, taskID)
    return nil
  })
}

func ListTasks(ctx context.Context, status, priority, memberID string) ([]*TeamTask, error) {
  db := wstore.GetGlobalDB()
  rows, err := db.Queryx(`SELECT task_id, title, description, priority, status, assigned_member_id, assigned_worker_id, depends_on, result, error, output_history, progress, retry_count, max_retries, next_retry_at, created_at, updated_at, completed_at FROM team_tasks ORDER BY created_at DESC`)
  if err != nil {
    return nil, err
  }
  defer rows.Close()
  var tasks []*TeamTask
  for rows.Next() {
    t, err := scanTask(rows)
    if err != nil {
      return nil, err
    }
    if status != "" && t.Status != status {
      continue
    }
    if priority != "" && t.Priority != priority {
      continue
    }
    if memberID != "" && t.AssignedMemberID != memberID {
      continue
    }
    tasks = append(tasks, t)
  }
  return tasks, rows.Err()
}

// --- Activity ---

func scanActivity(row interface{ Scan(dest ...any) error }) (*TeamActivity, error) {
  var a TeamActivity
  err := row.Scan(&a.Id, &a.TaskID, &a.WorkerID, &a.MemberID, &a.Type, &a.Description, &a.Meta, &a.CreatedAt)
  if err != nil {
    return nil, err
  }
  return &a, nil
}

func AddActivity(ctx context.Context, a *TeamActivity) error {
  a.CreatedAt = time.Now().Unix()
  return wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
    tx.Exec(`INSERT INTO team_activity (task_id, worker_id, member_id, type, description, meta, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
      a.TaskID, a.WorkerID, a.MemberID, a.Type, a.Description, a.Meta, a.CreatedAt)
    return nil
  })
}

func ListActivities(ctx context.Context, limit int, taskID, workerID, memberID string) ([]*TeamActivity, error) {
  if limit <= 0 {
    limit = 100
  }
  db := wstore.GetGlobalDB()
  query := `SELECT id, task_id, worker_id, member_id, type, description, meta, created_at FROM team_activity`
  var args []interface{}
  if taskID != "" || workerID != "" || memberID != "" {
    query += ` WHERE`
    if taskID != "" {
      query += ` task_id = ? AND`
      args = append(args, taskID)
    }
    if workerID != "" {
      query += ` worker_id = ? AND`
      args = append(args, workerID)
    }
    if memberID != "" {
      query += ` member_id = ? AND`
      args = append(args, memberID)
    }
    query = query[:len(query)-4]
  }
  query += ` ORDER BY created_at DESC LIMIT ?`
  args = append(args, limit)
  rows, err := db.Queryx(query, args...)
  if err != nil {
    return nil, err
  }
  defer rows.Close()
  var activities []*TeamActivity
  for rows.Next() {
    a, err := scanActivity(rows)
    if err != nil {
      return nil, err
    }
    activities = append(activities, a)
  }
  return activities, rows.Err()
}

func CleanupOldActivities(ctx context.Context, maxCount int) error {
  return wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
    tx.Exec(`DELETE FROM team_activity WHERE id NOT IN (SELECT id FROM team_activity ORDER BY created_at DESC LIMIT ?)`, maxCount)
    return nil
  })
}

// --- Status Aggregate ---

func GetStatus(ctx context.Context) (*TeamStatusData, error) {
  db := wstore.GetGlobalDB()
  status := &TeamStatusData{}

  var count sql.NullInt64
  err := db.Get(&count, `SELECT COUNT(*) FROM team_members`)
  if err != nil {
    return nil, fmt.Errorf("error counting members: %w", err)
  }
  status.TotalMembers = int(count.Int64)

  err = db.Get(&count, `SELECT COUNT(*) FROM team_workers WHERE status = ?`, WorkerStatusWorking)
  if err != nil {
    return nil, fmt.Errorf("error counting active workers: %w", err)
  }
  status.ActiveWorkers = int(count.Int64)

  err = db.Get(&count, `SELECT COUNT(*) FROM team_workers WHERE status = ?`, WorkerStatusIdle)
  if err != nil {
    return nil, fmt.Errorf("error counting idle workers: %w", err)
  }
  status.IdleWorkers = int(count.Int64)

  err = db.Get(&count, `SELECT COUNT(*) FROM team_workers WHERE status = ?`, WorkerStatusOffline)
  if err != nil {
    return nil, fmt.Errorf("error counting offline workers: %w", err)
  }
  status.OfflineWorkers = int(count.Int64)

  taskCounts := map[string]*int{
    TaskStatusPending: &status.PendingTasks,
    TaskStatusWorking: &status.WorkingTasks,
    TaskStatusDone:    &status.DoneTasks,
    TaskStatusFailed:  &status.FailedTasks,
    TaskStatusPaused:  &status.PausedTasks,
  }
  for taskStatus, ptr := range taskCounts {
    err = db.Get(&count, `SELECT COUNT(*) FROM team_tasks WHERE status = ?`, taskStatus)
    if err != nil {
      return nil, fmt.Errorf("error counting %s tasks: %w", taskStatus, err)
    }
    *ptr = int(count.Int64)
  }

  return status, nil
}

// --- WPS Event Publishing ---

func PublishTaskUpdate() {
  wps.Broker.Publish(wps.WaveEvent{Event: wps.Event_TeamTaskUpdate})
}

func PublishWorkerUpdate() {
  wps.Broker.Publish(wps.WaveEvent{Event: wps.Event_TeamWorkerUpdate})
}

func PublishMemberUpdate() {
	wps.Broker.Publish(wps.WaveEvent{Event: wps.Event_TeamMemberUpdate})
}

func PublishProjectUpdate() {
	wps.Broker.Publish(wps.WaveEvent{Event: wps.Event_TeamProjectUpdate})
}

// --- Project CRUD ---

func CreateProject(ctx context.Context, p *TeamProject) error {
	if p.ProjectID == "" {
		p.ProjectID = uuid.New().String()
	}
	now := time.Now().Unix()
	p.CreatedAt = now
	p.UpdatedAt = now
	return wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
		tx.Exec(`INSERT INTO team_projects (project_id, name, path, spec, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
			p.ProjectID, p.Name, p.Path, p.Spec, p.CreatedAt, p.UpdatedAt)
		return nil
	})
}

func scanProject(row interface{ Scan(dest ...any) error }) (*TeamProject, error) {
	var p TeamProject
	err := row.Scan(&p.ProjectID, &p.Name, &p.Path, &p.Spec, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func GetProject(ctx context.Context, projectID string) (*TeamProject, error) {
	db := wstore.GetGlobalDB()
	row := db.QueryRowx(`SELECT project_id, name, path, spec, created_at, updated_at FROM team_projects WHERE project_id = ?`, projectID)
	return scanProject(row)
}

func UpdateProject(ctx context.Context, p *TeamProject) error {
	p.UpdatedAt = time.Now().Unix()
	return wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
		tx.Exec(`UPDATE team_projects SET name=?, path=?, spec=?, updated_at=? WHERE project_id=?`,
			p.Name, p.Path, p.Spec, p.UpdatedAt, p.ProjectID)
		return nil
	})
}

func DeleteProject(ctx context.Context, projectID string) error {
	return wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
		tx.Exec(`DELETE FROM team_projects WHERE project_id = ?`, projectID)
		return nil
	})
}

func ListProjects(ctx context.Context) ([]*TeamProject, error) {
	db := wstore.GetGlobalDB()
	rows, err := db.Queryx(`SELECT project_id, name, path, spec, created_at, updated_at FROM team_projects ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var projects []*TeamProject
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

