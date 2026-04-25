# Wave Terminal — AGENTS.md

## Build System

Uses [Task](https://taskfile.dev) (not Make/npm scripts) for all build orchestration. Config: `Taskfile.yml`.

| Command | What it does |
|---|---|
| `task init` | First-time setup: npm install + go mod tidy + docs npm install |
| `task dev` | Electron dev server with HMR (builds backend + wsh first) |
| `task start` | Electron standalone (no HMR) |
| `task quickdev` | macOS arm64 only: skip generate, skip wsh, fastest iteration |
| `task preview` | Standalone React preview at http://localhost:7007 — **no Electron, no Go backend** |
| `task build:preview` | Static build of preview server |
| `task package` | Production build + electron-builder package → `make/` |
| `task generate` | Go→TS codegen (after changing RPC types, config types, or wtypemeta) |
| `task check:ts` | Typecheck all TypeScript (`npx tsc --noEmit`) |
| `task test` | Run frontend tests (Vitest) |

**Do NOT use** `npm run dev` or `npm run start` directly — they launch Electron. Use `task dev` / `task start`.

### Backend builds

| Command | What |
|---|---|
| `task build:backend` | Build wavesrv + wsh (all targets) |
| `task build:server` | Build wavesrv only (all archs) |
| `task build:wsh` | Build wsh only (all os/arch combos) |

Linux builds use `zig` as cross-compiler for CGO static linking. Go 1.25+ required. Node 22 LTS required.

## Architecture

**Electron app** = React 19 frontend + Go backend (`wavesrv`) communicating over WebSocket RPC.

```
emain/           Electron main process (window mgmt, native APIs, IPC)
frontend/        React renderer (views, blocks, state)
  app/view/      View types (term, preview, web, waveai, etc.)
  app/block/     Block system (frame, registry, split/layout)
  app/store/     Jotai state management, RPC client API
cmd/server/      Go backend entrypoint (wavesrv)
cmd/wsh/         Go CLI helper (wsh) — cross-compiled for many targets
pkg/             Go packages (wshrpc, wcore, wstore, waveai, etc.)
tsunami/         Separate Go module — in-app component builder (has own go.mod)
db/              SQLite migrations (wstore + filestore)
schema/          JSON schemas for config validation
```

### Tsunami is a separate Go module

`tsunami/go.mod` is independent (`github.com/wavetermdev/waveterm/tsunami`). It has its own frontend (`tsunami/frontend/`). Do not import root pkg/ from tsunami or vice versa.

### RPC System (wshrpc)

All frontend↔backend communication goes through `pkg/wshrpc/wshrpctypes.go`. This is the **source of truth** for all RPC commands.

- Define command → `pkg/wshrpc/wshrpctypes.go`
- Implement server handler → `pkg/wshrpc/wshserver/wshserver.go`
- Run `task generate` → produces `frontend/types/gotypes.d.ts` and `frontend/app/store/wshclientapi.ts`
- **Never manually edit generated TS files.**

## View System

Views are registered in `frontend/app/block/blockregistry.ts` as a `Map<string, ViewModelClass>`. Each view implements `ViewModel` (defined in `frontend/types/custom.d.ts`).

Creating a new view = 3 files:
1. `frontend/app/view/<name>/<name>-model.ts` — ViewModel class (Jotai atoms, no React hooks)
2. `frontend/app/view/<name>/<name>.tsx` — React component
3. Register in `blockregistry.ts`: `BlockRegistry.set("<name>", MyViewModel)`

See `.kilocode/skills/create-view/SKILL.md` for the full pattern.

## State Management (Jotai)

Models use the **singleton pattern**: `private static instance`, `private constructor`, `static getInstance()`.

- Simple atoms → field initializers
- Derived atoms → created in constructor (depend on other atoms via `get`)
- Models never use React hooks — use `globalStore.get/set`
- Components use `useAtomValue` / `useAtom` (these are hooks, must be at top level)

## Coding Conventions

### Go
- String constants for enums (not custom types): `const StatusRunning = "running"`
- Use `Make` not `New` for struct constructors
- `Printf` not `Println`
- `lock.Lock(); defer lock.Unlock()` — always defer, create helper functions
- **NEVER run `go build`** — trust VSCode/lsp errors instead
- Run tests from project root: `go test ./pkg/wcore/...` not `cd pkg/wcore && go test`

### TypeScript
- `@/...` imports (maps to `frontend/*` via tsconfig)
- Named exports only — no default exports
- 4-space indent
- All lowercase filenames (except `Taskfile.yml`)
- Strict null checks OFF — don't add `| null` everywhere
- Never `=== undefined` / `!== undefined` — use `== null` / `!= null`
- Never `atob()`/`btoa()` — use `frontend/util/util.ts` base64 helpers
- No private fields (`#field`) — they're impossible to inspect
- `React.RefObject` not `React.MutableRefObject` (React 19)

### Styling
- Tailwind v4 for new code, migrating away from SCSS
- Never `cursor-help` or `cursor-not-allowed`
- Accent buttons: `bg-accent/80 text-primary rounded hover:bg-accent transition-colors cursor-pointer`
- Import `cn` from `@/util/util` for classname merging

### Comments
- Only comments that explain **WHY** (not WHAT)
- Never add comments describing what code does
- Never remove existing comments unless directed

## Codegen

Three generators in `cmd/`:
- `generatets/` — Go → TypeScript type bindings
- `generatego/` — generates Go code
- `generateschema/` — generates JSON schema from config types

Trigger `task generate` after changing:
- `pkg/wshrpc/wshrpctypes.go` (RPC interface)
- `pkg/wconfig/settingsconfig.go` (config)
- `pkg/waveobj/wtypemeta.go` (wave object metadata)

## Testing

- Frontend: `npm test` (Vitest), config in `vitest.config.ts`
- Go: `go test ./pkg/...` from project root
- Coverage: `npm run coverage`

## Config & Env

Dev env vars (set by `task dev`):
- `WAVETERM_ENVFILE` — `.env` file for local secrets
- `WCLOUD_ENDPOINT` / `WCLOUD_WS_ENDPOINT` — dev cloud endpoints
- `WAVETERM_NOCONFIRMQUIT=1` — skip quit confirmation

Backend logs: `~/.waveterm-dev/waveapp.log`

## Important Files Reference

| File | Purpose |
|---|---|
| `.kilocode/rules/rules.md` | Full coding guidelines (authoritative) |
| `.kilocode/rules/overview.md` | Architecture overview |
| `.kilocode/skills/create-view/SKILL.md` | How to add a new view type |
| `.kilocode/skills/add-rpc/SKILL.md` | How to add a new RPC command |
| `.kilocode/skills/add-wshcmd/SKILL.md` | How to add a wsh CLI command |
| `.kilocode/skills/wps-events/SKILL.md` | How to use WPS pub/sub events |
| `.kilocode/skills/add-config/SKILL.md` | How to add config settings |
| `.kilocode/skills/electron-api/SKILL.md` | How to add Electron IPC APIs |
| `.github/copilot-instructions.md` | Skill guide index + preview server docs |

## Cowork Feature (Design Only)

`cowork.md` is a **design document only** — no implementation exists. The `cowork` branch is identical to `main` except for this file.

The design proposes a multi-agent AI collaboration view within Wave Terminal. Per the user's constraint, implementation should be as an **independent module** — not modifying the main codebase trunk. The view system's `BlockRegistry` pattern is the natural extension point for this (add `BlockRegistry.set("cowork", CoworkViewModel)`).

Key architectural decisions for cowork implementation:
- Frontend-only: Can live entirely in `frontend/app/view/cowork/` using the ViewModel pattern
- Backend needed: Would require new RPCs in `pkg/wshrpc/wshrpctypes.go` + `task generate`
- AI integration: Leverages existing `pkg/waveai/` and the AI provider infrastructure
- State: Use Jotai model singleton pattern with `globalStore`

## Contributing Notes

- Solo-maintainer project — discuss features in Discord before PRs
- One PR = one logical change, keep it focused
- Copyright year in new/modified files: 2026
