// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	"testing"
	"time"

	"github.com/wavetermdev/waveterm/pkg/assistant"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
)

func TestSchedulerLifecycle(t *testing.T) {
	// Create scheduler with assistant (but won't call it due to no tasks in DB)
	scheduler := NewScheduler(assistant.NewAssistant(nil))

	// Test initial state
	if scheduler.IsRunning() {
		t.Error("scheduler should not be running initially")
	}

	// Test Start()
	err := scheduler.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Give the goroutine a moment to start (for test stability)
	time.Sleep(10 * time.Millisecond)

	if !scheduler.IsRunning() {
		t.Error("scheduler should be running after Start()")
	}

	// Test idempotent Start()
	err = scheduler.Start()
	if err != nil {
		t.Fatalf("second Start() should not fail: %v", err)
	}

	if !scheduler.IsRunning() {
		t.Error("scheduler should still be running after idempotent Start()")
	}

	// Test Stop()
	err = scheduler.Stop()
	if err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}

	if scheduler.IsRunning() {
		t.Error("scheduler should not be running after Stop()")
	}

	// Test idempotent Stop()
	err = scheduler.Stop()
	if err != nil {
		t.Fatalf("second Stop() should not fail: %v", err)
	}
}

func TestCalculateNextRun_Once(t *testing.T) {
	scheduler := NewScheduler(nil)

	now := time.Now().UnixMilli()
	task := &waveobj.ScheduledTask{
		OID:       "task-001",
		Pattern:   "once",
		NextRun:   now,
		ExecCount: 0,
	}

	nextRun, status, execCount := scheduler.calculateNextRun(task)

	if status != "completed" {
		t.Errorf("expected status 'completed', got: %s", status)
	}

	if nextRun != 0 {
		t.Errorf("expected nextRun 0 for 'once' pattern, got: %d", nextRun)
	}

	if execCount != 1 {
		t.Errorf("expected execCount 1, got: %d", execCount)
	}
}

func TestCalculateNextRun_Daily(t *testing.T) {
	scheduler := NewScheduler(nil)

	now := time.Now().UnixMilli()
	task := &waveobj.ScheduledTask{
		OID:       "task-002",
		Pattern:   "daily",
		NextRun:   now,
		ExecCount: 0,
	}

	nextRun, status, execCount := scheduler.calculateNextRun(task)

	expectedNextRun := now + (24 * time.Hour).Milliseconds()
	if nextRun != expectedNextRun {
		t.Errorf("expected nextRun %d, got: %d", expectedNextRun, nextRun)
	}

	if status != "pending" {
		t.Errorf("expected status 'pending', got: %s", status)
	}

	if execCount != 1 {
		t.Errorf("expected execCount 1, got: %d", execCount)
	}
}

func TestCalculateNextRun_Hourly(t *testing.T) {
	scheduler := NewScheduler(nil)

	now := time.Now().UnixMilli()
	task := &waveobj.ScheduledTask{
		OID:       "task-003",
		Pattern:   "hourly",
		NextRun:   now,
		ExecCount: 0,
	}

	nextRun, status, execCount := scheduler.calculateNextRun(task)

	expectedNextRun := now + (1 * time.Hour).Milliseconds()
	if nextRun != expectedNextRun {
		t.Errorf("expected nextRun %d, got: %d", expectedNextRun, nextRun)
	}

	if status != "pending" {
		t.Errorf("expected status 'pending', got: %s", status)
	}

	if execCount != 1 {
		t.Errorf("expected execCount 1, got: %d", execCount)
	}
}

func TestCalculateNextRun_Weekly(t *testing.T) {
	scheduler := NewScheduler(nil)

	now := time.Now().UnixMilli()
	task := &waveobj.ScheduledTask{
		OID:       "task-004",
		Pattern:   "weekly",
		NextRun:   now,
		ExecCount: 0,
	}

	nextRun, status, execCount := scheduler.calculateNextRun(task)

	expectedNextRun := now + (168 * time.Hour).Milliseconds() // 7 days
	if nextRun != expectedNextRun {
		t.Errorf("expected nextRun %d, got: %d", expectedNextRun, nextRun)
	}

	if status != "pending" {
		t.Errorf("expected status 'pending', got: %s", status)
	}

	if execCount != 1 {
		t.Errorf("expected execCount 1, got: %d", execCount)
	}
}

func TestCalculateNextRun_Repeat_UnderLimit(t *testing.T) {
	scheduler := NewScheduler(nil)

	now := time.Now().UnixMilli()
	task := &waveobj.ScheduledTask{
		OID:       "task-005",
		Pattern:   "repeat",
		NextRun:   now,
		ExecCount: 0,
		MaxExecs:  5,
	}

	nextRun, status, execCount := scheduler.calculateNextRun(task)

	expectedNextRun := now + (1 * time.Hour).Milliseconds()
	if nextRun != expectedNextRun {
		t.Errorf("expected nextRun %d, got: %d", expectedNextRun, nextRun)
	}

	if status != "pending" {
		t.Errorf("expected status 'pending', got: %s", status)
	}

	if execCount != 1 {
		t.Errorf("expected execCount 1, got: %d", execCount)
	}
}

func TestCalculateNextRun_Repeat_AtLimit(t *testing.T) {
	scheduler := NewScheduler(nil)

	now := time.Now().UnixMilli()
	task := &waveobj.ScheduledTask{
		OID:       "task-006",
		Pattern:   "repeat",
		NextRun:   now,
		ExecCount: 4, // Already executed 4 times, max is 5
		MaxExecs:  5,
	}

	nextRun, status, execCount := scheduler.calculateNextRun(task)

	if status != "completed" {
		t.Errorf("expected status 'completed' when max_execs reached, got: %s", status)
	}

	if nextRun != 0 {
		t.Errorf("expected nextRun 0 for completed 'repeat' task, got: %d", nextRun)
	}

	if execCount != 5 {
		t.Errorf("expected execCount 5 (at limit), got: %d", execCount)
	}
}

func TestCalculateNextRun_Repeat_OverLimit(t *testing.T) {
	scheduler := NewScheduler(nil)

	now := time.Now().UnixMilli()
	task := &waveobj.ScheduledTask{
		OID:       "task-007",
		Pattern:   "repeat",
		NextRun:   now,
		ExecCount: 5, // Already exceeded max_execs of 5
		MaxExecs:  5,
	}

	nextRun, status, execCount := scheduler.calculateNextRun(task)

	if status != "completed" {
		t.Errorf("expected status 'completed' when over max_execs, got: %s", status)
	}

	if nextRun != 0 {
		t.Errorf("expected nextRun 0 for completed 'repeat' task, got: %d", nextRun)
	}

	if execCount != 6 {
		t.Errorf("expected execCount 6, got: %d", execCount)
	}
}

func TestCalculateNextRun_Repeat_Unlimited(t *testing.T) {
	scheduler := NewScheduler(nil)

	now := time.Now().UnixMilli()
	task := &waveobj.ScheduledTask{
		OID:       "task-008",
		Pattern:   "repeat",
		NextRun:   now,
		ExecCount: 10,
		MaxExecs:  0, // 0 means unlimited
	}

	nextRun, status, execCount := scheduler.calculateNextRun(task)

	expectedNextRun := now + (1 * time.Hour).Milliseconds()
	if nextRun != expectedNextRun {
		t.Errorf("expected nextRun %d, got: %d", expectedNextRun, nextRun)
	}

	if status != "pending" {
		t.Errorf("expected status 'pending' for unlimited 'repeat', got: %s", status)
	}

	if execCount != 11 {
		t.Errorf("expected execCount 11, got: %d", execCount)
	}
}

func TestCalculateNextRun_UnknownPattern(t *testing.T) {
	scheduler := NewScheduler(nil)

	now := time.Now().UnixMilli()
	task := &waveobj.ScheduledTask{
		OID:       "task-009",
		Pattern:   "unknown-pattern",
		NextRun:   now,
		ExecCount: 0,
	}

	nextRun, status, execCount := scheduler.calculateNextRun(task)

	if status != "completed" {
		t.Errorf("expected status 'completed' for unknown pattern, got: %s", status)
	}

	if nextRun != 0 {
		t.Errorf("expected nextRun 0 for unknown pattern, got: %d", nextRun)
	}

	if execCount != 1 {
		t.Errorf("expected execCount 1, got: %d", execCount)
	}
}

func TestCalculateNextRun_MultipleExecutions(t *testing.T) {
	scheduler := NewScheduler(nil)

	now := time.Now().UnixMilli()
	task := &waveobj.ScheduledTask{
		OID:       "task-010",
		Pattern:   "hourly",
		NextRun:   now,
		ExecCount: 5,
	}

	nextRun, status, execCount := scheduler.calculateNextRun(task)

	expectedNextRun := now + (1 * time.Hour).Milliseconds()
	if nextRun != expectedNextRun {
		t.Errorf("expected nextRun %d, got: %d", expectedNextRun, nextRun)
	}

	if status != "pending" {
		t.Errorf("expected status 'pending', got: %s", status)
	}

	if execCount != 6 {
		t.Errorf("expected execCount 6, got: %d", execCount)
	}
}

// Note: Integration tests requiring database access are in a separate test file (scheduler_integration_test.go)
// to allow fast unit testing without database dependencies.
