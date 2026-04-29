# PRD: Cowork 配置界面重新设计

## Overview

重新设计 Cowork 功能的整个前端 UI，以看板（Board）视图为核心，参考 Multica 的设计语言，结合 cowork 自身的功能特点（Worker = 终端 block + Agent CLI），实现美观、易用、信息密度合理的配置界面。

## 目标

1. **看板优先** — Board 视图作为主视图，按任务状态分列展示
2. **美观易用** — 参考 Multica 设计语言（细边框卡片、语义颜色、hover 效果）
3. **功能完整** — 覆盖所有现有 cowork RPC 命令的功能
4. **动态铺满** — 页面布局自适应，卡片网格填满可用空间

## 现有功能清单（必须全部保留）

| 功能 | RPC 命令 |
|------|---------|
| Runtime 检测 | `CoworkDetectRuntimesCommand` |
| Worker 注册（含 name/tool/role/desc/soul/skills/mcp/concurrency/timeout/maxRetries/capabilities） | `CoworkRegisterWorkerCommand` |
| Worker 列表 | `CoworkListWorkersCommand` |
| Worker 更新 | `CoworkUpdateWorkerCommand` |
| Worker 删除 | `CoworkDeleteWorkerCommand` |
| Task 创建（title/desc/priority/deps） | `CoworkCreateTaskCommand` |
| Task 列表/筛选（status/priority） | `CoworkListTasksCommand` |
| Task 更新（status/assignedworker/result/error/progress） | `CoworkUpdateTaskCommand` |
| Task 删除 | `CoworkDeleteTaskCommand` |
| Task 执行（指定 worker+command） | `CoworkExecuteTaskCommand` |
| Task 暂停 | `CoworkPauseTaskCommand` |
| Task 恢复 | `CoworkResumeTaskCommand` |
| Task 重试 | `CoworkRetryTaskCommand` |
| Task 输出历史 | `CoworkGetTaskOutputHistoryCommand` |
| Activity 日志 | `CoworkListActivityCommand` / `CoworkAddActivityCommand` |
| 状态概览 | `CoworkGetStatusCommand` |
| 自动监督模式 | 前端 LLM 循环（startSupervision/stopSupervision） |

## 数据模型

### CoworkTask
```
TaskId, Title, Description, Priority (urgent/high/medium/low),
Status (pending/assigned/working/paused/done/failed),
AssignedWorker, CreatedAt, UpdatedAt, CompletedAt,
Result, Error, Progress, OutputHistory[], DependsOn[],
RetryCount, MaxRetries, NextRetryAt
```

### CoworkWorker
```
WorkerId, Name, Tool (claude/opencode/cursor/aider/custom),
CustomCmd, Role, Desc, Soul, Skills, McpServers,
Status (idle/working/offline/error),
AssignedTask, BlockId, TabId, CreatedAt, LastActiveAt,
LastOutputHash, ErrorMsg, Capabilities[]
```

### CoworkActivity
```
Id, TaskId, WorkerId, Type, Description, Meta, CreatedAt
```

### AIRuntime
```
Name, DisplayName, Command, Version, Status (online/offline)
```

### CoworkStatusData
```
PendingTasks, WorkingTasks, DoneTasks, FailedTasks,
ActiveWorkers, IdleWorkers, TotalWorkers
```

## UI 设计

### 整体布局

```
┌──────────────────────────────────────────────────────────────────┐
│ PageHeader                                                       │
├──────────────────────────────────────────────────────────────────┤
│ Status Summary Strip                                             │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Pending  │  Working  │  Done  │  Failed                         │
│  ┌───┐    │  ┌───┐    │  ┌───┐  │  ┌───┐                         │
│  │   │    │  │   │    │  │   │  │  │   │                         │
│  └───┘    │  └───┘    │  └───┘  │  └───┘                         │
│  ┌───┐    │           │  ┌───┐  │                                │
│  │   │    │           │  │   │  │                                │
│  └───┘    │           │  └───┘  │                                │
│           │           │         │                                │
├──────────────────────────────────────────────────────────────────┤
│ Activity (collapsible)                                           │
└──────────────────────────────────────────────────────────────────┘
```

右侧可滑入 Task Detail 面板（380px）。

### 1. PageHeader

```
左: "Cowork" 标题
右: Runtime 状态条 | [👑 Auto] [+ Task] [+ Worker] [⟳]
```

- Runtime 状态条：`● Claude v4 ✓  ● OpenCode ✓`，紧凑内联
- 👑 Auto：切换监督模式，开启时 `bg-success/80 text-white`
- + Task：快捷键 C
- + Worker：打开 Worker 配置 dialog
- ⟳：手动刷新

### 2. Status Summary Strip

```
● 2 Workers (1 active)  ·  3 Pending  ·  1 Working  ·  5 Done  ·  0 Failed
```

- `bg-muted/30 border-b border-border/50`
- Working 黄色、Done 蓝色、Failed 红色、Pending 灰色

### 3. Board View

四列看板，`flex` 布局均分，`flex-1 min-w-[220px]`，容器 `overflow-x-auto`。

#### 列头

```
● Working  2
```

颜色系统：

| 状态 | 列头色点 | 列背景 |
|------|---------|--------|
| Pending | `bg-muted-foreground` | `bg-muted/30` |
| Working | `bg-warning` | `bg-warning/5` |
| Done | `bg-info` | `bg-info/5` |
| Failed | `bg-destructive` | `bg-destructive/5` |

#### Task Card

三层结构（参考 Multica BoardCardContent）：

```
┌────────────────────────────────────┐
│  CW-07                             │  Row 1: ID (text-xs text-muted-foreground)
│                                    │
│  Add unit tests for auth           │  Row 2: Title (text-sm font-medium line-clamp-2)
│  middleware                        │
│                                    │
│  ▎▎ medium  🤖 Claude-01  68%     │  Row 3: Priority + Worker + Progress
│  deps: CW-05                      │  Row 4: Dependencies (可选)
└────────────────────────────────────┘
```

**卡片样式**：
```css
rounded-lg border-[0.5px] border-border bg-card py-3 px-2.5
shadow-[0_3px_6px_-2px_rgba(0,0,0,0.02)]
hover: border-accent bg-accent/50
```

**Priority Badge（柱状图）**：

| Priority | 柱数 | 样式 |
|----------|------|------|
| urgent | 4 | `bg-red-500/10 text-red-400` |
| high | 3 | `bg-orange-500/10 text-orange-400` |
| medium | 2 | `bg-yellow-500/10 text-yellow-400` |
| low | 1 | `bg-muted text-muted-foreground` |

**Worker 展示**：
- 已分配：`🤖 WorkerName` + 进度或状态
- 未分配：不显示

**Working 状态卡片** — 底部内嵌 Mini Output：
```
┌──────────────────────────────────┐
│ border-info/20 bg-info/5         │
│ ▸ Read src/auth/middleware       │
│ ▸ Write jwt.ts                   │
│ ▸ Test 12/15 ✓         [▼]     │
└──────────────────────────────────┘
```

**Failed 状态卡片** — 显示错误 + Retry 按钮：
```
✗ timeout
[↻ Retry]
```

**Done 状态卡片** — 文字稍淡，显示完成时间：
```
✓ done · 5m
```

### 4. Task Detail（右侧滑入面板，380px）

点击卡片展开，`border-l border-border`：

```
← Back     CW-05              [⋮]

Implement auth module with JWT

Status     [Working ▾]
Priority   [High ▾]
Worker     [🤖 Claude-01 ▾]
Created    Apr 27, 10:25
Deps       CW-03 (done)

── Description ──────────────────
Implement JWT-based auth...

── Actions ──────────────────────
[▶ Execute] [⏸ Pause] [↻ Retry]

── Output ───────────────────────
> Analyzing codebase...
> Created auth middleware
> Running tests: 12/15 ✓
              [Expand Full ▸]

── Activity ─────────────────────
10:30 Claude-01 started
10:28 Assigned → Claude-01
10:25 Task created

── Danger ───────────────────────
[Delete Task]
```

**功能**：
- Status/Priority/Worker 是内联下拉选择器（点击即编辑）
- Execute 弹出内联 command 输入框
- Output 区展示 `CoworkGetTaskOutputHistory` 数据
- Activity 展示该 Task 相关的活动记录
- Delete 带确认

### 5. Create Task Dialog

```
┌────────────────────────────────────┐
│  New Task                   [✕]   │
│  ──────────────────────────────── │
│  Title *                           │
│  [____________________________]    │
│                                    │
│  Description                       │
│  [                              ]  │
│                                    │
│  Priority [medium▾]  Assign [none▾]│
│                                    │
│  Depends on                        │
│  ☑ CW-05 Implement auth           │
│  ☐ CW-06 Add tests                │
│                                    │
│          [Create Task]             │
└────────────────────────────────────┘
```

- Title 必填，Enter 提交
- Priority 和 Assignee 并排
- Assignee 列出所有 Worker（idle/working 分组）
- Dependencies 列出 pending/working 的 Task

### 6. Create Worker Dialog

```
┌────────────────────────────────────┐
│  New Worker                 [✕]   │
│  ──────────────────────────────── │
│  Name                              │
│  [____________________________]    │
│                                    │
│  Runtime  ──────────────────────── │
│  ┌──────────┐ ┌──────────┐        │
│  │ 🟢 Claude│ │ 🟢 Open  │        │
│  │   v4.0   │ │  Code    │        │
│  └──────────┘ └──────────┘        │
│  ┌──────────┐ ┌──────────┐        │
│  │ ⚫ Aider │ │ ⚫ Cursor│        │
│  └──────────┘ └──────────┘        │
│                                    │
│  Custom Command (optional)         │
│  [____________________________]    │
│                                    │
│  Quick Presets                     │
│  [Standard 3c/300s/3r]            │
│  [Quick     1c/120s/1r]           │
│  [Power     5c/600s/5r]           │
│                                    │
│  Concurrency [3]  Timeout [300]s   │
│  Max Retries  [3]                  │
│                                    │
│  Capabilities                      │
│  [frontend] [backend] [debugging]  │
│  [testing]  [review]  [refactor]   │
│                                    │
│  Role (optional)                   │
│  [____________________________]    │
│                                    │
│          [Create Worker]           │
└────────────────────────────────────┘
```

**关键设计**：
1. Runtime 选择器：卡片式，显示检测状态（🟢/⚫），选择后自动填入 Name
2. Quick Presets：三个按钮，一键配置 concurrency/timeout/maxRetries
3. Capabilities：Tag 选择器
4. Custom Command：可选，覆盖默认启动命令

### 7. Activity Bar（底部可折叠）

```
▸ Activity (12) · Last: Claude-01 started task CW-05
─── 展开后 ───
10:30  🤖 Claude-01  started task CW-05
10:28  👤 You         assigned CW-05 → Claude-01
10:25  👤 You         created task CW-05
```

- 默认收起一行
- 最大高度 200px，可滚动

## 交互操作映射

| 用户操作 | 触发方式 | 调用 RPC |
|---------|---------|---------|
| 创建 Task | `+ Task` / 快捷键 C | `CoworkCreateTaskCommand` |
| 创建 Worker | `+ Worker` 按钮 | `CoworkRegisterWorkerCommand` |
| 编辑 Task | 点击卡片 → 右侧面板内编辑 | `CoworkUpdateTaskCommand` |
| 分配 Worker | Task Detail 的 Worker 下拉 | `CoworkUpdateTaskCommand` |
| 执行 Task | Task Detail 的 Execute | `CoworkExecuteTaskCommand` |
| 暂停 Task | Task Detail / 卡片按钮 | `CoworkPauseTaskCommand` |
| 恢复 Task | Task Detail / 卡片按钮 | `CoworkResumeTaskCommand` |
| 重试 Task | Failed 卡片的 Retry | `CoworkRetryTaskCommand` |
| 删除 Task | Task Detail 底部 | `CoworkDeleteTaskCommand` |
| 删除 Worker | Worker 管理（右键/长按） | `CoworkDeleteWorkerCommand` |
| 查看输出 | 点击 Task → Output 区 | `CoworkGetTaskOutputHistoryCommand` |
| 切换监督 | `👑 Auto` 按钮 | `startSupervision/stopSupervision` |
| 检测 Runtime | 自动 + 页面加载 | `CoworkDetectRuntimesCommand` |
| 刷新数据 | `⟳` 按钮 | `refreshAllData` |

## 组件文件规划

### 新建文件

| 文件 | 组件 | 说明 |
|------|------|------|
| `frontend/app/view/cowork/board-view.tsx` | `BoardView` | 看板容器（列布局） |
| `frontend/app/view/cowork/board-column.tsx` | `BoardColumn` | 单列（头部 + 卡片列表） |
| `frontend/app/view/cowork/board-card.tsx` | `BoardCard` | 任务卡片 |
| `frontend/app/view/cowork/mini-output.tsx` | `MiniOutput` | 卡片内嵌 Worker 输出 |
| `frontend/app/view/cowork/task-detail.tsx` | `TaskDetail` | 右侧滑入面板 |
| `frontend/app/view/cowork/create-task-dialog.tsx` | `CreateTaskDialog` | 创建 Task 弹窗 |
| `frontend/app/view/cowork/create-worker-dialog.tsx` | `CreateWorkerDialog` | 创建 Worker 弹窗 |
| `frontend/app/view/cowork/runtime-bar.tsx` | `RuntimeBar` | Runtime 状态条 |
| `frontend/app/view/cowork/status-strip.tsx` | `StatusStrip` | 状态摘要条 |
| `frontend/app/view/cowork/activity-bar.tsx` | `ActivityBar` | 活动日志 |
| `frontend/app/view/cowork/priority-badge.tsx` | `PriorityBadge` | 优先级柱状图 |
| `frontend/app/view/cowork/assignee-picker.tsx` | `AssigneePicker` | Worker 选择下拉 |

### 修改文件

| 文件 | 变更 |
|------|------|
| `frontend/app/view/cowork/cowork.tsx` | **重写**：主容器改为 Board 布局 |
| `frontend/app/view/cowork/cowork-model.ts` | **微调**：新增 board 相关 atom |

### 删除文件

| 文件 | 替换为 |
|------|--------|
| `frontend/app/view/cowork/worker-config-dialog.tsx` | `create-worker-dialog.tsx` |
| `frontend/app/view/cowork/runtime-detection-panel.tsx` | `runtime-bar.tsx` |
| `frontend/app/view/cowork/task-output-history.tsx` | 集成到 `task-detail.tsx` + `mini-output.tsx` |

## 实施计划

### Phase 1：核心骨架（Board 布局 + 卡片）

1. 重写 `cowork.tsx` 主容器
2. 新建 `board-view.tsx` + `board-column.tsx`（看板列布局）
3. 新建 `board-card.tsx`（任务卡片渲染）
4. 新建 `status-strip.tsx`（状态摘要条）
5. 新建 `runtime-bar.tsx`（Runtime 状态条）

### Phase 2：交互功能（弹窗 + 详情面板）

6. 新建 `create-task-dialog.tsx`
7. 新建 `create-worker-dialog.tsx`
8. 新建 `task-detail.tsx`（右侧滑入面板）
9. 新建 `priority-badge.tsx`
10. 新建 `assignee-picker.tsx`

### Phase 3：增强功能（实时输出 + 活动日志）

11. 新建 `mini-output.tsx`（Working 卡片内嵌输出）
12. 新建 `activity-bar.tsx`（底部活动日志）
13. 删除旧文件，清理引用
14. 完整测试所有 RPC 调用路径

## 约束

- 使用 Tailwind CSS（语义 token：`text-muted-foreground`、`bg-card`、`border-border`）
- 使用 Jotai atoms 进行状态管理（不直接调用 model 方法获取渲染数据）
- 不修改 Go 后端代码，仅修改前端
- 保留 `cowork-model.ts` 中的 singleton pattern
- Worker 创建时必须关联一个终端 block（`BlockId` + `TabId`）
- 遵循项目约定：`@/` imports、named exports、4-space indent、lowercase filenames

## 验收标准

1. Board 视图正确按状态分列显示所有 Task
2. Task 卡片正确显示 ID、标题、Priority、Worker、进度
3. 点击卡片展开 Task Detail 面板，可编辑所有字段
4. Create Task / Create Worker 弹窗功能完整
5. Runtime 检测状态正确显示
6. Status Strip 数字与实际数据一致
7. Working 状态卡片内嵌 Mini Output 实时显示
8. Activity Bar 正确显示活动记录
9. 自动监督模式开关正常工作
10. 所有现有 RPC 调用路径无回归
11. TypeScript 类型检查通过（`task check:ts`）
