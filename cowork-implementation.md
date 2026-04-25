# Cowork 功能 — 详细实现计划

> 本文档为独立模块实现指南，供 AI Agent 直接执行。
> 所有实现必须遵循 [AGENTS.md](./AGENTS.md) 和 `.kilocode/rules/rules.md` 中的编码规范。

---

## 1. 架构概述

### 1.1 核心概念

Cowork 是 Wave Terminal 内置的一个 **多 Worker 协作管理面板**，以 View Block 的形式存在。

**两种角色：**

| 角色 | 是什么 | LLM 需求 | 运行方式 |
|---|---|---|---|
| **助理 Agent** | 监工秘书，负责调度 | 需要（复用 waveai） | 前端定时循环，调现有 `StreamWaveAiCommand` RPC |
| **Worker** | 执行具体任务的终端会话 | 不需要（是真实终端） | 独立终端块，跑 claude/opencode/cursor 等 CLI 工具 |

**关键设计决策：**
- 助理 Agent 是**纯前端逻辑**（setInterval 循环），通过现有 RPC 完成所有操作
- Worker 是助理通过 `CreateBlockCommand` 创建的终端块，通过 `ControllerInputCommand` 向其写入自然语言指令
- 数据持久化：后端 SQLite（新增 `pkg/cowork/` Go 包 + migration）
- Agent 间通信：共享任务板（SQLite 表）
- 助理通过 `TermGetScrollbackLinesCommand` 拉取 Worker 终端输出，用 LLM 分析进度/异常

### 1.2 架构图

```
┌──────────────────────────────────────────────────────────┐
│                  Cowork Dashboard Block                   │
│  (frontend/app/view/cowork/ — 纯前端模块)                  │
│                                                           │
│  ┌────────────────┐     ┌────────────────────────────┐  │
│  │  CoworkViewModel│────►│  CoworkCrudRPC (Go 后端)    │  │
│  │  (助理调度引擎)  │     │  pkg/cowork/               │  │
│  │  setInterval    │     │  cowork_db.go              │  │
│  │  10s 循环       │◄────│  cowork_types.go           │  │
│  └───┬────┬───┬───┘     └────────────────────────────┘  │
│      │    │    │                                          │
│      │    │    │  现有 RPC (不修改)                        │
│      │    │    ├── StreamWaveAiCommand (LLM 调用)         │
│      │    │    ├── CreateBlockCommand (创建终端块)         │
│      │    │    ├── ControllerInputCommand (写入终端)       │
│      │    │    ├── TermGetScrollbackLinesCommand (读终端)  │
│      │    │    ├── BlockInfoCommand (查 block 信息)       │
│      │    │    ├── DeleteBlockCommand (销毁终端块)         │
│      │    │    ├── GetSecretsCommand (读 API Key)         │
│      │    │    └── EventSubCommand (订阅 WPS 事件)        │
│      │    │                                              │
│      │    └── SQLite 任务板 (pkg/cowork/ 新增表)          │
│      │                                                   │
│      └── 创建/管理 Worker 终端块                           │
│          ┌─────────┐ ┌─────────┐ ┌─────────┐            │
│          │ Worker 1│ │ Worker 2│ │ Worker N│            │
│          │ claude  │ │opencode │ │ aider   │            │
│          │ (term)  │ │ (term)  │ │ (term)  │            │
│          └─────────┘ └─────────┘ └─────────┘            │
└──────────────────────────────────────────────────────────┘
```

### 1.3 对主干的修改清单

| 文件 | 改什么 | 量级 |
|---|---|---|
| `pkg/wshrpc/wshrpctypes.go` | 新增 cowork CRUD RPC 接口（~8 个方法） | ~60 行 |
| `pkg/wshrpc/wshserver/wshserver.go` | 实现 cowork RPC handler，委托给 `pkg/cowork/` | ~100 行 |
| `pkg/wps/wpstypes.go` | 新增 cowork WPS 事件常量 | ~5 行 |
| `db/migrations-wstore/000012_cowork.up.sql` | 新建 cowork 表 | 新文件 |
| `db/migrations-wstore/000012_cowork.down.sql` | 回滚 SQL | 新文件 |
| `frontend/app/block/blockregistry.ts` | 注册 cowork view | 1 行 |

**不修改的文件：** `pkg/waveai/`、`emain/`、`cmd/server/`、`pkg/wstore/`、`electron.vite.config.ts`、`Taskfile.yml`

---

## 2. 文件结构与职责

### 2.1 新增文件

```
pkg/cowork/                              # Go 后端 — cowork 数据层
  cowork_types.go                        # 数据类型定义
  cowork_db.go                           # SQLite CRUD 操作

db/migrations-wstore/
  000012_cowork.up.sql                   # 建表
  000012_cowork.down.sql                 # 回滚

frontend/app/view/cowork/                # 前端 — cowork 模块
  cowork-types.ts                        # TypeScript 类型定义
  cowork-model.ts                        # ViewModel（助理调度引擎 + UI 状态）
  cowork.tsx                             # React 组件（Dashboard UI）
```

### 2.2 各文件职责

**`pkg/cowork/cowork_types.go`** — Go 结构体定义
- `CoworkTask`：任务数据结构
- `CoworkWorker`：Worker 注册信息
- `CoworkActivity`：活动日志条目
- 各种 RPC 请求/响应类型

**`pkg/cowork/cowork_db.go`** — 纯数据访问层
- `InitCoworkDB()`：建表（备用，优先用 migration）
- CRUD 函数：`CreateTask`, `GetTask`, `UpdateTask`, `DeleteTask`, `ListTasks`
- CRUD 函数：`RegisterWorker`, `GetWorker`, `UpdateWorker`, `DeleteWorker`, `ListWorkers`
- `AddActivity`, `ListActivities`, `ClearOldActivities`

**`frontend/app/view/cowork/cowork-types.ts`** — 前端类型
- 从生成的 `gotypes.d.ts` 中获得 RPC 类型（`task generate` 后自动生成）
- 前端特有的 UI 状态类型

**`frontend/app/view/cowork/cowork-model.ts`** — 核心逻辑
- 继承 `ViewModel` 接口
- 助理调度引擎：`startSupervision()` / `stopSupervision()`
- 任务管理：创建/分配/更新/删除任务
- Worker 管理：创建终端块 / 监控输出 / 唤醒卡住的 Worker
- 所有状态用 Jotai atoms

**`frontend/app/view/cowork/cowork.tsx`** — UI
- 任务面板（待处理/进行中/已完成）
- Worker 列表（状态、分配的任务）
- 活动日志流
- 监控开关

---

## 3. 数据库设计

### 3.1 Migration 文件

**`db/migrations-wstore/000012_cowork.up.sql`**

```sql
-- 任务表
CREATE TABLE IF NOT EXISTS cowork_tasks (
    task_id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT DEFAULT '',
    priority TEXT NOT NULL DEFAULT 'medium' CHECK(priority IN ('low', 'medium', 'high', 'urgent')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'assigned', 'working', 'done', 'failed')),
    assigned_worker TEXT DEFAULT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
    completed_at INTEGER DEFAULT NULL,
    result TEXT DEFAULT NULL,
    error TEXT DEFAULT NULL,
    progress TEXT DEFAULT NULL
);

-- Worker 注册表
CREATE TABLE IF NOT EXISTS cowork_workers (
    worker_id TEXT PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    tool TEXT NOT NULL DEFAULT '' CHECK(tool IN ('claude', 'opencode', 'cursor', 'aider', 'custom')),
    custom_cmd TEXT DEFAULT NULL,
    status TEXT NOT NULL DEFAULT 'idle' CHECK(status IN ('idle', 'working', 'offline', 'error')),
    assigned_task TEXT DEFAULT NULL,
    block_id TEXT NOT NULL,
    tab_id TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    last_active_at INTEGER NOT NULL DEFAULT (unixepoch()),
    last_output_hash TEXT DEFAULT NULL,
    error_msg TEXT DEFAULT NULL
);

-- 活动日志（滚动窗口）
CREATE TABLE IF NOT EXISTS cowork_activity (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT DEFAULT NULL,
    worker_id TEXT DEFAULT NULL,
    type TEXT NOT NULL DEFAULT 'info',
    description TEXT NOT NULL DEFAULT '',
    meta TEXT DEFAULT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_cowork_tasks_status ON cowork_tasks(status, priority);
CREATE INDEX IF NOT EXISTS idx_cowork_tasks_worker ON cowork_tasks(assigned_worker);
CREATE INDEX IF NOT EXISTS idx_cowork_workers_status ON cowork_workers(status);
CREATE INDEX IF NOT EXISTS idx_cowork_activity_created ON cowork_activity(created_at);
```

**`db/migrations-wstore/000012_cowork.down.sql`**

```sql
DROP TABLE IF EXISTS cowork_activity;
DROP TABLE IF EXISTS cowork_workers;
DROP TABLE IF EXISTS cowork_tasks;
```

### 3.2 表关系

- `cowork_tasks.assigned_worker` → `cowork_workers.worker_id`（逻辑外键，不建约束）
- `cowork_activity.task_id` / `cowork_activity.worker_id` → 关联引用，允许 NULL
- 活动日志定期清理：保留最近 2000 条

---

## 4. Go 后端实现

### 4.1 RPC 接口定义

在 `pkg/wshrpc/wshrpctypes.go` 的 `WshRpcInterface` 中新增以下方法：

```go
// cowork
CoworkCreateTaskCommand(ctx context.Context, data CoworkCreateTaskData) (*CoworkTask, error)
CoworkGetTaskCommand(ctx context.Context, taskId string) (*CoworkTask, error)
CoworkUpdateTaskCommand(ctx context.Context, data CoworkUpdateTaskData) (*CoworkTask, error)
CoworkDeleteTaskCommand(ctx context.Context, taskId string) error
CoworkListTasksCommand(ctx context.Context, data CoworkListTasksData) ([]*CoworkTask, error)
CoworkRegisterWorkerCommand(ctx context.Context, data CoworkRegisterWorkerData) (*CoworkWorker, error)
CoworkUpdateWorkerCommand(ctx context.Context, data CoworkUpdateWorkerData) (*CoworkWorker, error)
CoworkListWorkersCommand(ctx context.Context) ([]*CoworkWorker, error)
CoworkDeleteWorkerCommand(ctx context.Context, workerId string) error
CoworkAddActivityCommand(ctx context.Context, data CoworkAddActivityData) error
CoworkListActivityCommand(ctx context.Context, data CoworkListActivityData) ([]*CoworkActivity, error)
CoworkGetStatusCommand(ctx context.Context) (*CoworkStatusData, error)
```

### 4.2 RPC 数据类型

在 `pkg/wshrpc/wshrpctypes.go` 中新增：

```go
// Cowork Task
type CoworkTask struct {
    TaskId        string `json:"taskid"`
    Title         string `json:"title"`
    Description   string `json:"description,omitempty"`
    Priority      string `json:"priority"`
    Status        string `json:"status"`
    AssignedWorker string `json:"assignedworker,omitempty"`
    CreatedAt     int64  `json:"createdat"`
    UpdatedAt     int64  `json:"updatedat"`
    CompletedAt   int64  `json:"completedat,omitempty"`
    Result        string `json:"result,omitempty"`
    Error         string `json:"error,omitempty"`
    Progress      string `json:"progress,omitempty"`
}

// Cowork Worker
type CoworkWorker struct {
    WorkerId      string `json:"workerid"`
    Name          string `json:"name"`
    Tool          string `json:"tool"`
    CustomCmd     string `json:"customcmd,omitempty"`
    Status        string `json:"status"`
    AssignedTask  string `json:"assignedtask,omitempty"`
    BlockId       string `json:"blockid"`
    TabId         string `json:"tabid"`
    CreatedAt     int64  `json:"createdat"`
    LastActiveAt  int64  `json:"lastactiveat"`
    LastOutputHash string `json:"lastoutputhash,omitempty"`
    ErrorMsg      string `json:"errormsg,omitempty"`
}

// Cowork Activity
type CoworkActivity struct {
    Id          int64  `json:"id"`
    TaskId      string `json:"taskid,omitempty"`
    WorkerId    string `json:"workerid,omitempty"`
    Type        string `json:"type"`
    Description string `json:"description"`
    Meta        string `json:"meta,omitempty"`
    CreatedAt   int64  `json:"createdat"`
}

// Request/Response types
type CoworkCreateTaskData struct {
    Title       string `json:"title"`
    Description string `json:"description,omitempty"`
    Priority    string `json:"priority"`
}

type CoworkUpdateTaskData struct {
    TaskId        string `json:"taskid"`
    Title         string `json:"title,omitempty"`
    Description   string `json:"description,omitempty"`
    Priority      string `json:"priority,omitempty"`
    Status        string `json:"status,omitempty"`
    AssignedWorker string `json:"assignedworker,omitempty"`
    Result        string `json:"result,omitempty"`
    Error         string `json:"error,omitempty"`
    Progress      string `json:"progress,omitempty"`
}

type CoworkListTasksData struct {
    Status   string `json:"status,omitempty"`
    Priority string `json:"priority,omitempty"`
}

type CoworkRegisterWorkerData struct {
    WorkerId  string `json:"workerid"`
    Name      string `json:"name"`
    Tool      string `json:"tool"`
    CustomCmd string `json:"customcmd,omitempty"`
    BlockId   string `json:"blockid"`
    TabId     string `json:"tabid"`
}

type CoworkUpdateWorkerData struct {
    WorkerId      string `json:"workerid"`
    Status        string `json:"status,omitempty"`
    AssignedTask  string `json:"assignedtask,omitempty"`
    LastOutputHash string `json:"lastoutputhash,omitempty"`
    ErrorMsg      string `json:"errormsg,omitempty"`
}

type CoworkAddActivityData struct {
    TaskId      string `json:"taskid,omitempty"`
    WorkerId    string `json:"workerid,omitempty"`
    Type        string `json:"type"`
    Description string `json:"description"`
    Meta        string `json:"meta,omitempty"`
}

type CoworkListActivityData struct {
    Limit int `json:"limit,omitempty"`
}

type CoworkStatusData struct {
    PendingTasks  int `json:"pendingtasks"`
    WorkingTasks  int `json:"workingtasks"`
    DoneTasks     int `json:"donetasks"`
    FailedTasks   int `json:"failedtasks"`
    ActiveWorkers int `json:"activeworkers"`
    IdleWorkers   int `json:"idleworkers"`
}
```

### 4.3 WPS 事件（新增常量）

在 `pkg/wps/wpstypes.go` 中新增：

```go
Event_CoworkTaskUpdate  = "cowork:taskupdate"   // type: none
Event_CoworkWorkerUpdate = "cowork:workerupdate" // type: none
```

同时将这两个常量加入 `AllEvents` 切片。

**注意：** 按照现有代码规范，还需要在 `pkg/tsgen/tsgenevent.go` 的 `WaveEventDataTypes` 中添加对应条目（用 `nil` 表示无数据），这样 `task generate` 才能正确生成 TS 类型。

### 4.4 `pkg/cowork/cowork_db.go` 实现要点

```go
package cowork

import (
    "context"
    "database/sql"
    "fmt"

    "github.com/wavetermdev/waveterm/pkg/wstore"
)

// 获取数据库连接（复用 wstore 的全局 DB）
func getDB(ctx context.Context) (*sql.DB, error) {
    // 通过 wstore 暴露的 DB 连接操作
    // 需要在 wstore 中新增一个导出函数 GetDB() *sqlx.DB
    // 或者 cowork 包直接打开同一个 SQLite 文件
}
```

**实际做法：** 因为 `wstore` 没有暴露 `*sqlx.DB`，cowork 有两种选择：

1. **推荐**：在 `pkg/wstore/wstore_dbsetup.go` 中新增一个 `func GetGlobalDB() *sqlx.DB` 导出函数（1 行代码），cowork 直接使用
2. **备选**：cowork 用 `wavebase.GetWaveDataDir()` + `wstore.WStoreDBName` 拼接路径，自己打开同一个 SQLite 文件

选择方案 1（改动最小，复用连接池）。在 `pkg/wstore/wstore_dbsetup.go` 新增：

```go
func GetGlobalDB() *sqlx.DB {
    return globalDB
}
```

### 4.5 CRUD 函数签名

```go
// cowork_db.go

func CreateTask(ctx context.Context, task *wshrpc.CoworkTask) error
func GetTask(ctx context.Context, taskId string) (*wshrpc.CoworkTask, error)
func UpdateTask(ctx context.Context, task *wshrpc.CoworkTask) error
func DeleteTask(ctx context.Context, taskId string) error
func ListTasks(ctx context.Context, status, priority string) ([]*wshrpc.CoworkTask, error)

func RegisterWorker(ctx context.Context, worker *wshrpc.CoworkWorker) error
func GetWorker(ctx context.Context, workerId string) (*wshrpc.CoworkWorker, error)
func UpdateWorker(ctx context.Context, worker *wshrpc.CoworkWorker) error
func DeleteWorker(ctx context.Context, workerId string) error
func ListWorkers(ctx context.Context) ([]*wshrpc.CoworkWorker, error)

func AddActivity(ctx context.Context, activity *wshrpc.CoworkActivity) error
func ListActivities(ctx context.Context, limit int) ([]*wshrpc.CoworkActivity, error)
func CleanupOldActivities(ctx context.Context, maxCount int) error

func GetStatus(ctx context.Context) (*wshrpc.CoworkStatusData, error)
```

### 4.6 `wshserver.go` Handler 实现

在 `pkg/wshrpc/wshserver/wshserver.go` 中新增 Handler 函数，每个都直接委托给 `cowork` 包：

```go
func (ws *WshServer) CoworkCreateTaskCommand(ctx context.Context, data wshrpc.CoworkCreateTaskData) (*wshrpc.CoworkTask, error) {
    task := &wshrpc.CoworkTask{
        TaskId:      uuid.New().String(),
        Title:       data.Title,
        Description: data.Description,
        Priority:    data.Priority,
        Status:      "pending",
        CreatedAt:   time.Now().Unix(),
        UpdatedAt:   time.Now().Unix(),
    }
    if task.Priority == "" {
        task.Priority = "medium"
    }
    err := cowork.CreateTask(ctx, task)
    if err != nil {
        return nil, err
    }
    // 发布 WPS 事件通知前端
    wps.PublishEvent(ctx, wps.WaveEvent{Event: wps.Event_CoworkTaskUpdate})
    return task, nil
}

// 其他 Handler 类似模式...
```

---

## 5. 前端实现

### 5.1 Codegen 步骤（必须先执行）

修改完 `wshrpctypes.go` 和 `wpstypes.go` 后，**必须运行**：

```bash
task generate
```

这会自动生成：
- `frontend/types/gotypes.d.ts` — 包含所有 cowork RPC 类型的 TS 定义
- `frontend/app/store/wshclientapi.ts` — 包含所有 cowork RPC 客户端方法

**不要手动编辑这两个文件。**

### 5.2 `cowork-model.ts` — ViewModel

遵循 `.kilocode/skills/create-view/SKILL.md` 的 ViewModel 模式和 `.kilocode/rules/rules.md` 的 Jotai Model Pattern。

```typescript
import { BlockNodeModel } from "@/app/block/blocktypes";
import { globalStore } from "@/app/store/jotaiStore";
import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";
import { waveEventSubscribeSingle } from "@/app/store/wps";
import { WOS } from "@/app/store/global";
import * as jotai from "jotai";
import { stringToBase64 } from "@/util/util";
import { CoworkView } from "./cowork";

export class CoworkViewModel implements ViewModel {
    viewType = "cowork";
    blockId: string;
    viewIcon = jotai.atom("users");
    viewName = jotai.atom("Cowork");
    noPadding = jotai.atom(false);
    viewComponent = CoworkView;

    private static instance: CoworkViewModel | null = null;

    // === Jotai Atoms（状态） ===

    // 任务列表（按状态分组）
    pendingTasksAtom = jotai.atom<CoworkTask[]>([]);
    workingTasksAtom = jotai.atom<CoworkTask[]>([]);
    doneTasksAtom = jotai.atom<CoworkTask[]>([]);
    failedTasksAtom = jotai.atom<CoworkTask[]>([]);

    // Worker 列表
    workersAtom = jotai.atom<CoworkWorker[]>([]);

    // 活动日志
    activityLogAtom = jotai.atom<CoworkActivity[]>([]);

    // 监控状态
    isSupervisingAtom = jotai.atom(false);
    supervisorInterval: number | null = null;
    lastLLMCallAtom = jotai.atom<string>("");
    isProcessingAtom = jotai.atom(false);

    // 汇总状态
    statusAtom!: jotai.Atom<CoworkStatusData>;

    // 助理配置
    supervisionIntervalMs = jotai.atom(10000); // 10秒
    maxScrollbackLines = 50; // 拉取终端最后50行

    // 错误信息
    errorAtom = jotai.atom<string | null>(null);

    constructor({ blockId, nodeModel }: ViewModelInitType) {
        this.blockId = blockId;

        this.statusAtom = jotai.atom((get) => ({
            pendingtasks: get(this.pendingTasksAtom).length,
            workingtasks: get(this.workingTasksAtom).length,
            donetasks: get(this.doneTasksAtom).length,
            failedtasks: get(this.failedTasksAtom).length,
            activeworkers: get(this.workersAtom).filter(w => w.status === "working").length,
            idleworkers: get(this.workersAtom).filter(w => w.status === "idle").length,
        }));
    }

    static getInstance(): CoworkViewModel {
        if (!CoworkViewModel.instance) {
            CoworkViewModel.instance = new CoworkViewModel({ blockId: "", nodeModel: null });
        }
        return CoworkViewModel.instance;
    }

    // === 生命周期 ===

    async init(): Promise<void> {
        await this.refreshAllData();
    }

    dispose(): void {
        this.stopSupervision();
        CoworkViewModel.instance = null;
    }

    // === 监控引擎 ===

    startSupervision(): void {
        if (globalStore.get(this.isSupervisingAtom)) return;
        globalStore.set(this.isSupervisingAtom, true);
        this.runSupervisionCycle(); // 立即执行一次
        const interval = globalStore.get(this.supervisionIntervalMs);
        this.supervisorInterval = window.setInterval(
            () => this.runSupervisionCycle(),
            interval,
        );
    }

    stopSupervision(): void {
        globalStore.set(this.isSupervisingAtom, false);
        if (this.supervisorInterval != null) {
            clearInterval(this.supervisorInterval);
            this.supervisorInterval = null;
        }
    }

    // === 核心调度循环 ===

    private async runSupervisionCycle(): Promise<void> {
        try {
            globalStore.set(this.isProcessingAtom, true);
            await this.refreshAllData();

            const pendingTasks = globalStore.get(this.pendingTasksAtom);
            const workers = globalStore.get(this.workersAtom);

            // 没有待处理任务且没有工作中任务 → 跳过 LLM 调用
            if (pendingTasks.length === 0 &&
                globalStore.get(this.workingTasksAtom).length === 0 &&
                workers.every(w => w.status !== "working")) {
                return;
            }

            // 收集各 Worker 终端输出
            const workerOutputs = await this.collectWorkerOutputs(workers);

            // 构建分析 Prompt
            const prompt = this.buildAnalysisPrompt(
                pendingTasks,
                globalStore.get(this.workingTasksAtom),
                globalStore.get(this.doneTasksAtom),
                globalStore.get(this.failedTasksAtom),
                workerOutputs,
            );

            // 调用 LLM 分析
            const action = await this.callAssistantLLM(prompt);
            globalStore.set(this.lastLLMCallAtom, new Date().toISOString());

            // 执行 LLM 返回的指令
            await this.executeAssistantActions(action);

        } catch (e) {
            globalStore.set(this.errorAtom, String(e));
        } finally {
            globalStore.set(this.isProcessingAtom, false);
        }
    }

    // ... 更多方法见下文
}
```

### 5.3 助理 LLM 调用

助理使用现有 `StreamWaveAiCommand` RPC，复用 Wave 的 AI 配置（API Key、Model 等）：

```typescript
private async callAssistantLLM(prompt: string): Promise<AssistantAction> {
    const request: WaveAIStreamRequest = {
        opts: {
            // 复用用户的默认 AI 配置
            // 通过 GetFullConfigCommand 获取，或使用 Block Meta 中的覆盖配置
            model: this.aiModel ?? "claude-sonnet-4-20250514",
            apitype: this.aiType ?? "anthropic",
        },
        prompt: [
            {
                role: "system",
                content: ASSISTANT_SYSTEM_PROMPT,
            },
            {
                role: "user",
                content: prompt,
            },
        ],
    };

    let fullResponse = "";
    const stream = RpcApi.StreamWaveAiCommand(TabRpcClient, request);
    try {
        while (true) {
            const result = await stream.next();
            if (result.done) break;
            const packet = result.value;
            if (packet.error) {
                throw new Error(`LLM error: ${packet.error}`);
            }
            if (packet.text) {
                fullResponse += packet.text;
            }
        }
    } finally {
        if (stream.return) {
            await stream.return();
        }
    }

    // 解析 LLM 返回的 JSON 指令
    return this.parseAssistantResponse(fullResponse);
}
```

### 5.4 创建 Worker 终端块

```typescript
private async createWorkerBlock(tool: string, taskTitle: string): Promise<string> {
    // 1. 获取当前 tabId
    const blockInfo = await RpcApi.BlockInfoCommand(TabRpcClient, this.blockId);
    const tabId = blockInfo.tabid;

    // 2. 获取当前 tab 下已有 blocks，确定插入位置
    // cowork block 本身作为 parent，创建 sub-block
    const workerBlockDef: BlockDef = {
        meta: {
            view: "term",
            "cowork:worker": "true",
            "cowork:tool": tool,
            connection: "", // 默认本地
        },
    };

    // 3. 创建终端子块（在 cowork block 内部）
    const oref = await RpcApi.CreateSubBlockCommand(TabRpcClient, {
        parentblockid: this.blockId,
        blockdef: workerBlockDef,
    });

    const workerBlockId = oref.oid;

    // 4. 等待终端初始化
    await new Promise(resolve => setTimeout(resolve, 2000));

    // 5. 根据工具类型，向终端写入启动命令
    const startCmd = this.getWorkerStartCommand(tool);
    await this.sendToTerminal(workerBlockId, startCmd + "\n");

    // 6. 注册 Worker
    await RpcApi.CoworkRegisterWorkerCommand(TabRpcClient, {
        workerid: workerBlockId,
        name: `${tool} (${taskTitle.substring(0, 30)})`,
        tool: tool,
        blockid: workerBlockId,
        tabid: tabId,
    });

    return workerBlockId;
}

private getWorkerStartCommand(tool: string): string {
    switch (tool) {
        case "claude":  return "claude";
        case "opencode": return "opencode";
        case "cursor":   return "cursor-agent";
        case "aider":    return "aider";
        default:         return tool; // custom
    }
}

private async sendToTerminal(blockId: string, text: string): Promise<void> {
    const b64data = stringToBase64(text);
    await RpcApi.ControllerInputCommand(TabRpcClient, {
        blockid: blockId,
        inputdata64: b64data,
    });
}
```

### 5.5 收集 Worker 输出

```typescript
private async collectWorkerOutputs(workers: CoworkWorker[]): Promise<Map<string, WorkerOutput>> {
    const outputs = new Map<string, WorkerOutput>();

    for (const worker of workers) {
        if (worker.status !== "working") continue;

        try {
            const scrollback = await RpcApi.TermGetScrollbackLinesCommand(TabRpcClient, {
                linestart: -this.maxScrollbackLines,
                lineend: -1,
                lastcommand: false,
            });

            // 注意：TermGetScrollbackLinesCommand 需要通过 block route 调用
            // 实际调用方式需要通过 WshRouter 路由到具体 block
            // 见下文 5.8 节关于 block route 的说明

            const outputHash = this.simpleHash(scrollback.lines.join("\n"));

            outputs.set(worker.workerid, {
                lines: scrollback.lines,
                totalLines: scrollback.totallines,
                lastUpdated: scrollback.lastupdated,
                hashChanged: outputHash !== worker.lastoutputhash,
            });

            // 更新 hash，避免重复分析
            if (outputHash !== worker.lastoutputhash) {
                await RpcApi.CoworkUpdateWorkerCommand(TabRpcClient, {
                    workerid: worker.workerid,
                    lastoutputhash: outputHash,
                });
            }
        } catch (e) {
            // Worker 终端可能已关闭
            outputs.set(worker.workerid, { error: String(e) });
        }
    }

    return outputs;
}
```

### 5.6 助理 System Prompt

```typescript
const ASSISTANT_SYSTEM_PROMPT = `You are a project management assistant embedded in Wave Terminal. You manage a team of AI coding agents that run in terminal sessions.

## Your Role
- Monitor task progress by analyzing terminal output
- Assign pending tasks to available workers
- Detect stuck/errored workers and send natural language prompts to wake them
- Report completion status

## Communication with Workers
You communicate with workers by writing text into their terminal sessions. The text you output will be typed into their terminal. Be concise and direct.

## Task Status Machine
- pending → assigned: when you assign to a worker
- assigned → working: when worker starts showing activity
- working → done: when worker output indicates completion
- working → failed: when worker shows repeated errors

## Response Format
You MUST respond with valid JSON in this exact format (no markdown, no explanation):
{
  "actions": [
    {
      "type": "assign_task",
      "task_id": "...",
      "worker_id": "...",
      "instruction": "natural language instruction for the worker"
    },
    {
      "type": "wake_worker",
      "worker_id": "...",
      "message": "natural language message to send to stuck worker"
    },
    {
      "type": "update_task",
      "task_id": "...",
      "status": "done|failed",
      "result": "completion summary",
      "progress": "optional progress note"
    },
    {
      "type": "create_worker",
      "tool": "claude|opencode|cursor|aider",
      "task_id": "..."
    },
    {
      "type": "noop",
      "reason": "why no action is needed"
    }
  ]
}

Only take actions when the situation actually warrants it. Do not wake workers that are making progress. Do not reassign tasks that are being worked on.`;
```

### 5.7 LLM 返回解析

```typescript
interface AssistantAction {
    actions: Array<{
        type: "assign_task" | "wake_worker" | "update_task" | "create_worker" | "noop";
        task_id?: string;
        worker_id?: string;
        instruction?: string;
        message?: string;
        status?: string;
        result?: string;
        progress?: string;
        tool?: string;
        reason?: string;
    }>;
}

private parseAssistantResponse(text: string): AssistantAction {
    // 尝试从文本中提取 JSON
    const jsonMatch = text.match(/\{[\s\S]*\}/);
    if (!jsonMatch) {
        return { actions: [{ type: "noop", reason: "Failed to parse LLM response" }] };
    }

    try {
        return JSON.parse(jsonMatch[0]) as AssistantAction;
    } catch {
        return { actions: [{ type: "noop", reason: "Invalid JSON from LLM" }] };
    }
}

private async executeAssistantActions(action: AssistantAction): Promise<void> {
    for (const act of action.actions) {
        try {
            switch (act.type) {
                case "assign_task":
                    await this.assignTaskToWorker(act.task_id!, act.worker_id!);
                    if (act.instruction) {
                        await this.sendToTerminal(act.worker_id!, act.instruction + "\n");
                    }
                    break;

                case "wake_worker":
                    if (act.message) {
                        await this.sendToTerminal(act.worker_id!, act.message + "\n");
                    }
                    await this.logActivity("worker_wake", `Woke worker ${act.worker_id}`);
                    break;

                case "update_task":
                    await this.updateTaskStatus(act.task_id!, act.status!, act.result, act.progress);
                    break;

                case "create_worker":
                    const workerId = await this.createWorkerBlock(act.tool!, act.task_id!);
                    await this.assignTaskToWorker(act.task_id!, workerId);
                    break;

                case "noop":
                    break;
            }
        } catch (e) {
            await this.logActivity("error", `Action ${act.type} failed: ${e}`);
        }
    }
}
```

### 5.8 关于 Block Route 调用

**重要：** `TermGetScrollbackLinesCommand` 和 `ControllerInputCommand` 需要路由到具体的 block，不是 tab 级别的 RPC。

查看现有代码，term-model.ts 中的 `sendDataToController` 直接用 `TabRpcClient` 发送，但 `blockid` 在 data 参数中指定。`TermGetScrollbackLinesCommand` 同理。

实际做法参考 `frontend/app/view/term/term-model.ts` 第 509-511 行：

```typescript
// 向终端写入
const b64data = stringToBase64(data);
RpcApi.ControllerInputCommand(TabRpcClient, { blockid: this.blockId, inputdata64: b64data });
```

**但注意：** `TermGetScrollbackLinesCommand` 的 `CommandTermGetScrollbackLinesData` 结构体中没有 `blockId` 字段。这说明这个命令是通过 block 级别的 route 发送的（不是 tab 级别）。

需要进一步确认：查看 `cmd/` 中是否有 block-level RPC 的实现模式。如果 `TermGetScrollbackLinesCommand` 必须在 block route 上调用，则需要通过 `WshRouter` 获取对应 block 的 RPC client。

替代方案：使用 `ControllerStatus` WPS 事件来监听 Worker 终端状态变化，然后通过 `Event_ControllerStatus` 事件获取终端数据。这种方式不需要直接拉取 scrollback。

**最终建议：** 先用 WPS 事件监控 Worker 状态变化，在需要详细分析时再通过 `GetBlockCommand` 或 block route 读取终端输出。实施时根据实际 API 行为调整。

### 5.9 UI 组件结构 (`cowork.tsx`)

```
┌──────────────────────────────────────────────────┐
│ [👑 助理监控: ON/OFF]  [间隔: 10s ▼]  [刷新]     │
├──────────────────────────────────────────────────┤
│                                                  │
│ 📋 任务板                           [+ 新建任务]  │
│ ┌──────────┬──────────┬──────────┬──────────┐   │
│ │ 待处理(3) │ 进行中(2) │ 已完成(5) │ 失败(1)  │   │
│ └──────────┴──────────┴──────────┴──────────┘   │
│                                                   │
│ 🔧 Workers                            [+ 拉起]    │
│ ┌────────────────────────────────────────────┐   │
│ │ 🟢 claude-1 (working) → 修复登录 bug       │   │
│ │ 🟢 opencode-2 (working) → 写单元测试        │   │
│ │ ⚪ aider-3 (idle)                          │   │
│ └────────────────────────────────────────────┘   │
│                                                   │
│ 📝 活动日志                                       │
│ ┌────────────────────────────────────────────┐   │
│ │ 12:30:01 [task] 任务"修复登录"分配给claude-1 │   │
│ │ 12:29:55 [worker] claude-1 终端输出变化       │   │
│ │ 12:29:50 [worker] 创建 claude-1 终端块        │   │
│ └────────────────────────────────────────────┘   │
│                                                   │
│ Worker 终端区域（子块，由助理自动创建管理）          │
│ ┌──────────────┬──────────────┐                  │
│ │ claude-1     │ opencode-2   │                  │
│ │ $ fix login  │ $ write tests│                  │
│ │ ...          │ ...          │                  │
│ └──────────────┴──────────────┘                  │
└──────────────────────────────────────────────────┘
```

UI 使用 Tailwind v4，遵循现有样式约定。参考样式模式：

```typescript
// 监控开关按钮
<button className="bg-accent/80 text-primary rounded hover:bg-accent transition-colors cursor-pointer">

// 任务卡片
<div className="rounded border border-border/50 bg-base p-3">

// Worker 状态指示
<span className={`inline-block w-2 h-2 rounded-full ${
    worker.status === "working" ? "bg-green-500" :
    worker.status === "idle" ? "bg-gray-400" :
    worker.status === "error" ? "bg-red-500" : "bg-gray-300"
}`} />
```

### 5.10 View 注册

在 `frontend/app/block/blockregistry.ts` 中新增：

```typescript
import { CoworkViewModel } from "@/app/view/cowork/cowork-model";
// ...
BlockRegistry.set("cowork", CoworkViewModel);
```

---

## 6. 实现步骤（执行顺序）

### Phase 1: 后端数据层

1. **创建 migration 文件**
   - `db/migrations-wstore/000012_cowork.up.sql`
   - `db/migrations-wstore/000012_cowork.down.sql`

2. **新增导出函数**（1 行改动）
   - 在 `pkg/wstore/wstore_dbsetup.go` 中新增 `func GetGlobalDB() *sqlx.DB`

3. **创建 `pkg/cowork/` 包**
   - `cowork_types.go` — 类型定义（直接 import `wshrpc` 中的类型）
   - `cowork_db.go` — 所有 CRUD 函数

4. **新增 RPC 接口**
   - 在 `pkg/wshrpc/wshrpctypes.go` 的 `WshRpcInterface` 中添加 cowork 方法
   - 添加所有请求/响应数据类型

5. **新增 WPS 事件**
   - 在 `pkg/wps/wpstypes.go` 添加 `Event_CoworkTaskUpdate` 和 `Event_CoworkWorkerUpdate`
   - 在 `AllEvents` 切片中添加这两个常量
   - 在 `pkg/tsgen/tsgenevent.go` 的 `WaveEventDataTypes` 中添加对应条目

6. **实现 RPC Handler**
   - 在 `pkg/wshrpc/wshserver/wshserver.go` 中实现所有 cowork Command 方法

7. **运行 codegen**
   ```bash
   task generate
   ```
   - 验证生成的 `frontend/types/gotypes.d.ts` 和 `frontend/app/store/wshclientapi.ts` 包含 cowork 类型

### Phase 2: 前端实现

8. **创建 `frontend/app/view/cowork/cowork-types.ts`**
   - 前端特有的 UI 状态类型（如果 generated types 不够用）
   - `AssistantAction` 接口等

9. **创建 `frontend/app/view/cowork/cowork-model.ts`**
   - `CoworkViewModel` 类
   - 监控引擎逻辑
   - LLM 调用逻辑
   - Worker 管理逻辑

10. **创建 `frontend/app/view/cowork/cowork.tsx`**
    - Dashboard UI 组件
    - 任务面板、Worker 列表、活动日志

11. **注册 View**
    - 修改 `frontend/app/block/blockregistry.ts`，添加 1 行

### Phase 3: 验证

12. **TypeScript 检查**
    ```bash
    task check:ts
    ```

13. **前端测试**
    ```bash
    task test
    ```

14. **启动验证**
    ```bash
    task dev
    ```
    - 创建新的 cowork block：在 Wave Terminal 中打开新 block，view type 选择 "cowork"
    - 验证 dashboard 正常渲染
    - 创建测试任务，验证 CRUD
    - 启动监控，验证 LLM 调用

---

## 7. 风险与注意事项

### 7.1 `TermGetScrollbackLinesCommand` 的路由问题

这个命令可能需要在 block-level route 上调用（而非 tab-level）。实施时需要：
1. 确认 `CommandTermGetScrollbackLinesData` 是否需要 blockId
2. 如果需要 block route，通过 `WshRouter.getRouteClient(blockId)` 获取对应 client
3. 备选方案：仅依赖 WPS `Event_ControllerStatus` 事件做状态监控

### 7.2 LLM 输出解析的鲁棒性

LLM 可能不返回严格 JSON。需要：
1. `parseAssistantResponse` 做好容错处理
2. 考虑在 system prompt 中强调 JSON 格式
3. 解析失败时走 noop 路径，不要 crash

### 7.3 Worker 终端块的 Sub-block vs Tab-level

当前设计是 Worker 作为 cowork block 的 sub-block 创建。需要验证：
1. `CreateSubBlockCommand` 创建的 term view 是否正常初始化 shell
2. sub-block 的 `ControllerInputCommand` 是否可以通过 TabRpcClient 发送
3. 如果 sub-block 有限制，备选方案是在同一 tab 下创建 sibling block

### 7.4 并发安全

助理的 `setInterval` 回调和 WPS 事件回调可能并发执行。需要：
1. 用 `isProcessingAtom` 做简单的互斥（一次只处理一个循环）
2. 如果前一个循环还在执行（LLM 调用中），跳过本次

### 7.5 Worker 断连处理

Worker 终端可能被用户手动关闭、shell 退出、或进程崩溃。需要：
1. 订阅 WPS `Event_BlockClose` 事件（scope 为 worker blockId）
2. 当收到 close 事件时，更新 worker 状态为 offline
3. 将其 assigned task 重新标记为 pending

---

## 8. 未来扩展（本次不实现）

- 命令审批队列（危险操作需人工确认）
- Token 用量追踪和预算控制
- Worker 的自动扩缩容策略
- 助理的 project context（自动读取项目 README/配置来增强 prompt）
- 任务依赖关系（DAG 调度）
- 多人协作（多个人类用户同时操作）
