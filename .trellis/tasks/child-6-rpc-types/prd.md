# Phase1C: RPC type definitions in wshrpctypes.go

> Parent: Refactor cowork to team - AI manages AI team with WaveAI as manager, Member templates with Worker instances, full rename cowork→team

## Shared Context

# Refactor Cowork → Team: AI Manages AI Team

## Overview

将现有 cowork 功能重构为通用的 "team" 系统。核心理念: **"We are all a team!"** — WaveAI 作为管理者,协调一组具备不同能力的 AI 团队成员(Member),根据任务需求动态分配工作。

现有 cowork 的 Worker 是终端中运行的 AI CLI(claude/opencode/cursor/aider),重构后保留这一形态。关键升级在于引入 **Member(模板) → Worker(实例)** 的分身机制:一个 Member 可以 fork 出多个 Worker 并行工作,任务历史统一归属于 Member。

全面破坏性重命名:所有 `cowork` → `team`(Go 包名、RPC 命令、DB 表名、前端文件名)。

## Design Decisions

| 决策项 | 结论 |
|--------|------|
| Agent 形态 | 仅终端 CLI — Worker 在终端 block 中运行 AI CLI |
| 管理层次 | 两层: WaveAI(Manager) → Workers; Member 自行处理 sub-agent(利用 CLI 工具能力) |
| 团队模型 | 单池共享: Member(模板) + Worker(运行实例/分身) |
| 配置管理 | 文件模板 + UI 表单,两者同步; 全局模板库 + 项目级覆盖 |
| 调度策略 | AI 调度 + 用户可覆盖 |
| 分身上限 | 每个 Member 可配置,默认 3 |
| 命名 | 全面重命名 cowork→team |
| 兼容性 | 破坏性重写,不保留旧数据 |

## Core Concepts

### Member (模板)
具备一定能力的 AI 角色定义,是模板而非运行实例。

属性(借鉴 gentle-ai、Claude Code subagent、CrewAI 但不照搬):

| 属性 | 说明 | 参考 |
|------|------|------|
| **Name** | 成员名称(如 "Go后端开发", "前端设计") | — |
| **Tool** | CLI 工具类型(claude/opencode/cursor/aider/custom) | Claude Code AgentID |
| **Description** | 成员能力简述(供 WaveAI 调度参考 + LLM 选择) | Claude Code `description` |
| **Persona** | 系统提示词。短文本可内联,长文本用 `personaPath` 引用外部 .md 文件 | gentle-ai Persona Prompts |
| **PersonaPath** | 外部 persona .md 文件路径(相对于配置文件目录)。优先级高于 `Persona` | OpenCode `{file:...}` |
| **Skills** | 技能名称列表(如 go-testing, react-patterns)。内容存储在全局/项目 skill 目录的 .md 文件中 | gentle-ai Skills Registry + Claude Code `skills` |
| **McpServers** | MCP 外部工具服务配置(标准格式: type/command/args/env 或 type/url/headers) | Claude Code `mcpServers` + gentle-ai MCP |
| **Capabilities** | 工具权限白名单(如 Read, Write, Edit, Bash, Glob, Grep)。未列出 = 全部允许 | Claude Code `tools`/`disallowedTools` |
| **Model** | 可选的模型指定(provider/model 格式,如 anthropic/claude-sonnet)。覆盖默认 | Claude Code `model` + gentle-ai ModelAssignment |
| **MaxConcurrency** | 最大并发 Worker 数(默认 3) | — |
| **MaxRetries** | 单任务最大重试次数(默认 3) | — |
| **Memory** | 记忆模式: none(无持久化) / session(会话内) / persistent(跨会话,借鉴 engram) | Claude Code `memory` + gentle-ai Engram |
| **CustomCmd** | 自定义启动命令(仅 tool=custom 时) | — |
| **Color** | UI 标识颜色(多 Worker 并行时视觉区分) | Claude Code `color` |

### Persona 文件引用机制

**问题**: 长格式 markdown persona 直接嵌入 YAML 容易导致缩进/格式解析错误,且内容注入到不同 CLI 工具时格式要求各异。

**方案**: 支持 `persona`(内联短文本) 和 `personaPath`(外部 .md 文件) 两种方式,`personaPath` 优先。

```yaml
# 短文本内联
members:
  - name: reviewer
    persona: "You are a code reviewer. Focus on correctness."

# 长文本外部引用(推荐)
members:
  - name: go-backend
    personaPath: ./personas/go-backend.md  # 相对于配置文件目录
```

**Persona .md 文件建议结构**(借鉴 gentle-ai Gentleman persona):

```markdown
## Rules
- 硬性规则列表

## Personality
- 人格描述

## Tone
- 语气风格

## Expertise
- 专业领域

## Behavior
- 行为准则
```

**注入策略**(Fork Worker 时根据 Tool 类型选择):

| CLI 工具 | 注入方式 | 参考 |
|----------|---------|------|
| Claude Code | 注入到 Worker block 的 CLAUDE.md `<!-- team:persona -->` section | gentle-ai MarkdownSections |
| OpenCode | 注入到 Worker block 的 AGENTS.md (替换整个文件) | gentle-ai FileReplace |
| 其他 | 作为首次 prompt 发送(无原生注入能力) | — |

### Skills 存储与注入

Skills 以**目录方式**存储,每个 skill 是一个文件夹,包含 `SKILL.md` 入口文件和可选的补充文档/脚本。这是 Agent CLI 的通用作法(如 `~/.claude/skills/go-testing/SKILL.md`)。

**双模式设计**: Team 有自己的技能库,Fork Worker 时通过软链接映射到 Agent CLI 的原生 skills 目录。

#### 技能库层级

| 层级 | 路径 | 说明 |
|------|------|------|
| **Team 全局库** | `~/.waveterm/team-skills/{skill-name}/` | Team 系统管理的技能,包含 SKILL.md + 任意补充文件 |
| **项目级库** | `.wave/skills/{skill-name}/` | 项目专用技能,覆盖全局库同名技能 |
| **Agent CLI 原生** | `~/.claude/skills/` 或 `~/.config/opencode/skills/` | CLI 工具原生加载位置 |

#### 注入流程(Fork Worker 时)

```
1. 解析 Member.skills 列表 (如 ["go-testing", "api-design"])
2. 按优先级查找技能源:
   a. 项目级: .wave/skills/{name}/
   b. Team 全局库: ~/.waveterm/team-skills/{name}/
   c. Agent CLI 原生目录已有 → 跳过(用户自行管理的技能)
3. 对找到的技能,创建软链接到 Worker 对应的 CLI 原生目录:
   - claude:   ~/.claude/skills/{name} → team-skills/{name}
   - opencode: ~/.config/opencode/skills/{name} → team-skills/{name}
4. Recycle Worker 时,清理对应的软链接
```

**注意**: 软链接是**幂等**的(已存在相同目标则跳过),不会覆盖用户自行管理的技能(除非同名覆盖)。

#### 示例

```
# Team 全局技能库
~/.waveterm/team-skills/
  ├── go-testing/
  │   ├── SKILL.md
  │   └── strict-tdd.md
  └── api-design/
      ├── SKILL.md
      └── patterns.md

# Fork Worker (tool=claude) 时创建软链接
~/.claude/skills/go-testing → ~/.waveterm/team-skills/go-testing
~/.claude/skills/api-design → ~/.waveterm/team-skills/api-design

# Agent CLI (claude code) 自动发现并加载
```

### MCP 注入策略

不同 CLI 工具需要不同的 MCP 配置格式(借鉴 gentle-ai 的 4 种 MCP 策略):

| CLI 工具 | MCP 配置位置 | 格式 |
|----------|-------------|------|
| Claude Code | `~/.claude/mcp/{server-name}.json` | 独立 JSON 文件 |
| OpenCode | AGENTS.md 的 settings merge 或专用 MCP 配置 | JSON overlay |
| 其他 | CLI 工具的标准 MCP 配置路径 | 各异 |

**Member McpServers 配置格式**(遵循 MCP 标准):
```yaml
mcpServers:
  - name: context7
    type: stdio
    command: npx
    args: ["-y", "@upstash/context7-mcp"]
  - name: playwright
    type: stdio
    command: npx
    args: ["-y", "@playwright/mcp@latest"]
```

### Worker (实例/分身)
Member 的运行实例,绑定到一个终端 block。

- 由 WaveAI 根据任务需求 fork 产生
- 有独立的终端 block、运行状态、当前任务
- 完成任务后可回收或保持待命
- 任务历史和活动记录归属于 Member
- **心跳检测**: 后端定期检查 Worker 终端进程状态,超时标记为 offline
- **自动恢复**: offline Worker 可被 WaveAI 重新激活或由新 Worker 替代

### Task (任务)
由用户或 WaveAI 创建的工作单元。

- 状态: pending → assigned → working → done/failed/paused
- 优先级: low/medium/high/urgent
- 支持暂停/恢复/重试
- 由 WaveAI 自动分配给最合适的 Member Worker,用户可覆盖
- **任务依赖**: 支持 `dependsOn` 字段,指定前置任务 ID 列表
- **输出收集**: Worker 终端的输出自动关联到 Task,可通过 `team_get_task_output` 获取

---

## Phase 1: Backend Foundation

**目标**: 建立 team 后端数据层和 RPC 骨架,可独立测试和验证。

### 1.1 DB Schema 设计

新建迁移文件 `0000015_team.up.sql`:

```sql
-- Team Members (模板)
CREATE TABLE team_members (
    member_id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    tool TEXT NOT NULL DEFAULT 'claude',  -- claude/opencode/cursor/aider/custom
    custom_cmd TEXT DEFAULT '',
    description TEXT DEFAULT '',           -- 能力简述(供调度参考)
    persona TEXT DEFAULT '',               -- 内联系统提示词(短文本)
    persona_path TEXT DEFAULT '',          -- 外部 .md 文件路径(优先级高于 persona)
    skills TEXT DEFAULT '[]',              -- JSON 数组: ["go-testing", "debugging"]
    mcp_servers TEXT DEFAULT '[]',         -- JSON 数组: [{name, type, command, args, url}]
    capabilities TEXT DEFAULT '[]',        -- JSON 数组: ["Read","Write","Edit","Bash","Glob","Grep"]
    model TEXT DEFAULT '',                 -- provider/model 格式(如 anthropic/claude-sonnet)
    max_concurrency INTEGER DEFAULT 3,     -- 最大并发 Worker 数
    max_retries INTEGER DEFAULT 3,         -- 单任务最大重试次数
    memory TEXT DEFAULT 'session',         -- none/session/persistent
    color TEXT DEFAULT '',                 -- UI 标识颜色(如 "#3B82F6")
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Team Workers (运行实例)
CREATE TABLE team_workers (
    worker_id TEXT PRIMARY KEY,
    member_id TEXT NOT NULL REFERENCES team_members(member_id) ON DELETE CASCADE,
    name TEXT NOT NULL,                   -- 运行时名称(如 "Go后端-1", "Go后端-2")
    status TEXT NOT NULL DEFAULT 'idle',  -- idle/working/offline/error
    assigned_task_id TEXT DEFAULT '',
    block_id TEXT DEFAULT '',
    tab_id TEXT DEFAULT '',
    pid INTEGER DEFAULT 0,                -- 终端进程 PID(心跳检测用)
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_heartbeat DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Team Tasks
CREATE TABLE team_tasks (
    task_id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT DEFAULT '',
    priority TEXT NOT NULL DEFAULT 'medium',  -- low/medium/high/urgent
    status TEXT NOT NULL DEFAULT 'pending',   -- pending/assigned/working/done/failed/paused
    assigned_member_id TEXT DEFAULT '',        -- 分配给的 Member
    assigned_worker_id TEXT DEFAULT '',        -- 实际执行的 Worker
    depends_on TEXT DEFAULT '[]',              -- JSON 数组: 前置任务 ID 列表
    result TEXT DEFAULT '',
    error TEXT DEFAULT '',
    output_history TEXT DEFAULT '[]',          -- JSON 数组: [{timestamp, type, content}] 输出历史
    progress INTEGER DEFAULT 0,
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 3,
    next_retry_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME
);

-- Team Activity Log
CREATE TABLE team_activity (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT DEFAULT '',
    worker_id TEXT DEFAULT '',
    member_id TEXT DEFAULT '',
    type TEXT NOT NULL,                   -- created/assigned/started/completed/failed/retried/forked/recycled
    description TEXT DEFAULT '',
    meta TEXT DEFAULT '{}',               -- JSON
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 索引
CREATE INDEX idx_team_tasks_status ON team_tasks(status);
CREATE INDEX idx_team_tasks_priority ON team_tasks(priority);
CREATE INDEX idx_team_tasks_status_priority ON team_tasks(status, priority);
CREATE INDEX idx_team_tasks_member ON team_tasks(assigned_member_id);
CREATE INDEX idx_team_workers_status ON team_workers(status);
CREATE INDEX idx_team_workers_member ON team_workers(member_id);
CREATE INDEX idx_team_activity_task ON team_activity(task_id);
CREATE INDEX idx_team_activity_worker ON team_activity(worker_id);
CREATE INDEX idx_team_activity_member ON team_activity(member_id);
CREATE INDEX idx_team_activity_created ON team_activity(created_at);
```

### 1.2 Go Package: pkg/team/

**pkg/team/team_types.go** — 类型定义(从 wshrpctypes.go 移入独立包):

```go
// TeamMember — AI 团队成员模板
type TeamMember struct {
    MemberID       string   `json:"memberId"`
    Name           string   `json:"name"`
    Tool           string   `json:"tool"`              // claude/opencode/cursor/aider/custom
    CustomCmd      string   `json:"customCmd"`
    Description    string   `json:"description"`
    Persona        string   `json:"persona"`           // 内联系统提示词(短文本)
    PersonaPath    string   `json:"personaPath"`       // 外部 .md 文件路径(优先级高于 Persona)
    Skills         []string `json:"skills"`             // 技能名称列表
    McpServers     []MCPConfig `json:"mcpServers"`     // MCP 服务配置(结构化)
    Capabilities   []string `json:"capabilities"`       // 工具权限白名单
    Model          string   `json:"model"`              // provider/model 格式(可选)
    MaxConcurrency int      `json:"maxConcurrency"`     // 最大并发 Worker
    MaxRetries     int      `json:"maxRetries"`         // 单任务最大重试次数
    Memory         string   `json:"memory"`             // none/session/persistent
    Color          string   `json:"color"`              // UI 标识颜色
    CreatedAt      string   `json:"createdAt"`
    UpdatedAt      string   `json:"updatedAt"`
}

// MCPConfig — MCP 服务器配置(遵循 MCP 标准)
type MCPConfig struct {
    Name    string   `json:"name"`              // 服务器名称
    Type    string   `json:"type"`              // stdio | http
    Command string   `json:"command,omitempty"` // stdio 专用
    Args    []string `json:"args,omitempty"`    // stdio 专用
    Env     MapStr   `json:"env,omitempty"`     // 环境变量
    URL     string   `json:"url,omitempty"`     // http 专用
    Headers MapStr   `json:"headers,omitempty"` // http 专用
}

// TeamWorker — Member 的运行实例
type TeamWorker struct {
    WorkerID       string `json:"workerId"`
    MemberID       string `json:"memberId"`
    Name           string `json:"name"`              // 运行时名称(如 "Go后端-1")
    Status         string `json:"status"`            // idle/working/offline/error
    AssignedTaskID string `json:"assignedTaskId"`
    BlockID        string `json:"blockId"`
    TabID          string `json:"tabId"`
    PID            int    `json:"pid"`               // 终端进程 PID
    CreatedAt      string `json:"createdAt"`
    UpdatedAt      string `json:"updatedAt"`
    LastHeartbeat  string `json:"lastHeartbeat"`
}

// TeamTask — 任务
type TeamTask struct {
    TaskID           string   `json:"taskId"`
    Title            string   `json:"title"`
    Description      string   `json:"description"`
    Priority         string   `json:"priority"`         // low/medium/high/urgent
    Status           string   `json:"status"`            // pending/assigned/working/done/failed/paused
    AssignedMemberID string   `json:"assignedMemberId"`
    AssignedWorkerID string   `json:"assignedWorkerId"`
    DependsOn        []string `json:"dependsOn"`         // 前置任务 ID 列表
    Result           string   `json:"result"`
    Error            string   `json:"error"`
    OutputHistory    []TaskOutput `json:"outputHistory"` // 输出历史
    Progress         int      `json:"progress"`
    RetryCount       int      `json:"retryCount"`
    MaxRetries       int      `json:"maxRetries"`
    NextRetryAt      string   `json:"nextRetryAt"`
    CreatedAt        string   `json:"createdAt"`
    UpdatedAt        string   `json:"updatedAt"`
    CompletedAt      string   `json:"completedAt"`
}

// TaskOutput — 任务输出条目
type TaskOutput struct {
    Timestamp string `json:"timestamp"`
    Type      string `json:"type"`      // stdout/stderr/result
    Content   string `json:"content"`
}

// TeamActivity — 活动日志
type TeamActivity struct { ... }
```

**pkg/team/team_db.go** — 数据访问层:
- Member CRUD: Create/Get/Update/Delete/List
- Worker CRUD: Create/Get/Update/Delete/List (+ 按 member_id 过滤)
- Task CRUD: Create/Get/Update/Delete/List (+ 按 status/priority/member 过滤)
- Activity: Add/List (+ 按 task/worker/member 过滤,滚动窗口)
- 状态机验证: Task 和 Worker 的合法状态转换
- WPS 事件发布: Event_TeamTaskUpdate / Event_TeamWorkerUpdate / Event_TeamMemberUpdate

**pkg/team/team_state.go** — 状态机:
```go
// 合法 Task 状态转换
var validTaskTransitions = map[string][]string{
    "pending":   {"assigned", "cancelled"},
    "assigned":  {"working", "pending", "cancelled"},
    "working":   {"done", "failed", "paused"},
    "paused":    {"working", "cancelled"},
    "failed":    {"working"},  // retry
    "cancelled": {},           // terminal state
    "done":      {},           // terminal state
}

// 合法 Worker 状态转换
var validWorkerTransitions = map[string][]string{
    "idle":    {"working", "offline"},
    "working": {"idle", "error", "offline"},
    "error":   {"idle", "offline"},
    "offline": {"idle"},
}
```

**pkg/team/team_fork.go** — Worker 分身逻辑:
```go
// ForkWorker 为 Member 创建新的 Worker 实例
// 1. 检查 MaxConcurrency (当前活跃 Worker 数 < MaxConcurrency)
// 2. 生成运行时名称(如 "Go后端-1", "Go后端-2", 递增编号)
// 3. 创建终端 block 并注入 Member 的 Persona/Skills/MCP 配置
// 4. 记录 Activity(forked)
func ForkWorker(memberID string) (*TeamWorker, error)
```

**pkg/team/team_inject.go** — CLI 配置注入(新增):
```go
// InjectWorkerConfig 将 Member 配置注入到 Worker 的终端 CLI 工具
// 包括: Persona 注入 + Skills 软链接映射 + MCP 配置
func InjectWorkerConfig(worker *TeamWorker, member *TeamMember) error

// loadPersona 加载 persona 内容(处理 personaPath 引用)
func loadPersona(member *TeamMember, configDir string) (string, error)

// linkSkills 为 Worker 的 CLI 工具创建 skills 软链接
// 查找优先级: 项目级 > Team全局库 > 跳过(CLI原生已有)
// 软链接: {cli-skills-dir}/{name} → {team-skills-source}/{name}
func linkSkills(skills []string, tool string, projectDir string) error

// unlinkSkills 清理 Worker 的 skills 软链接(Recycle 时调用)
func unlinkSkills(skills []string, tool string) error
```

**pkg/team/team_heartbeat.go** — Worker 心跳检测(新增):
```go
// CheckWorkerHealth 检查所有活跃 Worker 的终端进程状态
// 超时(30s 无心跳) → 标记 offline → 发布 WPS 事件
func CheckWorkerHealth() error
```

### 1.3 RPC 命令 (pkg/wshrpc/wshrpctypes.go)

全面重命名 CoworkXxx → TeamXxx, 新增 Member 管理:

**Member CRUD (新增):**
- `TeamCreateMemberCommand` / `TeamGetMemberCommand` / `TeamUpdateMemberCommand` / `TeamDeleteMemberCommand` / `TeamListMembersCommand`

**Worker 管理 (重写):**
- `TeamRegisterWorkerCommand` → `TeamForkWorkerCommand` (分身语义)
- `TeamGetWorkerCommand` / `TeamUpdateWorkerCommand` / `TeamDeleteWorkerCommand` / `TeamListWorkersCommand`
- `TeamRecycleWorkerCommand` (回收 Worker,释放终端 block)

**Task CRUD (重命名):**
- `TeamCreateTaskCommand` / `TeamGetTaskCommand` / `TeamUpdateTaskCommand` / `TeamDeleteTaskCommand` / `TeamListTasksCommand`

**执行/生命周期 (重命名):**
- `TeamExecuteTaskCommand` (创建终端 block + 发送命令)
- `TeamPauseTaskCommand` / `TeamResumeTaskCommand` / `TeamRetryTaskCommand`
- `TeamGetTaskOutputHistoryCommand`

**状态 (重命名):**
- `TeamGetStatusCommand` (聚合统计: Members/Workers/Tasks)

**运行时 (重命名):**
- `TeamDetectRuntimesCommand`

**Activity (重命名):**
- `TeamAddActivityCommand` / `TeamListActivityCommand`

### 1.4 AI 工具重写 (pkg/aiusechat/tools_team.go)

替换现有 9 个 cowork 工具为 team 工具:

| 旧工具 | 新工具 | 说明 |
|--------|--------|------|
| cowork_create_worker | team_fork_worker | 从 Member 分身出 Worker(检查 MaxConcurrency) |
| cowork_list_workers | team_list_workers | 列出所有活跃 Worker(含状态、绑定 Member、当前 Task) |
| — | team_list_members | 列出所有 Member 模板(含 Skills、Capabilities、可用性) |
| — | team_create_member | 创建 Member 模板(含 Persona/Skills/MCP 注入) |
| — | team_update_member | 更新 Member 属性 |
| — | team_delete_member | 删除 Member(级联删除 Workers) |
| cowork_create_task | team_create_task | 创建任务(支持 priority、dependsOn) |
| cowork_assign_task | team_assign_task | 分配任务给 Member(自动 fork Worker,检查依赖) |
| cowork_update_task | team_update_task | 更新任务状态/进度 |
| cowork_get_status | team_get_status | 获取团队状态(聚合统计: Members/Workers/Tasks) |
| cowork_execute_task | team_execute_task | 执行任务(创建终端+注入配置+发送命令) |
| cowork_terminate_worker | team_recycle_worker | 回收 Worker(释放终端 block,记录 Activity) |
| cowork_send_prompt | team_send_prompt | 向 Worker 终端发送提示(后续指令) |
| — | team_get_task_output | 获取任务输出历史(收集终端输出) |
| — | team_list_activity | 获取活动日志(按 Member/Worker/Task 过滤) |

### 1.5 System Prompt 重写 (pkg/aiusechat/usechat-prompts.go)

`SystemPromptText_CoworkMode` → `SystemPromptText_TeamMode`:

```
You are the team manager of an AI development team ("We are all a team!").
You coordinate team members by breaking down tasks and assigning them to the right members.
You are a COORDINATOR — delegate real work to team members, don't do it yourself.

## Team Tools

- team_list_members: List all team members with their capabilities, skills, and availability
- team_create_member: Define a new team member (with persona, skills, tools, MCP servers)
- team_fork_worker: Create a worker instance from a member template (fork = clone)
- team_list_workers: List all active worker instances
- team_create_task: Create a new task with title, description, priority, and optional dependencies
- team_assign_task: Assign a task to a member (auto-forks worker if needed, respects maxConcurrency)
- team_execute_task: Start task execution (creates terminal block + sends command)
- team_get_status: Get team overview (members, active workers, pending/working tasks)
- team_recycle_worker: Recycle a worker when its task is done (releases terminal block)
- team_send_prompt: Send a follow-up prompt to a worker's terminal
- team_update_task: Update task status, progress, or details

## Scheduling Strategy

1. **Analyze** the user's request → identify what skills/tools are needed
2. **Check members** (team_list_members) → find the best-fit member by:
   - Skills match (does the member have the required skills?)
   - Tool match (does the member's CLI tool support the needed operations?)
   - Description relevance (does the member's description match the task domain?)
   - Availability (how many workers are already active vs maxConcurrency?)
3. **Break down** complex requests into independent tasks when possible
4. **Assign tasks** respecting dependencies (dependsOn) and priority
5. **Execute** — fork workers, send commands, monitor progress
6. **Collect results** — when workers complete, gather outputs and synthesize
7. **Recycle** — clean up workers when done to free resources

## Key Rules

- Always check team_get_status before creating new workers (respect maxConcurrency)
- If no suitable member exists, suggest creating one with team_create_member
- For independent tasks, fork multiple workers in parallel
- For sequential tasks with dependencies, assign in order, respect dependsOn
- Monitor task status and retry failed tasks (up to maxRetries)
- Report results back to the user with a synthesis of all worker outputs
```

### 1.6 配置文件系统

**配置目录结构**:
```
~/.waveterm/
  ├── team-templates/                # 全局 Member 模板库
  │   ├── go-backend.yaml            # Member 元数据
  │   ├── go-backend.md              # Persona 内容(外部 .md 引用)
  │   ├── frontend-dev.yaml
  │   ├── frontend-dev.md
  │   └── ...
  └── team-skills/                   # Team 全局技能库(目录结构)
      ├── go-testing/                # 技能目录(包含 SKILL.md + 任意文件)
      │   ├── SKILL.md
      │   └── strict-tdd.md
      ├── api-design/
      │   └── SKILL.md
      └── ...

# Agent CLI 原生 skills 目录(Fork Worker 时通过软链接映射):
# Claude Code: ~/.claude/skills/{name} → ~/.waveterm/team-skills/{name}
# OpenCode:    ~/.config/opencode/skills/{name} → ~/.waveterm/team-skills/{name}

.wave/                               # 项目级配置
  ├── team.yaml                      # 项目 Member 定义
  ├── personas/                      # 项目级 Persona 文件
  │   ├── backend.md
  │   └── frontend.md
  └── skills/                        # 项目级技能库(优先级高于全局)
      └── custom-patterns/
          └── SKILL.md
```

**全局模板 YAML 格式** (`~/.waveterm/team-templates/go-backend.yaml`):
```yaml
name: "Go Backend Developer"
description: "Go backend development with testing and API design expertise"
tool: opencode                    # claude/opencode/cursor/aider/custom
model: "anthropic/claude-sonnet"  # 可选: provider/model 格式
color: "#3B82F6"                  # UI 标识颜色

# Persona: 用 personaPath 引用外部 .md 文件(推荐,避免 YAML 解析问题)
personaPath: ./go-backend.md      # 相对于 YAML 文件所在目录

# Skills: 名称列表,内容在 team-skills/ 目录下
skills:
  - go-testing
  - debugging
  - api-design

# MCP 服务器(标准格式)
mcpServers:
  - name: context7
    type: stdio
    command: npx
    args: ["-y", "@upstash/context7-mcp"]

# 工具权限(白名单,未列出 = 全部允许)
capabilities:
  - Read
  - Write
  - Edit
  - Bash
  - Glob
  - Grep

maxConcurrency: 3
maxRetries: 3
memory: session                  # none/session/persistent
```

**项目级覆盖** (`.wave/team.yaml`):
```yaml
# 项目级配置可以: 定义新 Member、覆盖全局模板、或两者混合
members:
  # 方式1: 完整定义(不引用模板)
  - name: project-specific-dev
    tool: claude
    persona: "You are a developer for this specific project..."
    skills: [react, typescript]
    capabilities: [Read, Write, Edit, Bash]

  # 方式2: 覆盖全局模板(只改几个字段)
  - template: go-backend          # 引用全局模板名称
    model: "anthropic/claude-opus" # 覆盖模型
    maxConcurrency: 5             # 覆盖并发数
```

**加载优先级**: 项目级 `.wave/team.yaml` > 全局 `~/.waveterm/team-templates/` > 内置默认

**Persona .md 文件解析规则**:
1. `personaPath` 存在 → 读取文件内容,忽略 `persona` 字段
2. `personaPath` 不存在 → 使用 `persona` 内联文本
3. `personaPath` 路径解析: 以 `./` 开头 = 相对于 YAML 文件目录; 以 `/` 开头 = 绝对路径; 否则 = 相对于 YAML 文件目录
4. 文件读取失败 → fallback 到 `persona` 字段,记录警告日志

---

## Phase 2: Frontend Rewrite

**目标**: 重写前端 UI,从 cowork 视图改为 team 视图。

### 2.1 文件重命名

```
frontend/app/view/cowork/  →  frontend/app/view/team/
  cowork-model.ts          →  team-model.ts (TeamViewModel)
  cowork.tsx               →  team.tsx (TeamView)
  cowork-types.ts          →  team-types.ts
  board-view.tsx           →  board-view.tsx (保留,微调)
  board-column.tsx         →  board-column.tsx (保留,微调)
  board-card.tsx           →  board-card.tsx (保留,微调)
  worker-panel.tsx         →  member-panel.tsx (Member 管理 + Worker 状态)
  worker-sidebar.tsx       →  删除(合并到 member-panel)
  worker-config-dialog.tsx →  删除(合并到 member-panel)
  task-detail.tsx          →  task-detail.tsx (保留,微调)
  其他组件保留,更新引用
```

### 2.2 TeamViewModel (Jotai Singleton)

核心 atoms:
```typescript
// Members
membersAtom: TeamMember[]
// Workers (运行实例)
workersAtom: TeamWorker[]
// Tasks (按状态分组)
pendingTasksAtom / workingTasksAtom / doneTasksAtom / failedTasksAtom: TeamTask[]
// Activity
activityLogAtom: TeamActivity[]
// Supervision
isSupervisingAtom / isProcessingAtom: boolean
// Status (computed)
statusAtom: TeamStatusData
```

关键方法:
- `forkWorker(memberId: string)` — 从 Member 分身 Worker
- `recycleWorker(workerId: string)` — 回收 Worker
- `assignTask(taskId: string, memberId: string)` — 分配任务(自动 fork if needed)
- `executeTask(taskId: string)` — 执行任务
- `startSupervision()` / `stopSupervision()` — 启停 LLM 调度循环

### 2.3 UI 组件

**TeamView**: 三栏布局
- 左侧: Member 列表 + Worker 状态
- 中间: 看板视图(Pending/Working/Done/Failed)
- 右侧: Task Detail 滑入面板

**MemberPanel**: Member 管理面板
- Member 列表(显示名称、工具、并发状态)
- 创建/编辑 Member 表单(Name, Tool, Persona, Skills, MCP, Capabilities)
- 每个 Member 下的活跃 Workers 列表
- 从模板导入/导出

**BoardView**: 保留看板,更新为 team 语义
- Task 卡片显示: assignedMember, assignedWorker, priority, status
- 拖拽分配: 拖 Task 到 Member → 自动 fork Worker + assign

### 2.4 AI Panel 集成

- `cowork-workers-panel.tsx` → `team-panel.tsx`
- CoworkMode → TeamMode toggle
- @mention Workers → @mention Members(自动 fork if needed)

---

## Phase 3: Configuration & Templates

**目标**: 实现配置文件系统,支持模板导入导出。

### 3.1 内置默认模板

提供 4 个内置 Member 模板:
1. **Go Backend Developer**: tool=opencode, skills=[go-testing, debugging]
2. **Frontend Developer**: tool=claude, skills=[react, typescript, tailwind]
3. **Code Reviewer**: tool=claude, capabilities=[read, edit]
4. **General Assistant**: tool=claude, capabilities=[read, write, edit, bash]

### 3.2 模板管理

- 全局模板存储在 `~/.waveterm/team-templates/`
- 项目模板存储在 `.wave/team.yaml`
- UI 中支持: 创建、编辑、删除、导入、导出模板
- Member 创建时可选择从模板导入

### 3.3 Persona 注入

Worker fork 时,将 Member 的 Persona 注入到 CLI 工具:
- **Claude Code**: 注入到 CLAUDE.md 的特定 section
- **OpenCode**: 注入到 AGENTS.md
- **其他**: 作为系统提示词参数

---

## Acceptance Criteria

### Phase 1 (Backend)
- [ ] `pkg/team/` 包完整,包含 team_db.go, team_types.go, team_state.go, team_fork.go, team_inject.go, team_heartbeat.go, team_config.go
- [ ] DB 迁移成功创建 4 张表(team_members, team_workers, team_tasks, team_activity)
- [ ] 所有 RPC 命令(TeamXxx)从 wshserver 到 wshclient 全链路可用
- [ ] 状态机验证阻止非法状态转换
- [ ] Worker 分身逻辑正确检查 MaxConcurrency
- [ ] Skills 软链接注入/清理正常(Fork/Recycle 时)
- [ ] Persona 文件引用(personaPath)解析正常
- [ ] Worker 心跳检测正常运行
- [ ] AI 工具集(tools_team.go)全部注册并可调用(15 个工具)
- [ ] System prompt 正确引导 WaveAI 进行团队管理
- [ ] 配置文件加载(全局 + 项目级)正常工作
- [ ] 测试覆盖率 >= 85%
- [ ] 所有旧 cowork 代码已移除

### Phase 2 (Frontend)
- [ ] `frontend/app/view/team/` 目录完整,所有组件重命名完成
- [ ] TeamViewModel 正确管理 Member/Worker/Task 状态
- [ ] 看板视图显示正确,拖拽分配可用
- [ ] Member 创建/编辑表单包含所有属性
- [ ] AI Panel 中 TeamMode 切换正常
- [ ] @mention Members 可用
- [ ] 所有旧 cowork 前端代码已移除

### Phase 3 (Configuration)
- [ ] 内置 4 个默认模板可加载
- [ ] 全局模板目录创建和读取正常
- [ ] 项目级 `.wave/team.yaml` 覆盖全局模板
- [ ] UI 中模板导入/导出可用

## Technical Constraints

- **DB**: 使用现有 `wstore.GetGlobalDB()` 共享 SQLite 连接
- **RPC**: 新增 Team 命令不影响其他 RPC; 移除所有 Cowork 命令
- **WPS Events**: 新增 Event_TeamTaskUpdate / Event_TeamWorkerUpdate / Event_TeamMemberUpdate, 移除 Event_Cowork*
- **Codegen**: 修改 wshrpctypes.go 后必须运行 `task generate` 更新生成文件
- **Frontend**: 遵循 Jotai singleton 模式,不用 React hooks 在 Model 中
- **Naming**: 4 空格缩进(TS), camelCase, named exports only

## Files to Create/Modify

### New Files
- `pkg/team/team_types.go` — Team 类型定义(TeamMember, TeamWorker, TeamTask, MCPConfig, TaskOutput)
- `pkg/team/team_db.go` — DB CRUD 层
- `pkg/team/team_state.go` — 状态机(Task/Worker 状态转换校验)
- `pkg/team/team_fork.go` — Worker 分身逻辑(MaxConcurrency 检查 + 运行时命名)
- `pkg/team/team_inject.go` — CLI 配置注入(Persona/Skills/MCP 注入到不同 CLI 工具)
- `pkg/team/team_heartbeat.go` — Worker 心跳检测(进程状态监控)
- `pkg/team/team_config.go` — 配置文件加载(全局+项目级 YAML 解析, personaPath 文件引用)
- `pkg/aiusechat/tools_team.go` — AI 工具集(15 个 team_* 工具)
- `db/migrations-wstore/0000015_team.up.sql` — Schema
- `db/migrations-wstore/0000015_team.down.sql` — 回滚
- `frontend/app/view/team/` — 整个目录(从 cowork 重写)

### Modified Files
- `pkg/wshrpc/wshrpctypes.go` — 移除 CoworkXxx, 新增 TeamXxx 类型
- `pkg/wshrpc/wshserver/wshserver.go` — 移除 Cowork handlers, 新增 Team handlers
- `pkg/wshrpc/wshclient/wshclient.go` — 移除 Cowork client, 新增 Team client
- `pkg/wps/wpstypes.go` — 移除 Event_Cowork*, 新增 Event_Team*
- `pkg/aiusechat/usechat.go` — CoworkMode → TeamMode
- `pkg/aiusechat/usechat-prompts.go` — SystemPromptText_CoworkMode → TeamMode
- `pkg/aiusechat/uctypes/uctypes.go` — WaveChatOpts.CoworkMode → TeamMode
- `frontend/app/block/blockregistry.ts` — cowork → team
- `frontend/app/aipanel/` — 多文件 cowork 引用 → team

### Deleted Files
- `pkg/cowork/` — 整个目录
- `pkg/aiusechat/tools_cowork.go`
- `frontend/app/view/cowork/` — 整个目录
- `frontend/app/aipanel/cowork-workers-panel.tsx`

### Preserved Files (不删除)
- `db/migrations-wstore/0000012_cowork.*.sql` — 迁移历史保留,确保已有 DB 可正常迁移

## Task Description

Remove all Cowork* types from pkg/wshrpc/wshrpctypes.go and add Team* types. This includes all Team command/request/response types, TeamMember/TeamWorker/TeamTask TeamXxxCommand structs, and TeamXxxCommandReturn structs. Also update WshRPCServer interface to remove Cowork methods and add Team methods.

## Files to Modify

- pkg/wshrpc/wshrpctypes.go

## Acceptance Criteria

- All functions compile without errors
- Follow existing code patterns (see pkg/team/team_types.go)
- Run `task generate` if modifying RPC types
