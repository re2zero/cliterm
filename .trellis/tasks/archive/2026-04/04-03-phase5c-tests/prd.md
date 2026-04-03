# Task: Phase 5c - ZeroAI Integration Tests

## Overview

Add comprehensive integration tests for ZeroAI backend Go packages to increase test coverage and ensure correctness of core service and store operations.

## Requirements

### P0 - Must Have (Service Layer Tests)

Create the following test files with meaningful test cases:

**1. `pkg/zeroai/service/session-service_test.go`**

- Test `CreateSession` — creates session with correct fields, returns session ID
- Test `GetSession` — retrieves session by ID, returns error for non-existent
- Test `ListSessions` — returns all sessions, empty list when none exist
- Test `DeleteSession` — removes session, returns error for non-existent
- Use a mock/fake store implementation (interface from `store-interface.go`)

**2. `pkg/zeroai/service/message-service_test.go`**

- Test `AddMessage` — adds message with correct fields (role, content, session ID)
- Test `GetMessages` — retrieves messages for a session, empty list when none
- Test `DeleteMessage` — removes message, returns error for non-existent
- Use a mock/fake store implementation

**3. `pkg/zeroai/service/provider-service_test.go`**

- Test `GetProviders` — returns list of configured providers
- Test `SetProvider` — saves/upserts a provider configuration
- Test `DeleteProvider` — removes a provider, returns error for non-existent
- Use a mock/fake store or config source

### P1 - Should Have (Store Layer Tests)

**4. `pkg/zeroai/store/session-store_test.go`**

- Test `CreateSession` — persists to SQLite, retrieves correctly
- Test `GetSession` — round-trip from create to get
- Test `ListSessions` — lists all after creating multiple
- Test `DeleteSession` — removes and verifies with GetSession
- Use in-memory SQLite (`driver: "sqlite"`, `dsn: ":memory:"`)
- Follow the pattern from `team-store_test.go` (setupTestDB helper)

**5. `pkg/zeroai/store/message-store_test.go`**

- Test `AddMessage` — persists message, verify fields
- Test `GetMessages` — retrieves by session ID
- Test `DeleteMessage` — removes and verifies
- Use in-memory SQLite

### P2 - Nice to Have (Protocol Tests)

**6. `pkg/zeroai/protocol/acp-adapter_test.go`**

- Test ACP message conversion functions
- Test protocol type mappings

## Test Patterns to Follow

### Gold Standard: `team-store_test.go`

- Uses `testify/assert` and `testify/require`
- In-memory SQLite with `setupTestDB()` helper
- Table-driven tests where appropriate
- Clean test isolation (each test gets fresh DB)

### Service Test Pattern: `agent-service_test.go`

- Mock implementations of store interfaces
- Direct instantiation of service with mock store
- Focus on business logic, not persistence

## Technical Notes

- `go.mod` already includes `github.com/stretchr/testify`
- SQLite driver: use `modernc.org/sqlite` (pure Go, already in go.mod)
- Copyright headers required: `// Copyright 2026, Command Line Inc.` and `// SPDX-License-Identifier: Apache-2.0`
- Tests must pass: `go test ./pkg/zeroai/...`
- NO stub/placeholder tests — each test must be meaningful and verifiable

## Acceptance Criteria

- [ ] All P0 test files created with at least 4 test functions each
- [ ] All P1 test files created with at least 3 test functions each
- [ ] `go test ./pkg/zeroai/...` passes with no failures
- [ ] All tests use proper assertions (testify), not just `t.Fatal`
- [ ] Copyright headers on all new files
- [ ] No test helper left unused

## Out of Scope

- Frontend/Vitest tests (separate task)
- End-to-end/integration tests requiring running processes
- Agent service tests (already exist in `agent-service_test.go`)
- Process manager tests (already exist in `process_manager_test.go`)
- Protocol tests beyond basic message conversion (P2 only)
