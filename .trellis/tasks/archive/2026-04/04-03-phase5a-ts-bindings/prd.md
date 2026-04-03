# Phase 5a: task generate + TypeScript Bindings Refactor

## Overview

All ZeroAI RPC types (Phase 1-4, 14 commands total) are defined in `WshRpcInterface` but have **never been run through `task generate`**. The frontend currently bypasses the type system using manual `wshRpcCall()` with string command names. This task generates proper TypeScript bindings and refactors the frontend to use them.

## Current Problems

1. **No generated bindings**: `frontend/types/gotypes.d.ts` contains ZERO ZeroAI types
2. **No generated RPC methods**: `frontend/app/store/wshclientapi.ts` contains ZERO ZeroAI methods
3. **Manual RPC calls**: `zeroai-client.ts` uses `wshRpcCall("zeroaicreatesession", ...)` instead of `RpcApi.ZeroAiCreateSessionCommand(...)`
4. **Duplicate type definitions**: `frontend/app/zeroai/types/index.ts` (155 lines) manually defines types that should come from gotypes.d.ts
5. **Every method has TODO comments**: `// Note: After running task generate, use: RpcApi.xxx(...)`

## Requirements

- [ ] Run `task generate` to generate TypeScript bindings for all ZeroAI types
- [ ] Verify all 14 ZeroAI commands appear in generated `wshclientapi.ts`
- [ ] Verify all ZeroAI types appear in generated `gotypes.d.ts`
- [ ] Refactor `frontend/app/zeroai/store/zeroai-client.ts` to use generated `RpcApi` methods
- [ ] Update `frontend/app/zeroai/types/index.ts` to re-export from gotypes.d.ts
- [ ] Fix all TypeScript type errors (`task check:ts`)
- [ ] Ensure all ZeroAI components still compile correctly

## Acceptance Criteria

- [ ] `grep "ZeroAiProviderInfo" frontend/types/gotypes.d.ts` returns matches
- [ ] `grep "ZeroAiCreateSessionCommand\|ZeroAiListProvidersCommand" frontend/app/store/wshclientapi.ts` returns matches
- [ ] `zeroai-client.ts` has ZERO `wshRpcCall("zeroai...")` calls remaining
- [ ] `zeroai-client.ts` has ZERO `wshRpcStream("zeroai...")` calls remaining
- [ ] `task check:ts` passes with no errors

## Technical Notes

### snake_case vs camelCase

The Go JSON tags use snake_case (e.g., `session_id`, `cli_command`), but the hand-written frontend types use camelCase (e.g., `sessionId`, `cliCommand`). Check how the existing WaveAI code handles this:

- Look at how `WaveAiData` types are used in `frontend/app/aipanel/`
- The generator may produce types with snake_case fields
- Decide: either use snake_case throughout (consistent with backend) or create camelCase wrappers

### Generated RPC Method Pattern

After `task generate`, the pattern will be:

```typescript
import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";

// Old (manual):
await client.wshRpcCall("zeroaicreatesession", { backend: "claude", ... });

// New (generated):
await RpcApi.ZeroAiCreateSessionCommand(TabRpcClient, { backend: "claude", ... });
```

### Streaming Pattern

```typescript
// Old (manual):
const stream = client.wshRpcStream("zeroaisendstreammessage", { ... });

// New (generated):
const stream = RpcApi.ZeroAiSendStreamMessageCommand(TabRpcClient, { ... });
```

### Type Re-export Strategy

Instead of duplicating types, re-export from gotypes.d.ts:

```typescript
// frontend/app/zeroai/types/index.ts
export type { ZeroAiSessionInfo, ZeroAiMessageInfo, ZeroAiProviderInfo, ... } from "@/types/gotypes";
// Keep only ZeroAI-specific types that don't exist in gotypes.d.ts
```

## Out of Scope

- Modifying Go backend code (all types and handlers already exist)
- Adding new RPC commands
- UI changes to components
- Writing tests
