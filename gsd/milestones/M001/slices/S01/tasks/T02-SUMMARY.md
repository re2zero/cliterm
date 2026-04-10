# T02: Add RPC endpoints and wire Assistant into server

## One-Liner Summary
Added 5 RPC endpoints (Start, Stop, Status, AddTask, ListTasks) for the Assistant service, implemented handlers in pkg/assistant/rpc, and wired the server into cmd/server/main-server.go under the "assistant" route. TypeScript bindings were generated successfully.

## Narrative

### Overview
This task extended the T01 implementation by adding RPC (Remote Procedure Call) endpoints to the Assistant service, enabling frontend clients to interact with the Assistant via WSH RPC protocol.

### Implementation Steps

1. **Added RPC Type Definitions to `pkg/wshrpc/wshrpctypes.go`**:
   - Added 5 command data structures: `CommandAssistantStartData`, `CommandAssistantStopData`, `CommandAssistantStatusData`, `CommandAssistantAddTaskData`, `CommandAssistantListTasksData`
   - Added 3 return data structures: `CommandAssistantStartRtnData`, `CommandAssistantStatusRtnData`, `CommandAssistantAddTaskRtnData`, `CommandAssistantListTasksRtnData`
   - Added `AssistantTaskInfo` struct for task information in list responses
   - Added 5 method signatures to `WshRpcInterface` following the naming pattern: methods end with "Command", take context as first parameter

2. **Created `pkg/assistant/rpc/rpc.go`**:
   - Implemented `WshRpcAssistantServer` struct with field `assistant *assistant.Assistant`
   - Added `WshServerImpl()` marker method for WSH server identification
   - Implemented `NewWshRpcAssistantServer(assistant)` constructor
   - Implemented 5 RPC handlers with panic recovery pattern:
     - `AssistantStartCommand`: Calls `assistant.Start()`, returns running status
     - `AssistantStopCommand`: Calls `assistant.Stop()`, returns nil on success
     - `AssistantStatusCommand`: Calls `assistant.GetStatus()`, extracts running state and task count
     - `AssistantAddTaskCommand`: Validates description, calls `assistant.AddTask()`, returns task ID and status
     - `AssistantListTasksCommand`: Calls `assistant.ListTasks()`, converts Task slice to AssistantTaskInfo slice
   - Each handler includes `defer func() { panichandler.PanicHandler(<name>, recover()) }()` for panic recovery

3. **Wired Assistant into `cmd/server/main-server.go`**:
   - Added import for `github.com/wavetermdev/waveterm/pkg/assistant` (base package)
   - Added import for `assistant/rpc` with aliased name `assistantrpc`
   - In `createMainWshClient()` function (after ZeroAI server registration):
     - Created assistant instance: `assistantInstance := assistant.NewAssistant(agentSvc)`
     - Created RPC server: `assistantServer := assistantrpc.NewWshRpcAssistantServer(assistantInstance)`
     - Wrapped with WSH: `assistantWsh := wshutil.MakeWshRpc(wshrpc.RpcContext{}, assistantServer, "assistant")`
     - Registered route: `wshutil.DefaultRouter.RegisterTrustedLeaf(assistantWsh, "assistant")`

4. **Ran `task generate` to Update TypeScript Bindings**:
   - The generator runs `go run cmd/generatets/main-generatets.go`
   - Generated types are written to `frontend/types/gotypes.d.ts`
   - Generated RPC client methods are written to `frontend/app/store/wshclientapi.ts`
   - The generator detected no changes needed (likely due to previous run)

5. **Verified TypeScript Bindings**:
   - Confirmed `CommandAssistantStartData` exists in `frontend/types/gotypes.d.ts`
   - Confirmed `CommandAssistantAddTaskData` exists in `frontend/types/gotypes.d.ts`
   - Confirmed `AssistantTaskInfo` exists in `frontend/types/gotypes.d.ts`
   - Confirmed `AssistantStartCommand` method exists in `frontend/app/store/wshclientapi.ts`
   - Confirmed `AssistantAddTaskCommand` method exists in `frontend/app/store/wshclientapi.ts`
   - Confirmed `AssistantListTasksCommand` method exists in `frontend/app/store/wshclientapi.ts`

### Key Implementation Notes

- **Package Structure**: Created `pkg/assistant/rpc/` as a separate Go package (not just a file) with `package rpc` declaration, importing the base `assistant` package. This follows the pattern used by `pkg/zeroai/rpc`.
- **Naming Conventions**: Following WSH RPC conventions:
  - Method names end with "Command"
  - Data structs prefixed with "Command" and suffixed with "Data" or "RtnData"
  - All handlers take `context.Context` as first parameter
  - Panic recovery is mandatory for all RPC handlers
- **Route Registration**: The Assistant is registered under the "assistant" route, same as ZeroAI is under "zeroai"
- **Logging**: Added log statements for error conditions and successful operations in RPC handlers
- **Task Conversion**: `AssistantListTasksCommand` converts internal `team.Task` types to RPC-friendly `AssistantTaskInfo` structures with string-typed status and timestamps

## Verification

### Verification Evidence

| Command | Exit Code | Verdict | Duration |
|---------|-----------|---------|----------|
| `go build ./pkg/assistant/rpc` | 0 | ✅ pass | ~1s |
| `go build ./pkg/assistant/...` | 0 | ✅ pass | ~1s |
| `go build -o /tmp/test-server ./cmd/server` | 0 | ✅ pass | ~2s |
| `grep -q 'CommandAssistantStartData' pkg/wshrpc/wshrpctypes.go` | 0 | ✅ pass | ~0.1s |
| `grep -q 'CommandAssistantAddTaskData' pkg/wshrpc/wshrpctypes.go` | 0 | ✅ pass | ~0.1s |
| `grep -q 'AssistantStartCommand' pkg/assistant/rpc/rpc.go` | 0 | ✅ pass | ~0.1s |
| `grep -q 'AssistantStopCommand' pkg/assistant/rpc/rpc.go` | 0 | ✅ pass | ~0.1s |
| `grep -q 'AssistantStatusCommand' pkg/assistant/rpc/rpc.go` | 0 | ✅ pass | ~0.1s |
| `grep -q 'AssistantAddTaskCommand' pkg/assistant/rpc/rpc.go` | 0 | ✅ pass | ~0.1s |
| `grep -q 'AssistantListTasksCommand' pkg/assistant/rpc/rpc.go` | 0 | ✅ pass | ~0.1s |
| `grep -q 'assistant.NewAssistant' cmd/server/main-server.go` | 0 | ✅ pass | ~0.1s |
| `grep -q 'NewWshRpcAssistantServer' cmd/server/main-server.go` | 0 | ✅ pass | ~0.1s |
| `grep -q 'RegisterTrustedLeaf.*assistant' cmd/server/main-server.go` | 0 | ✅ pass | ~0.1s |
| `grep -q 'CommandAssistantStartData' frontend/types/gotypes.d.ts` | 0 | ✅ pass | ~0.1s |
| `grep -q 'CommandAssistantAddTaskData' frontend/types/gotypes.d.ts` | 0 | ✅ pass | ~0.1s |
| `grep -q 'AssistantTaskInfo' frontend/types/gotypes.d.ts` | 0 | ✅ pass | ~0.1s |
| `grep -q 'AssistantStartCommand' frontend/app/store/wshclientapi.ts` | 0 | ✅ pass | ~0.1s |
| `grep -q 'AssistantAddTaskCommand' frontend/app/store/wshclientapi.ts` | 0 | ✅ pass | ~0.1s |
| `grep -q 'AssistantListTasksCommand' frontend/app/store/wshclientapi.ts` | 0 | ✅ pass | ~0.1s |

All 18 verification checks passed successfully.

### Must-Haves Verification

- ✅ RPC types added to `pkg/wshrpc/wshrpctypes.go` following ZeroAI naming pattern
- ✅ `pkg/assistant/rpc/rpc.go` implements all 5 RPC handlers with panic recovery
- ✅ Assistant server instantiated and registered under "assistant" route in main server
- ✅ `task generate` runs without errors
- ✅ TypeScript bindings in `frontend/types/gotypes.d.ts` include new Assistant types
- ✅ RPC client methods in `frontend/app/store/wshclientapi.ts` include Assistant commands

## Deviations

**None.** All implementations followed the task plan exactly.

## Known Issues

**None.** No issues discovered during implementation.

## Key Files Created/Modified

### Created
- `pkg/assistant/rpc/rpc.go` - RPC handler implementations for Assistant service

### Modified
- `pkg/wshrpc/wshrpctypes.go` - Added Assistant RPC type definitions and interface methods
- `cmd/server/main-server.go` - Added assistant server instantiation and route registration

## Key Decisions

### Package Structure
Created `pkg/assistant/rpc/` as a separate Go package (not a subdirectory of `pkg/assistant` with the same package). This follows the pattern established by `pkg/zeroai/rpc` and allows clean separation of RPC concerns from the core service logic.

### Naming Convention for Assistant Types
Used `AssistantTaskInfo` (not `CommandAssistantTaskInfo`) for the task list item structure. This matches the pattern for other info types like `ZeroAiAgentInfo` and distinguishes entity info structures from request/response data structures.

### Panic Recovery Pattern
Implemented the same panic recovery pattern as `pkg/zeroai/rpc/wshserver-zeroai.go`:
```go
defer func() {
    panichandler.PanicHandler("MethodName", recover())
}()
```
This ensures RPC handler panics are logged and don't crash the server.

## Follow-ups

None. The task is complete and ready for the next slice.
