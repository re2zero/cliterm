// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package waveobj

import (
	"reflect"
	"testing"
)

// init registers the ScheduledTask type for testing
func init() {
	RegisterType(reflect.TypeOf(&ScheduledTask{}))
}

func TestScheduledTaskTypeRegistered(t *testing.T) {
	// Verify OType_ScheduledTask constant is defined
	if OType_ScheduledTask != "scheduledtask" {
		t.Errorf("OType_ScheduledTask should be 'scheduledtask', got '%s'", OType_ScheduledTask)
	}

	// Verify OType_ScheduledTask is in ValidOTypes
	if !ValidOTypes[OType_ScheduledTask] {
		t.Errorf("OType_ScheduledTask should be in ValidOTypes map")
	}
}

func TestScheduledTaskInAllWaveObjTypes(t *testing.T) {
	allTypes := AllWaveObjTypes()
	scheduledTaskType := reflect.TypeOf(&ScheduledTask{})

	found := false
	for _, typ := range allTypes {
		if typ == scheduledTaskType {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("ScheduledTask type not found in AllWaveObjTypes()")
	}
}

func TestScheduledTaskGetOType(t *testing.T) {
	task := &ScheduledTask{}
	if task.GetOType() != OType_ScheduledTask {
		t.Errorf("Expected GetOType() to return OType_ScheduledTask, got '%s'", task.GetOType())
	}
}

func TestScheduledTaskJSONSerialization(t *testing.T) {
	task := &ScheduledTask{
		OID:       "test-oid-123",
		Version:   1,
		TaskYAML:  "version: 1\ntask: test",
		NextRun:   1234567890000,
		Status:    "pending",
		Pattern:   "daily",
		ExecCount: 0,
		MaxExecs:  10,
		LastRun:   0,
		Meta:      MetaMapType{"key": "value"},
	}

	// Test serialization to JSON
	data, err := ToJsonMap(task)
	if err != nil {
		t.Fatalf("ToJsonMap failed: %v", err)
	}

	// Verify key fields are present
	// Note: ToJsonMap sets oid, otype, version via GetOID/GetOType/GetVersion
	if data["oid"] != "test-oid-123" {
		t.Errorf("Expected oid 'test-oid-123', got '%v'", data["oid"])
	}
	if data["otype"] != "scheduledtask" {
		t.Errorf("Expected otype 'scheduledtask', got '%v'", data["otype"])
	}
	if data["version"] != 1 {
		t.Errorf("Expected version 1, got '%v' (type %T)", data["version"], data["version"])
	}
	if data["taskyaml"] != "version: 1\ntask: test" {
		t.Errorf("Expected taskyaml, got '%v'", data["taskyaml"])
	}
	// Numeric fields are preserved as their Go types (int64, int)
	if data["nextrun"] != int64(1234567890000) {
		t.Errorf("Expected nextrun 1234567890000, got '%v' (type %T)", data["nextrun"], data["nextrun"])
	}
	if data["status"] != "pending" {
		t.Errorf("Expected status 'pending', got '%v'", data["status"])
	}
	if data["pattern"] != "daily" {
		t.Errorf("Expected pattern 'daily', got '%v'", data["pattern"])
	}
	if data["execcount"] != 0 {
		t.Errorf("Expected execcount 0, got '%v' (type %T)", data["execcount"], data["execcount"])
	}
	if data["maxexecs"] != 10 {
		t.Errorf("Expected maxexecs 10, got '%v' (type %T)", data["maxexecs"], data["maxexecs"])
	}
	if data["lastrun"] != int64(0) {
		t.Errorf("Expected lastrun 0, got '%v' (type %T)", data["lastrun"], data["lastrun"])
	}
}

func TestScheduledTaskJSONDeserialization(t *testing.T) {
	objMap := map[string]any{
		"oid":       "test-oid-456",
		"otype":     "scheduledtask",
		"version":   2,
		"taskyaml":  "name: mytask",
		"nextrun":   9876543210000,
		"status":    "scheduled",
		"pattern":   "hourly",
		"execcount": 5,
		"maxexecs":  0,
		"lastrun":   1234567890000,
		"meta":      map[string]any{"custom": "data"},
	}

	// Test deserialization from JSON map
	obj, err := FromJsonMap(objMap)
	if err != nil {
		t.Fatalf("FromJsonMap failed: %v", err)
	}

	task, ok := obj.(*ScheduledTask)
	if !ok {
		t.Fatalf("Expected *ScheduledTask, got %T", obj)
	}

	// Verify all fields
	if task.OID != "test-oid-456" {
		t.Errorf("Expected oid 'test-oid-456', got '%s'", task.OID)
	}
	if task.Version != 2 {
		t.Errorf("Expected version 2, got %d", task.Version)
	}
	if task.TaskYAML != "name: mytask" {
		t.Errorf("Expected taskyaml 'name: mytask', got '%s'", task.TaskYAML)
	}
	if task.NextRun != 9876543210000 {
		t.Errorf("Expected nextrun 9876543210000, got %d", task.NextRun)
	}
	if task.Status != "scheduled" {
		t.Errorf("Expected status 'scheduled', got '%s'", task.Status)
	}
	if task.Pattern != "hourly" {
		t.Errorf("Expected pattern 'hourly', got '%s'", task.Pattern)
	}
	if task.ExecCount != 5 {
		t.Errorf("Expected execcount 5, got %d", task.ExecCount)
	}
	if task.MaxExecs != 0 {
		t.Errorf("Expected maxexecs 0, got %d", task.MaxExecs)
	}
	if task.LastRun != 1234567890000 {
		t.Errorf("Expected lastrun 1234567890000, got %d", task.LastRun)
	}
	if task.Meta["custom"] != "data" {
		t.Errorf("Expected meta[custom] 'data', got '%v'", task.Meta["custom"])
	}
}

func TestScheduledTaskMinimalFields(t *testing.T) {
	// Test with minimal required fields
	objMap := map[string]any{
		"oid":     "minimal-oid",
		"otype":   "scheduledtask",
		"version": 1,
	}

	obj, err := FromJsonMap(objMap)
	if err != nil {
		t.Fatalf("FromJsonMap with minimal fields failed: %v", err)
	}

	task, ok := obj.(*ScheduledTask)
	if !ok {
		t.Fatalf("Expected *ScheduledTask, got %T", obj)
	}

	// Verify default values
	if task.OID != "minimal-oid" {
		t.Errorf("Expected oid 'minimal-oid', got '%s'", task.OID)
	}
	if task.TaskYAML != "" {
		t.Errorf("Expected empty taskyaml, got '%s'", task.TaskYAML)
	}
	if task.Status != "" {
		t.Errorf("Expected empty status, got '%s'", task.Status)
	}
	if task.Pattern != "" {
		t.Errorf("Expected empty pattern, got '%s'", task.Pattern)
	}
	if task.ExecCount != 0 {
		t.Errorf("Expected execcount 0, got %d", task.ExecCount)
	}
}

func TestScheduledTaskPatterns(t *testing.T) {
	// Test common pattern values
	patterns := []string{"once", "daily", "hourly", "weekly", "repeat 5"}

	for _, pattern := range patterns {
		task := &ScheduledTask{
			OID:     "pattern-test-" + pattern,
			Version: 1,
			Pattern: pattern,
		}

		data, err := ToJsonMap(task)
		if err != nil {
			t.Errorf("ToJsonMap failed for pattern '%s': %v", pattern, err)
			continue
		}

		if data["pattern"] != pattern {
			t.Errorf("Expected pattern '%s', got '%v'", pattern, data["pattern"])
		}
	}
}
