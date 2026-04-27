# WaveAI Cowork Mode 优化 PRD

> **版本**: v2.1 (简化版)
> **日期**: 2026-04-27
> **状态**: 待实施
> **基于**: docs/cowork-optimization-design.md v1.0 + 架构审查 + 用户反馈
> 
> **v2.1 变更**: 简化Worker输出历史实现，利用CLI原生session恢复机制

---

## 📋 目录

1. [概述](#概述)
2. [当前状态分析](#当前状态分析)
3. [优化目标](#优化目标)
4. [P0 功能需求](#p0-功能需求)
5. [P1 功能需求](#p1-功能需求)
6. [P2 功能需求](#p2-功能需求)
7. [技术方案](#技术方案)
8. [实施计划](#实施计划)
9. [验收标准](#验收标准)
10. [附录](#附录)

---

## 概述

### 背景

WaveAI Cowork Mode 是一个多AI代理协作系统，通过在终端子块中运行AI CLI工具（Claude Code、OpenCode、Cursor Agent等）来协作完成任务。当前系统已具备基础的Task和Worker管理功能，但存在以下核心问题：

1. **Runtime检测缺失**: 用户不知道系统中有哪些可用的AI CLI
2. **Worker配置繁琐**: 每次创建Worker都要手动配置所有字段
3. **Task分配受限**: 只能通过Supervisor自动分配，无法手动指定
4. **执行控制不足**: 无法暂停/恢复正在运行的Task
5. **历史追溯困难**: Worker输出关闭后无法回顾

### 目标用户

- **开发者**: 使用多个AI工具辅助编程
- **项目经理**: 协调多个AI Agent并行处理任务
- **研究者**: 对比不同AI工具的输出结果

### 核心价值

- **可见性**: 用户能看到Worker的完整执行过程（相比OpenFang/Multica的后台模式）
- **灵活性**: 支持自动和手动两种Task分配方式
- **可追溯**: 保留Worker历史输出，支持会话恢复
- **可扩展**: 支持自定义Worker模板和能力标签

---

## 当前状态分析

### 现有数据结构

#### CoworkWorker（pkg/wshrpc/wshrpctypes.go:1001-1019）

```go
type CoworkWorker struct {
    WorkerId       string  // Worker唯一标识
    Name           string  // Worker显示名称
    Tool           string  // AI CLI工具类型 (claude/opencode/cursor/aider)
    CustomCmd      string  // 自定义命令
    Role           string  // 角色描述
    Desc           string  // 详细描述
    Soul           string  // ⭐ 系统提示词（Instructions）- 已有字段
    Skills         string  // 技能标签（JSON数组）
    McpServers     string  // MCP服务器配置（JSON）
    Status         string  // idle/working/offline/error
    AssignedTask   string  // 当前分配的任务ID
    BlockId        string  // 关联的终端块ID
    TabId          string  // 关联的标签页ID
    CreatedAt      int64   // 创建时间
    LastActiveAt   int64   // 最后活跃时间
    LastOutputHash string  // 输出哈希（用于变化检测）
    ErrorMsg       string  // 错误信息
}
```

**关键发现**: `Soul`字段已存在，可用于存储System Prompt（Instructions），无需新增字段。

#### CoworkTask（pkg/wshrpc/wshrpctypes.go:986-999）

```go
type CoworkTask struct {
    TaskId         string  // Task唯一标识
    Title          string  // Task标题
    Description    string  // 详细描述
    Priority       string  // low/medium/high/urgent
    Status         string  // pending/assigned/working/done/failed
    AssignedWorker string  // 分配的Worker ID
    CreatedAt      int64   // 创建时间
    UpdatedAt      int64   // 更新时间
    CompletedAt    int64   // 完成时间
    Result         string  // 执行结果
    Error          string  // 错误信息
    Progress       string  // 进度信息（0-100或描述性文本）
}
```

### 现有功能

✅ **已实现**:
- Task CRUD（创建、读取、更新、删除）
- Worker注册和管理
- LLM Supervision（每10秒自动分配Task）
- Task Board（4列：pending/working/done/failed）
- Workers列表
- Activity Log
- WPS事件订阅（cowork:taskupdate, cowork:workerupdate）

❌ **缺失**:
- Runtime检测（AI CLI工具检测）
- Worker配置UI（当前只能通过RPC手动创建）
- Task手动分配
- Task执行控制（暂停/恢复/取消）
- Worker输出历史
- Worker模板系统

---

## 优化目标

### 核心原则

1. **保持term块执行模式**: 这是Cowork的独特优势，不改为后台运行
2. **渐进式增强**: 分P0/P1/P2三个优先级实施
3. **充分利用现有字段**: 优先使用Soul/Skills等已有字段
4. **用户友好**: 提供预设模板，降低配置复杂度

### 优化目标矩阵

| 优先级 | 功能模块 | 核心价值 | 实施难度 |
|--------|----------|----------|----------|
| **P0** | Runtime检测 | 解决"不知道有哪些CLI"的问题 | 中 |
| **P0** | Worker配置UI | 简化Worker创建流程 | 中 |
| **P0** | Worker模板 | 减少重复配置，提升效率 | 低 |
| **P0** | Task手动分配 | 支持用户指定Worker | 低 |
| **P1** | Task执行控制 | 暂停/恢复/取消Task | 中 |
| **P1** | Worker输出历史 | 支持会话恢复和历史追溯 | 高 |
| **P1** | UI简化 | 简化AI Panel入口 | 低 |
| **P2** | Task依赖关系 | 支持复杂任务拆分 | 高 |
| **P2** | Worker能力标签 | 更智能的Task分配 | 中 |
| **P2** | Task重试机制 | 提升任务成功率 | 中 |

---

## P0 功能需求

### 1. Runtime检测面板

**优先级**: P0
**目标**: 让用户知道系统中有哪些可用的AI CLI工具

#### 功能描述

在CoworkView顶部新增"Runtime Detection"区域，用户点击"Detect AI CLIs"按钮后，系统自动检测已安装的AI CLI工具（Claude Code、OpenCode、Cursor Agent、Aider）。

#### UI设计

```
┌─────────────────────────────────────────────────┐
│ Cowork Mode                                      │
├─────────────────────────────────────────────────┤
│ 🔍 AI CLI Detection                             │
│ [Detect AI CLIs]                                │
│ ┌───────────────────────────────────────────┐  │
│ │ ● Claude Code   1.2.3  (claude)          │  │
│ │ ● OpenCode     2.4.5  (opencode)         │  │
│ │ ○ Cursor Agent N/A    (not found)        │  │
│ │ ○ Aider        N/A    (not found)        │  │
│ └───────────────────────────────────────────┘  │
└─────────────────────────────────────────────────┘
```

#### 技术实现

**前端组件** (`frontend/app/view/cowork/cowork-runtime-panel.tsx`):
```typescript
export function RuntimeDetectionPanel() {
    const [isDetecting, setIsDetecting] = useState(false);
    const [runtimes, setRuntimes] = useState<AIRuntime[]>([]);

    const handleDetect = async () => {
        setIsDetecting(true);
        try {
            const detected = await RpcApi.CoworkDetectRuntimesCommand(TabRpcClient);
            setRuntimes(detected ?? []);
        } catch (e) {
            console.error("Detection failed:", e);
        } finally {
            setIsDetecting(false);
        }
    };

    return (
        <div className="border border-border/50 rounded bg-base p-3">
            <h3 className="text-sm font-semibold mb-2">🔍 AI CLI Detection</h3>
            <button onClick={handleDetect} disabled={isDetecting}>
                {isDetecting ? "Detecting..." : "Detect AI CLIs"}
            </button>
            {runtimes.map(runtime => (
                <div key={runtime.name}>
                    <span>{runtime.status === "online" ? "●" : "○"}</span>
                    <span>{runtime.displayName}</span>
                    <span>{runtime.version || "N/A"}</span>
                </div>
            ))}
        </div>
    );
}
```

**后端RPC** (`pkg/wshrpc/wshrpctypes.go`):
```go
// CoworkDetectRuntimesCommand 检测系统中可用的AI CLI
type CoworkDetectRuntimesCommand struct {
    Command
}

type AIRuntime struct {
    Name        string `json:"name"`        // claude, opencode, cursor, aider
    DisplayName string `json:"display_name"` // Claude Code, OpenCode...
    Command     string `json:"command"`     // 检测到的完整路径
    Version     string `json:"version"`     // 版本号
    Status      string `json:"status"`      // online, offline
}

type CoworkDetectRuntimesCommandReturn struct {
    Runtimes []AIRuntime `json:"runtimes"`
}

func (c *CoworkDetectRuntimesCommand) Run(ctx context.Context, m *Mailbox, returnValue *CoworkDetectRuntimesCommandReturn) error {
    providers := []struct {
        name        string
        displayName string
        command     string
        flag        string
    }{
        {"claude", "Claude Code", "claude", "--version"},
        {"opencode", "OpenCode", "opencode", "--version"},
        {"cursor", "Cursor Agent", "cursor-agent", "--version"},
        {"aider", "Aider", "aider", "--version"},
    }

    var runtimes []AIRuntime

    for _, p := range providers {
        cmd := exec.Command(p.command, p.flag)
        output, err := cmd.CombinedOutput()
        if err != nil {
            runtimes = append(runtimes, AIRuntime{
                Name:        p.name,
                DisplayName: p.displayName,
                Command:     p.command,
                Version:     "",
                Status:      "offline",
            })
            continue
        }

        version := strings.TrimSpace(string(output))
        if idx := strings.Index(version, "\n"); idx != -1 {
            version = version[:idx]
        }

        runtimes = append(runtimes, AIRuntime{
            Name:        p.name,
            DisplayName: p.displayName,
            Command:     p.command,
            Version:     version,
            Status:      "online",
        })
    }

    returnValue.Runtimes = runtimes
    return nil
}
```

#### 优化点

1. **并发检测**: 使用goroutine并发检测多个CLI，提升速度
2. **结果缓存**: 缓存检测结果（TTL 1小时），避免重复检测
3. **后台自动检测**: 可选的启动时自动检测

---

### 2. Worker配置对话框

**优先级**: P0
**目标**: 提供友好的UI界面创建和编辑Worker

#### 功能描述

用户点击"+ New Worker"按钮后，弹出Worker配置对话框，支持配置Worker的基本信息、Tool选择、System Prompt等。

#### UI设计

```
┌─────────────────────────────────────────────────┐
│ Create New Worker                                │
├─────────────────────────────────────────────────┤
│ Name: [Code Reviewer                    ]       │
│                                                 │
│ Description: [Reviews code for bugs...    ]     │
│                                                 │
│ Runtime: [Claude Code (1.2.3)            ▼]     │
│                                                 │
│ Instructions (System Prompt):                   │
│ ┌─────────────────────────────────────────┐    │
│ │ You are a code reviewer. Focus on:      │    │
│ │ 1. Logic errors                         │    │
│ │ 2. Security issues                      │    │
│ │ 3. Performance problems                 │    │
│ │                                         │    │
│ │ Provide specific, actionable feedback.  │    │
│ └─────────────────────────────────────────┘    │
│                                                 │
│ Skills (optional, comma-separated):             │
│ [code-review, debugging, optimization]          │
│                                                 │
│                        [Cancel] [Create Worker]│
└─────────────────────────────────────────────────┘
```

#### 技术实现

**前端组件** (`frontend/app/view/cowork/cowork-worker-dialog.tsx`):
```typescript
export function WorkerCreateDialog({ runtimes, onSave, onCancel }: WorkerCreateDialogProps) {
    const [name, setName] = useState("");
    const [description, setDescription] = useState("");
    const [selectedRuntime, setSelectedRuntime] = useState("");
    const [instructions, setInstructions] = useState("");
    const [skills, setSkills] = useState("");

    const handleSave = async () => {
        if (!name.trim() || !selectedRuntime) {
            return; // 验证失败
        }

        const worker: Partial<CoworkWorker> = {
            name: name.trim(),
            desc: description.trim(),
            tool: selectedRuntime,
            soul: instructions.trim(), // ⭐ 使用Soul字段存储Instructions
            skills: skills.trim() ? JSON.stringify(skills.split(",").map(s => s.trim())) : "",
        };

        await onSave(worker);
    };

    return (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
            <div className="bg-base rounded-lg border border-border/50 p-6 w-full max-w-2xl">
                <h2 className="text-lg font-semibold mb-4">Create New Worker</h2>

                <div className="space-y-4">
                    {/* Name */}
                    <div>
                        <label className="block text-sm font-medium mb-1">Name *</label>
                        <input
                            type="text"
                            value={name}
                            onChange={e => setName(e.target.value)}
                            placeholder="e.g. Code Reviewer"
                            className="w-full bg-base/50 border border-border/50 rounded px-3 py-2 text-sm"
                        />
                    </div>

                    {/* Description */}
                    <div>
                        <label className="block text-sm font-medium mb-1">Description</label>
                        <textarea
                            value={description}
                            onChange={e => setDescription(e.target.value)}
                            placeholder="What does this worker do?"
                            rows={2}
                            className="w-full bg-base/50 border border-border/50 rounded px-3 py-2 text-sm"
                        />
                    </div>

                    {/* Runtime选择器 */}
                    <div>
                        <label className="block text-sm font-medium mb-1">Runtime *</label>
                        <select
                            value={selectedRuntime}
                            onChange={e => setSelectedRuntime(e.target.value)}
                            className="w-full bg-base/50 border border-border/50 rounded px-3 py-2 text-sm"
                        >
                            <option value="">Select runtime...</option>
                            {runtimes.filter(r => r.status === "online").map(r => (
                                <option key={r.name} value={r.name}>
                                    {r.displayName} {r.version && `(${r.version})`}
                                </option>
                            ))}
                        </select>
                    </div>

                    {/* Instructions (System Prompt) */}
                    <div>
                        <label className="block text-sm font-medium mb-1">Instructions (System Prompt)</label>
                        <textarea
                            value={instructions}
                            onChange={e => setInstructions(e.target.value)}
                            placeholder="You are a code reviewer. Focus on logic errors, security issues, and performance problems..."
                            rows={6}
                            className="w-full bg-base/50 border border-border/50 rounded px-3 py-2 text-sm font-mono text-xs"
                        />
                    </div>

                    {/* Skills */}
                    <div>
                        <label className="block text-sm font-medium mb-1">Skills (optional, comma-separated)</label>
                        <input
                            type="text"
                            value={skills}
                            onChange={e => setSkills(e.target.value)}
                            placeholder="code-review, debugging, optimization"
                            className="w-full bg-base/50 border border-border/50 rounded px-3 py-2 text-sm"
                        />
                    </div>
                </div>

                {/* 底部操作栏 */}
                <div className="flex justify-end gap-2 mt-6 pt-4 border-t border-border/50">
                    <button
                        onClick={onCancel}
                        className="px-4 py-2 bg-base/50 text-primary rounded text-sm hover:bg-base/70 transition-colors cursor-pointer"
                    >
                        Cancel
                    </button>
                    <button
                        onClick={handleSave}
                        disabled={!name.trim() || !selectedRuntime}
                        className="px-4 py-2 bg-accent/80 text-primary rounded text-sm hover:bg-accent transition-colors cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                        Create Worker
                    </button>
                </div>
            </div>
        </div>
    );
}
```

**后端RPC** (`pkg/wshrpc/wshrpctypes.go`):
```go
// 修改现有的CoworkRegisterWorkerCommand，支持更多字段
type CoworkRegisterWorkerData struct {
    WorkerId   string `json:"workerid"`
    Name       string `json:"name"`
    Tool       string `json:"tool"`
    Desc       string `json:"desc,omitempty"`       // ⭐ 使用Desc字段
    Soul       string `json:"soul,omitempty"`      // ⭐ 使用Soul字段存储Instructions
    Skills     string `json:"skills,omitempty"`    // ⭐ 使用Skills字段存储JSON数组
    BlockId    string `json:"blockid"`
    TabId      string `json:"tabid"`
}
```

#### 关键决策

**Model字段**: 经过评估，当前Worker通过AI CLI工具运行，这些工具内部已处理model选择（如`claude`使用用户的默认配置）。因此**P0阶段不新增Model字段**，未来如果需要可在P1阶段添加。

---

### 3. Worker模板系统

**优先级**: P0
**目标**: 减少重复配置，提升Worker创建效率

#### 功能描述

用户可以创建Worker模板，模板包含常用的Worker配置。创建Worker时可以选择基于模板初始化，系统自动填充模板中的配置。

#### UI设计

```
┌─────────────────────────────────────────────────┐
│ Create New Worker                                │
├─────────────────────────────────────────────────┤
│ Template: [Code Reviewer                ▼]       │
│           ───────────────────────────────        │
│           (No Template)                          │
│           Researcher                             │
│           Tester                                 │
│           Custom...                              │
│                                                 │
│ Name: [Code Reviewer                    ]       │
│ Description: [Reviews code for bugs...    ]     │
│ Runtime: [Claude Code (1.2.3)            ▼]     │
│ Instructions: (auto-filled from template)        │
│                                                 │
│                        [Cancel] [Create Worker]│
└─────────────────────────────────────────────────┘
```

#### 技术实现

**数据库Schema** (`db/migrations/`):
```sql
CREATE TABLE IF NOT EXISTS cowork_worker_templates (
    template_id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    tool TEXT NOT NULL,
    desc TEXT,
    soul TEXT,
    skills TEXT,
    mcp_servers TEXT,
    is_system BOOLEAN DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
```

**后端RPC** (`pkg/wshrpc/wshrpctypes.go`):
```go
// CoworkWorkerTemplate Worker模板
type CoworkWorkerTemplate struct {
    TemplateId  string `json:"templateid"`
    Name        string `json:"name"`
    Description string `json:"description,omitempty"`
    Tool        string `json:"tool"`
    Desc        string `json:"desc,omitempty"`
    Soul        string `json:"soul,omitempty"`
    Skills      string `json:"skills,omitempty"`
    McpServers  string `json:"mcpservers,omitempty"`
    IsSystem    bool   `json:"is_system"`
    CreatedAt   int64  `json:"createdat"`
    UpdatedAt   int64  `json:"updatedat"`
}

// CoworkListTemplatesCommand 列出所有模板
type CoworkListTemplatesCommand struct {
    Command
}

type CoworkListTemplatesCommandReturn struct {
    Templates []CoworkWorkerTemplate `json:"templates"`
}

// CoworkCreateTemplateCommand 创建模板
type CoworkCreateTemplateCommand struct {
    Command
    Name        string `json:"name"`
    Description string `json:"description,omitempty"`
    Tool        string `json:"tool"`
    Desc        string `json:"desc,omitempty"`
    Soul        string `json:"soul,omitempty"`
    Skills      string `json:"skills,omitempty"`
    McpServers  string `json:"mcpservers,omitempty"`
}

// CoworkDeleteTemplateCommand 删除模板
type CoworkDeleteTemplateCommand struct {
    Command
    TemplateId string `json:"templateid"`
}
```

**预设模板** (系统默认提供):
```go
var systemTemplates = []CoworkWorkerTemplate{
    {
        TemplateId:  "system-code-reviewer",
        Name:        "Code Reviewer",
        Description: "Reviews code for bugs, security issues, and performance problems",
        Tool:        "claude",
        Soul:        "You are a code reviewer. Focus on:\n1. Logic errors\n2. Security issues\n3. Performance problems\n\nProvide specific, actionable feedback.",
        Skills:      `["code-review", "debugging", "optimization"]`,
        IsSystem:    true,
    },
    {
        TemplateId:  "system-researcher",
        Name:        "Researcher",
        Description: "Researches and analyzes code, APIs, and best practices",
        Tool:        "opencode",
        Soul:        "You are a technical researcher. Find and analyze:\n1. Best practices\n2. API documentation\n3. Implementation examples\n\nProvide citations and sources.",
        Skills:      `["research", "analysis", "documentation"]`,
        IsSystem:    true,
    },
    {
        TemplateId:  "system-tester",
        Name:        "Tester",
        Description: "Writes and runs tests to verify code correctness",
        Tool:        "aider",
        Soul:        "You are a QA engineer. Focus on:\n1. Unit tests\n2. Integration tests\n3. Edge cases\n\nEnsure comprehensive test coverage.",
        Skills:      `["testing", "qa", "automation"]`,
        IsSystem:    true,
    },
}
```

#### 使用流程

1. 用户点击"+ New Worker"
2. 对话框顶部显示"Template"下拉框，默认为"(No Template)"
3. 用户选择模板后，自动填充Name、Description、Instructions、Skills等字段
4. 用户可以修改填充的内容
5. 点击"Create Worker"创建Worker

---

### 4. Task手动分配

**优先级**: P0
**目标**: 支持用户在创建Task时手动指定Worker

#### 功能描述

在创建Task时，除了默认的"Auto-assign by Supervisor"选项，用户还可以手动选择一个空闲的Worker来处理该任务。

#### UI设计

```
┌─────────────────────────────────────────────────┐
│ New Task                                        │
├─────────────────────────────────────────────────┤
│ Title: [Fix login bug                    ]      │
│                                                 │
│ Description:                                    │
│ ┌─────────────────────────────────────────┐    │
│ │ Users report login fails when...        │    │
│ └─────────────────────────────────────────┘    │
│                                                 │
│ Priority: [Medium ▼]                            │
│                                                 │
│ Assign to: [Auto-assign by Supervisor    ▼]     │
│            ────────────────────────────────     │
│            Code Reviewer (idle)                 │
│            Researcher (idle)                    │
│            Tester (idle)                        │
│                                                 │
│                        [Cancel] [Create Task]   │
└─────────────────────────────────────────────────┘
```

#### 技术实现

**前端组件** (`cowork.tsx`修改):
```typescript
const [assignTo, setAssignTo] = useState<string>("");

const handleCreateTask = async () => {
    if (!newTaskTitle.trim()) {
        return;
    }

    await model.createTask(
        newTaskTitle,
        newTaskDesc,
        newTaskPriority,
        assignTo || undefined  // 传递assignedworker
    );

    setNewTaskTitle("");
    setNewTaskDesc("");
    setNewTaskPriority("medium");
    setAssignTo("");
};

// 在表单中添加Worker选择器
<select
    className="flex-1 bg-base/50 border border-border/50 rounded px-2 py-1 text-sm"
    value={assignTo}
    onChange={e => setAssignTo(e.target.value)}
>
    <option value="">Auto-assign by Supervisor</option>
    <optgroup label="Available Workers">
        {workers.filter(w => w.status === "idle").map(w => (
            <option key={w.workerid} value={w.workerid}>
                {w.name} ({w.tool})
            </option>
        ))}
    </optgroup>
</select>
```

**后端RPC修改** (`pkg/wshrpc/wshrpctypes.go`):
```go
// CoworkCreateTaskData 新增assignedworker字段
type CoworkCreateTaskData struct {
    Title          string `json:"title"`
    Description    string `json:"description,omitempty"`
    Priority       string `json:"priority"`
    AssignedWorker string `json:"assignedworker,omitempty"` // ⭐ 新增字段
}
```

---

## P1 功能需求

### 5. Task执行控制（暂停/恢复/取消）

**优先级**: P1
**目标**: 支持用户控制Task的执行流程

#### 功能描述

在Task Card上添加操作按钮，支持暂停、恢复、取消Task。

#### UI设计

```
┌─────────────────────────────────────────────────┐
│ Task: Fix login bug                              │
│ ━━━━━━━━━━━━━━━━━━━━━━━━━━ 60%                 │
│ → Code Reviewer (running)                        │
│                                                 │
│ [⏸ Pause] [✕ Cancel]                           │
└─────────────────────────────────────────────────┘

暂停后：

┌─────────────────────────────────────────────────┐
│ Task: Fix login bug                              │
│ ━━━━━━━━━━━━━━━━━━━━━━━━━━ 60% (Paused)         │
│ → Code Reviewer (paused)                         │
│                                                 │
│ [▶ Resume] [✕ Cancel]                           │
└─────────────────────────────────────────────────┘
```

#### 技术实现

**方案选择**: 逻辑暂停（推荐）

由于Worker是term子块，直接发送系统信号（SIGSTOP/SIGCONT）可能影响整个终端会话，因此采用**逻辑暂停**方案：

1. **暂停**: 更新Task状态为"paused"，Worker继续运行但忽略其输出
2. **恢复**: 更新Task状态为"working"，重新监听Worker输出
3. **取消**: 更新Task状态为"cancelled"，如果Worker正在运行则发送Ctrl+C

**前端组件** (`cowork.tsx`修改):
```typescript
const handlePauseTask = async (taskId: string) => {
    await RpcApi.CoworkPauseTaskCommand(TabRpcClient, taskId);
    await model.refreshAllData();
};

const handleResumeTask = async (taskId: string) => {
    await RpcApi.CoworkResumeTaskCommand(TabRpcClient, taskId);
    await model.refreshAllData();
};

const handleCancelTask = async (taskId: string) => {
    await RpcApi.CoworkCancelTaskCommand(TabRpcClient, taskId);
    await model.refreshAllData();
};

// 在TaskColumn中添加操作按钮
<div className="flex gap-1 mt-1">
    {t.status === "working" && (
        <>
            <button onClick={() => onPause(t.taskid)}>⏸</button>
            <button onClick={() => onCancel(t.taskid)}>✕</button>
        </>
    )}
    {t.status === "paused" && (
        <>
            <button onClick={() => onResume(t.taskid)}>▶</button>
            <button onClick={() => onCancel(t.taskid)}>✕</button>
        </>
    )}
</div>
```

**后端RPC** (`pkg/wshrpc/wshrpctypes.go`):
```go
// CoworkPauseTaskCommand 暂停Task
type CoworkPauseTaskCommand struct {
    Command
    TaskID string `json:"taskid"`
}

func (c *CoworkPauseTaskCommand) Run(ctx context.Context, m *Mailbox) error {
    // 1. 更新Task状态为"paused"
    // 2. 发送cowork:taskupdate事件
    return nil
}

// CoworkResumeTaskCommand 恢复Task
type CoworkResumeTaskCommand struct {
    Command
    TaskID string `json:"taskid"`
}

func (c *CoworkResumeTaskCommand) Run(ctx context.Context, m *Mailbox) error {
    // 1. 更新Task状态为"working"
    // 2. 发送cowork:taskupdate事件
    return nil
}

// CoworkCancelTaskCommand 取消Task
type CoworkCancelTaskCommand struct {
    Command
    TaskID string `json:"taskid"`
}

func (c *CoworkCancelTaskCommand) Run(ctx context.Context, m *Mailbox) error {
    // 1. 更新Task状态为"cancelled"
    // 2. 如果Worker正在运行，发送Ctrl+C（通过ControllerInputCommand）
    // 3. 发送cowork:taskupdate事件
    return nil
}
```

**数据库Schema更新**:
```sql
-- CoworkTask表需要支持paused和cancelled状态
-- 已有的Status字段是string，无需修改schema
-- 只需确保应用层正确处理这两个状态
```

---

### 6. Worker输出历史（Session恢复）

**优先级**: P1
**目标**: 记录Worker会话元数据，利用CLI原生恢复机制

#### 核心设计理念

> **AI CLI工具都有自己的session恢复机制，我们只需要记录元数据，让CLI自己处理恢复。**

各CLI工具的恢复命令：
- **Claude Code**: `claude --restore-session <session-id>`
- **OpenCode**: `opencode --resume <session-id>`
- **Cursor Agent**: `cursor-agent --session <session-id>`
- **Aider**: `aider --recover <session-id>`

因此，我们**不需要存储完整输出内容**，只需要记录：
- Session ID（CLI工具生成）
- Session Title
- 工具类型（用于生成恢复命令）
- 时间范围

#### 功能描述

**Worker运行时**:
1. 启动时：记录session_id、title、tool、started_at
2. 结束时：标记ended_at

**用户操作**:
1. 查看Worker的历史会话列表（只显示元数据）
2. 点击"Restore"按钮：创建新的term子块，自动执行恢复命令
3. 删除历史记录

#### UI设计

**Worker Card新增按钮**:
```
┌─────────────────────────────────────────────────┐
│ Code Reviewer                        ● Working   │
│ → Fix login bug (60%)                           │
│                                                 │
│ [⏸ Pause] [📜 History (5)] [⚙️ Edit]         │
└─────────────────────────────────────────────────┘
```

**History对话框**（极简版）:
```
┌─────────────────────────────────────────────────┐
│ Worker Session History - Code Reviewer          │
├─────────────────────────────────────────────────┤
│ [Clear All History]                             │
│                                                 │
│ ┌───────────────────────────────────────────┐  │
│ │ 📄 Fix login bug                          │  │
│ │    Session: abc-123-def                   │  │
│ │    2026-04-27 10:30 - 11:15 (45 min)     │  │
│ │    [🔄 Restore to New Terminal]           │  │
│ └───────────────────────────────────────────┘  │
│ ┌───────────────────────────────────────────┐  │
│ │ 📄 Implement feature X                   │  │
│ │    Session: xyz-789-uvw                  │  │
│ │    2026-04-26 14:20 - 15:30 (70 min)     │  │
│ │    [🔄 Restore to New Terminal]           │  │
│ └───────────────────────────────────────────┘  │
│                                                 │
│                          [Close]                │
└─────────────────────────────────────────────────┘
```

#### 技术实现

**数据库Schema** (`db/migrations/`):
```sql
CREATE TABLE IF NOT EXISTS cowork_worker_sessions (
    session_id TEXT PRIMARY KEY,        -- CLI工具的session ID
    worker_id TEXT NOT NULL,            -- 关联的Worker
    task_id TEXT,                       -- 关联的Task（可选）
    title TEXT NOT NULL,                -- 会话标题
    tool TEXT NOT NULL,                 -- CLI工具类型（claude/opencode/cursor/aider）
    started_at INTEGER NOT NULL,        -- 开始时间
    ended_at INTEGER,                   -- 结束时间（null表示进行中）
    
    FOREIGN KEY (worker_id) REFERENCES cowork_workers(worker_id) ON DELETE CASCADE,
    FOREIGN KEY (task_id) REFERENCES cowork_tasks(task_id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_cowork_worker_sessions_worker_id 
ON cowork_worker_sessions(worker_id, started_at DESC);
```

**关键设计决策**:
- ❌ **不存储content字段**（不需要，CLI工具自己管理）
- ✅ **只存储元数据**（session_id、title、tool、时间）
- ✅ **极简表结构**（只有7个字段）

**后端RPC** (`pkg/wshrpc/wshrpctypes.go`):
```go
// CoworkWorkerSession Worker会话记录（极简版）
type CoworkWorkerSession struct {
    SessionId string `json:"sessionid"`
    WorkerId  string `json:"workerid"`
    TaskId    string `json:"taskid,omitempty"`
    Title     string `json:"title"`
    Tool      string `json:"tool"`       // ⭐ CLI工具类型
    StartedAt int64  `json:"startedat"`
    EndedAt   int64  `json:"endedat,omitempty"`
}

// CoworkListWorkerSessionsCommand 列出Worker的历史会话
type CoworkListWorkerSessionsCommand struct {
    Command
    WorkerId string `json:"workerid"`
    Limit    int    `json:"limit,omitempty"`
}

type CoworkListWorkerSessionsCommandReturn struct {
    Sessions []CoworkWorkerSession `json:"sessions"`
}

// CoworkRestoreWorkerSessionCommand 恢复会话（创建新term子块）
type CoworkRestoreWorkerSessionCommand struct {
    Command
    SessionId     string `json:"sessionid"`
    ParentBlockId string `json:"parentblockid"` // 父块ID（CoworkView的blockId）
}

type CoworkRestoreWorkerSessionCommandReturn struct {
    BlockId string `json:"blockid"` // 新创建的term子块ID
    Success bool   `json:"success"`
    Error   string `json:"error,omitempty"`
}

// CoworkDeleteWorkerSessionCommand 删除会话记录
type CoworkDeleteWorkerSessionCommand struct {
    Command
    SessionId string `json:"sessionid"`
}
```

**实现细节** (`pkg/cowork/session_manager.go`):
```go
// SessionManager 管理Worker会话（极简版）
type SessionManager struct {
    mutex sync.RWMutex
}

// StartSession 开始新会话
func (sm *SessionManager) StartSession(ctx context.Context, workerId, taskId, title, tool string) (string, error) {
    // 注意：这里应该使用CLI工具返回的session ID，而不是自己生成
    // 如果CLI工具不返回session ID，则使用UUID
    sessionId := uuid.New().String()
    now := time.Now().Unix()

    _, err := db.Exec(`
        INSERT INTO cowork_worker_sessions (session_id, worker_id, task_id, title, tool, started_at)
        VALUES (?, ?, ?, ?, ?, ?)
    `, sessionId, workerId, taskId, title, tool, now)

    return sessionId, err
}

// EndSession 结束会话
func (sm *SessionManager) EndSession(ctx context.Context, sessionId string) error {
    now := time.Now().Unix()
    _, err := db.Exec(`
        UPDATE cowork_worker_sessions
        SET ended_at = ?
        WHERE session_id = ?
    `, now, sessionId)
    return err
}

// RestoreSession 恢复会话（创建新term子块）
func (sm *SessionManager) RestoreSession(ctx context.Context, sessionId, parentBlockId string) (string, error) {
    // 1. 查询会话信息
    var session CoworkWorkerSession
    err := db.Get(&session, `
        SELECT session_id, tool, title 
        FROM cowork_worker_sessions 
        WHERE session_id = ?
    `, sessionId)
    if err != nil {
        return "", err
    }

    // 2. 根据tool类型生成恢复命令
    restoreCmd := sm.getRestoreCommand(session.Tool, sessionId)

    // 3. 创建新的term子块
    blockDef := &BlockDef{
        Meta: map[string]string{
            "view": "term",
        },
    }
    
    oref, err := RpcApi.CreateSubBlockCommand(ctx, &CreateSubBlockData{
        ParentBlockId: parentBlockId,
        BlockDef:      blockDef,
    })
    if err != nil {
        return "", err
    }

    newBlockId := oref.(string)

    // 4. 在新term中执行恢复命令
    b64Cmd := stringToBase64(restoreCmd + "\n")
    err = RpcApi.ControllerInputCommand(ctx, &ControllerInputData{
        BlockId:     newBlockId,
        InputData64: b64Cmd,
    })

    return newBlockId, err
}

// getRestoreCommand 根据工具类型生成恢复命令
func (sm *SessionManager) getRestoreCommand(tool, sessionId string) string {
    switch tool {
    case "claude":
        return fmt.Sprintf("claude --restore-session %s", sessionId)
    case "opencode":
        return fmt.Sprintf("opencode --resume %s", sessionId)
    case "cursor":
        return fmt.Sprintf("cursor-agent --session %s", sessionId)
    case "aider":
        return fmt.Sprintf("aider --recover %s", sessionId)
    default:
        return fmt.Sprintf("%s --restore %s", tool, sessionId)
    }
}
```

**前端组件** (`frontend/app/view/cowork/cowork-session-history.tsx`):
```typescript
export function WorkerSessionHistory({ workerId, onClose }: WorkerSessionHistoryProps) {
    const [sessions, setSessions] = useState<CoworkWorkerSession[]>([]);

    useEffect(() => {
        const loadSessions = async () => {
            const result = await RpcApi.CoworkListWorkerSessionsCommand(TabRpcClient, {
                workerid: workerId,
                limit: 20
            });
            setSessions(result.sessions ?? []);
        };
        loadSessions();
    }, [workerId]);

    const handleRestore = async (sessionId: string) => {
        try {
            const result = await RpcApi.CoworkRestoreWorkerSessionCommand(TabRpcClient, {
                sessionid: sessionId,
                parentblockid: model.blockId, // CoworkView的blockId
            });

            if (result.success) {
                console.log("Restored session to block:", result.blockid);
                // 可选：高亮显示新创建的term子块
            }
        } catch (e) {
            console.error("Restore failed:", e);
        }
    };

    const formatDuration = (started: number, ended: number | undefined) => {
        const start = new Date(started * 1000);
        const end = ended ? new Date(ended * 1000) : new Date();
        const minutes = Math.floor((end.getTime() - start.getTime()) / 60000);
        return `${minutes} min`;
    };

    return (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
            <div className="bg-base rounded-lg border border-border/50 p-6 w-full max-w-md">
                <div className="flex justify-between items-center mb-4">
                    <h2 className="text-lg font-semibold">Worker Session History</h2>
                    <button onClick={onClose}>✕</button>
                </div>

                <div className="space-y-2 max-h-96 overflow-auto">
                    {sessions.length === 0 ? (
                        <p className="text-sm text-gray-500">No history yet</p>
                    ) : (
                        sessions.map(session => (
                            <div key={session.sessionid} className="border border-border/50 rounded p-3">
                                <div className="font-medium">{session.title}</div>
                                <div className="text-xs text-gray-500 mt-1">
                                    Session: <code className="bg-base/50 px-1 rounded">{session.sessionid}</code>
                                </div>
                                <div className="text-xs text-gray-500">
                                    {new Date(session.startedat * 1000).toLocaleString()} - 
                                    {session.endedat 
                                        ? new Date(session.endedat * 1000).toLocaleTimeString() 
                                        : " ongoing"
                                    } 
                                    ({formatDuration(session.startedat, session.endedat)})
                                </div>
                                <div className="mt-2">
                                    <button
                                        onClick={() => handleRestore(session.sessionid)}
                                        className="text-sm bg-accent/80 text-primary px-3 py-1 rounded hover:bg-accent transition-colors cursor-pointer"
                                    >
                                        🔄 Restore to New Terminal
                                    </button>
                                </div>
                            </div>
                        ))
                    )}
                </div>
            </div>
        </div>
    );
}
```

#### 核心优势

| 对比维度 | 复杂版（存储内容） | 简化版（元数据+CLI恢复） |
|----------|-------------------|-------------------------|
| **数据库字段** | 9个（含content） | 7个（无content） |
| **存储大小** | 每个会话MB级别 | 每个会话KB级别 |
| **实现复杂度** | 高（流式存储、压缩、清理） | 低（只存元数据） |
| **RPC命令** | 5个 | 3个 |
| **前端代码** | ~200行 | ~100行 |
| **恢复机制** | 自实现（不可靠） | CLI原生（可靠） |
| **维护成本** | 高 | 低 |
| **性能** | 需要压缩、清理优化 | 几乎无性能问题 |

#### Unix哲学的体现

> **做好一件事，依赖其他工具做好它们的事。**

- Cowork：管理Worker会话元数据
- CLI工具：管理session状态和输出内容
- 各司其职，简单可靠

---

### 7. UI简化（AI Panel摘要）

**优先级**: P1
**目标**: 简化AI Panel中的Cowork入口，避免功能重复

#### 功能描述

将AI Panel中的`CoworkWorkersPanel`简化为摘要面板，只显示状态统计和快速跳转按钮。

#### UI设计

**当前**（完整功能）:
```
┌─────────────────────────────────────────────────┐
│ AI Panel                                        │
├─────────────────────────────────────────────────┤
│ 👥 Cowork Mode                                  │
│ ┌─────────────────────────────────────────┐    │
│ │ Code Reviewer (working)                 │    │
│ │ → Fix login bug (60%)                   │    │
│ │ [⏸ Pause] [✕ Delete]                  │    │
│ └─────────────────────────────────────────┘    │
│ ┌─────────────────────────────────────────┐    │
│ │ Researcher (idle)                       │    │
│ │ [Activate]                              │    │
│ └─────────────────────────────────────────┘    │
└─────────────────────────────────────────────────┘
```

**简化后**（摘要）:
```
┌─────────────────────────────────────────────────┐
│ AI Panel                                        │
├─────────────────────────────────────────────────┤
│ [Chat] [Settings]                               │
├─────────────────────────────────────────────────┤
│ 👥 Cowork Mode        (2 active, 3 pending)     │
│                     [Open Cowork View →]       │
└─────────────────────────────────────────────────┘
```

#### 技术实现

**前端组件** (`frontend/app/aipanel/cowork-summary-panel.tsx`):
```typescript
import { useNavigate } from "react-router-dom";

export function CoworkSummaryPanel() {
    const navigate = useNavigate();
    const { data } = useCoworkSummary(); // 自定义hook，只获取摘要数据

    return (
        <div className="border-t border-gray-700">
            <button
                onClick={() => navigate("/view/cowork")}
                className="w-full px-3 py-2 flex items-center justify-between text-sm text-gray-300 hover:bg-gray-800 transition-colors"
            >
                <div className="flex items-center gap-2">
                    <span>👥</span>
                    <span className="font-medium">Cowork Mode</span>
                    <span className="text-xs text-gray-500">
                        ({data.activeWorkers} active, {data.pendingTasks} pending)
                    </span>
                </div>
                <span className="text-xs">Open →</span>
            </button>
        </div>
    );
}

function useCoworkSummary() {
    const [data, setData] = useState({
        activeWorkers: 0,
        pendingTasks: 0,
    });

    useEffect(() => {
        const interval = setInterval(async () => {
            const status = await RpcApi.CoworkGetStatusCommand(TabRpcClient);
            setData({
                activeWorkers: status.activeworkers ?? 0,
                pendingTasks: status.pendingtasks ?? 0,
            });
        }, 5000);
        return () => clearInterval(interval);
    }, []);

    return { data };
}
```

---

## P2 功能需求

### 8. Task依赖关系

**优先级**: P2
**目标**: 支持复杂任务的拆分和依赖管理

#### 功能描述

Task可以有父任务和子任务，支持：
- 创建子任务
- 设置依赖关系（子任务等待父任务完成）
- Task依赖树可视化

#### UI设计

```
┌─────────────────────────────────────────────────┐
│ Task: Implement User Authentication              │
│ Status: working                                 │
│ Progress: 50%                                   │
│                                                 │
│ ▼ Subtasks (3)                                  │
│   ┌─────────────────────────────────────────┐  │
│   │ ✓ Design database schema               │  │
│   │   completed at 2026-04-27 10:00        │  │
│   └─────────────────────────────────────────┘  │
│   ┌─────────────────────────────────────────┐  │
│   │ → Implement login API                  │  │
│   │   working (50%)                         │  │
│   └─────────────────────────────────────────┘  │
│   ┌─────────────────────────────────────────┐  │
│   │ ○ Implement logout API                 │  │
│   │   pending (waiting for parent)          │  │
│   └─────────────────────────────────────────┘  │
│                                                 │
│ [+ Add Subtask]                                 │
└─────────────────────────────────────────────────┘
```

#### 技术实现

**数据库Schema更新**:
```sql
ALTER TABLE cowork_tasks ADD COLUMN parent_task_id TEXT;
ALTER TABLE cowork_tasks ADD COLUMN depends_on TEXT; -- JSON array of task IDs
CREATE INDEX IF NOT EXISTS idx_cowork_tasks_parent ON cowork_tasks(parent_task_id);
```

**后端RPC**:
```go
// CoworkTask结构体新增字段
type CoworkTask struct {
    // ... 现有字段
    ParentTaskId string `json:"parenttaskid,omitempty"`
    DependsOn    string `json:"dependson,omitempty"` // JSON array
}

// CoworkCreateSubTaskCommand 创建子任务
type CoworkCreateSubTaskCommand struct {
    Command
    ParentTaskId string `json:"parenttaskid"`
    Title        string `json:"title"`
    Description  string `json:"description,omitempty"`
    Priority     string `json:"priority"`
}
```

---

### 9. Worker能力标签系统

**优先级**: P2
**目标**: 更智能的Task分配

#### 功能描述

Worker可以有多个能力标签（如"frontend", "backend", "testing"），Task可以有标签要求。Supervisor根据标签匹配分配Task。

#### UI设计

```
┌─────────────────────────────────────────────────┐
│ Create New Worker                                │
├─────────────────────────────────────────────────┤
│ Name: [Frontend Dev                     ]       │
│ Runtime: [Claude Code                    ▼]     │
│                                                 │
│ Tags:                                           │
│ [frontend] [react] [typescript] [+ Add Tag]     │
│                                                 │
│ Instructions: ...                               │
│                                                 │
│                        [Cancel] [Create Worker]│
└─────────────────────────────────────────────────┘
```

#### 技术实现

**使用现有的Skills字段**: Skills字段已经是JSON数组，可以复用为Tags。

**后端RPC修改**:
```go
// CoworkCreateTaskData 新增requiredtags字段
type CoworkCreateTaskData struct {
    Title         string `json:"title"`
    Description   string `json:"description,omitempty"`
    Priority      string `json:"priority"`
    RequiredTags  string `json:"requiredtags,omitempty"` // JSON array
}

// Supervisor逻辑更新：在prompt中加入标签匹配
func (m *CoworkViewModel) buildAnalysisPrompt(...) string {
    // ... 现有逻辑

    prompt += "\n## Worker Skills/Tags\n"
    for _, worker := range workers {
        var skills []string
        json.Unmarshal([]byte(worker.Skills), &skills)
        prompt += fmt.Sprintf("- %s: %v\n", worker.Name, skills)
    }

    return prompt
}
```

---

### 10. Task重试机制

**优先级**: P2
**目标**: 提升任务成功率

#### 功能描述

Task失败后自动重试，支持：
- 配置最大重试次数
- 失败后自动重新分配给其他Worker
- 重试次数达到上限后标记为最终失败

#### 技术实现

**数据库Schema更新**:
```sql
ALTER TABLE cowork_tasks ADD COLUMN attempt INTEGER DEFAULT 0;
ALTER TABLE cowork_tasks ADD COLUMN max_attempts INTEGER DEFAULT 3;
```

**后端逻辑**:
```go
// Supervisor的executeAssistantActions中增加重试逻辑
case "update_task":
    if act.status == "failed" {
        // 检查是否可以重试
        task, _ := getTask(act.task_id)
        if task.Attempt < task.MaxAttempts {
            // 重新分配
            await assignTaskToAnotherWorker(act.task_id)
        } else {
            // 标记为最终失败
            await updateTaskStatus(act.task_id, "permanently_failed")
        }
    }
```

---

## 技术方案

### 架构设计

```
┌─────────────────────────────────────────────────┐
│ CoworkView (Block View)                         │
│ - Runtime Detection Panel                        │
│ - Workers Registry (3列Grid)                     │
│ - Task Board (4列)                               │
│ - Activity Log                                   │
└─────────────────────────────────────────────────┘
            ↓ (RPC)
┌─────────────────────────────────────────────────┐
│ Go Backend (pkg/wshrpc)                         │
│ - CoworkDetectRuntimesCommand                   │
│ - CoworkCreateWorkerCommand (支持模板)          │
│ - CoworkCreateTaskCommand (支持手动分配)        │
│ - CoworkPause/Resume/CancelTaskCommand          │
│ - CoworkSessionCommands (历史会话)              │
└─────────────────────────────────────────────────┘
            ↓ (WPS Events)
┌─────────────────────────────────────────────────┐
│ WPS Pub/Sub                                      │
│ - cowork:taskupdate                             │
│ - cowork:workerupdate                           │
│ - cowork:sessionupdate                          │
└─────────────────────────────────────────────────┘
```

### 数据库Schema

#### 新增表

```sql
-- Worker模板表
CREATE TABLE IF NOT EXISTS cowork_worker_templates (
    template_id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    tool TEXT NOT NULL,
    desc TEXT,
    soul TEXT,
    skills TEXT,
    mcp_servers TEXT,
    is_system BOOLEAN DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

-- Worker会话历史表（简化版：只存元数据，不存储content）
CREATE TABLE IF NOT EXISTS cowork_worker_sessions (
    session_id TEXT PRIMARY KEY,
    worker_id TEXT NOT NULL,
    task_id TEXT,
    title TEXT NOT NULL,
    tool TEXT NOT NULL,
    started_at INTEGER NOT NULL,
    ended_at INTEGER,
    FOREIGN KEY (worker_id) REFERENCES cowork_workers(worker_id) ON DELETE CASCADE,
    FOREIGN KEY (task_id) REFERENCES cowork_tasks(task_id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_cowork_worker_sessions_worker_id 
ON cowork_worker_sessions(worker_id, started_at DESC);
```

#### 修改现有表

```sql
-- Task表增加字段
ALTER TABLE cowork_tasks ADD COLUMN parent_task_id TEXT;
ALTER TABLE cowork_tasks ADD COLUMN depends_on TEXT;
ALTER TABLE cowork_tasks ADD COLUMN attempt INTEGER DEFAULT 0;
ALTER TABLE cowork_tasks ADD COLUMN max_attempts INTEGER DEFAULT 3;

CREATE INDEX IF NOT EXISTS idx_cowork_tasks_parent ON cowork_tasks(parent_task_id);
```

### RPC API清单

#### P0 RPC命令

| 命令 | 功能 | 新增/修改 |
|------|------|----------|
| `CoworkDetectRuntimesCommand` | 检测AI CLI | 新增 |
| `CoworkListTemplatesCommand` | 列出Worker模板 | 新增 |
| `CoworkCreateTemplateCommand` | 创建模板 | 新增 |
| `CoworkDeleteTemplateCommand` | 删除模板 | 新增 |
| `CoworkRegisterWorkerCommand` | 注册Worker（支持更多字段） | 修改 |
| `CoworkCreateTaskCommand` | 创建Task（支持assignedworker） | 修改 |

#### P1 RPC命令

| 命令 | 功能 | 新增/修改 |
|------|------|----------|
| `CoworkPauseTaskCommand` | 暂停Task | 新增 |
| `CoworkResumeTaskCommand` | 恢复Task | 新增 |
| `CoworkCancelTaskCommand` | 取消Task | 新增 |
| `CoworkListWorkerSessionsCommand` | 列出Worker会话（元数据） | 新增 |
| `CoworkRestoreWorkerSessionCommand` | 恢复会话（创建新term子块） | 新增 |
| `CoworkDeleteWorkerSessionCommand` | 删除会话记录 | 新增 |

**注意**: 简化版去掉了`CoworkGetWorkerSessionCommand`（不需要获取详情，直接恢复）

#### P2 RPC命令

| 命令 | 功能 | 新增/修改 |
|------|------|----------|
| `CoworkCreateSubTaskCommand` | 创建子任务 | 新增 |
| `CoworkGetTaskTreeCommand` | 获取Task依赖树 | 新增 |
| `CoworkRetryTaskCommand` | 重试Task | 新增 |

---

## 实施计划

### Phase 1: P0基础增强（1-2周）

**目标**: 解决最紧迫的用户痛点

**任务清单**:
- [ ] 实现`CoworkDetectRuntimesCommand` RPC
- [ ] 创建`RuntimeDetectionPanel`组件
- [ ] 创建`WorkerCreateDialog`组件
- [ ] 实现`CoworkListTemplatesCommand` RPC
- [ ] 实现`CoworkCreateTemplateCommand` RPC
- [ ] 创建3个系统默认模板
- [ ] 创建`WorkersRegistryPanel`组件（3列Grid）
- [ ] 修改Task创建表单，添加Worker选择器
- [ ] 修改`CoworkCreateTaskCommand`支持assignedworker
- [ ] 集成所有组件到CoworkView
- [ ] 编写单元测试
- [ ] 手动测试所有功能

**验收标准**:
- ✅ 用户可以检测到已安装的AI CLI
- ✅ 用户可以基于模板快速创建Worker
- ✅ 用户可以手动分配Task给指定Worker
- ✅ 所有RPC命令有单元测试覆盖

### Phase 2: P1执行控制（1-2周）

**目标**: 增强Task执行控制能力

**任务清单**:
- [ ] 实现`CoworkPauseTaskCommand` RPC
- [ ] 实现`CoworkResumeTaskCommand` RPC
- [ ] 实现`CoworkCancelTaskCommand` RPC
- [ ] 在TaskCard添加操作按钮
- [ ] 创建`cowork_worker_sessions`表
- [ ] 实现`SessionManager`
- [ ] 实现`CoworkListWorkerSessionsCommand` RPC
- [ ] 实现`CoworkGetWorkerSessionCommand` RPC
- [ ] 实现`CoworkRestoreWorkerSessionCommand` RPC
- [ ] 创建`WorkerSessionHistory`组件
- [ ] 简化AI Panel的CoworkWorkersPanel
- [ ] 编写单元测试
- [ ] 手动测试所有功能

**验收标准**:
- ✅ 用户可以暂停/恢复Task
- ✅ 用户可以查看Worker历史输出
- ✅ 用户可以恢复历史会话到当前Terminal
- ✅ AI Panel显示Cowork摘要而非完整列表

### Phase 3: P2高级功能（按需实施）

**目标**: 支持复杂任务场景

**任务清单**:
- [ ] 修改`cowork_tasks`表，增加parent_task_id等字段
- [ ] 实现`CoworkCreateSubTaskCommand` RPC
- [ ] 实现`CoworkGetTaskTreeCommand` RPC
- [ ] 创建Task依赖树可视化组件
- [ ] 修改Supervisor逻辑，支持标签匹配
- [ ] 实现Task重试机制
- [ ] 编写单元测试
- [ ] 手动测试所有功能

**验收标准**:
- ✅ 用户可以创建子任务
- ✅ 用户可以查看Task依赖关系
- ✅ Supervisor根据标签匹配分配Task
- ✅ Task失败后自动重试

---

## 验收标准

### P0验收标准

1. **Runtime检测**:
   - ✅ 能正确检测已安装的AI CLI（Claude Code、OpenCode、Cursor Agent、Aider）
   - ✅ 显示版本号
   - ✅ 标记未安装的CLI为offline

2. **Worker配置**:
   - ✅ 用户可以通过对话框创建Worker
   - ✅ 支持选择模板
   - ✅ 支持配置Name、Description、Runtime、Instructions、Skills
   - ✅ 使用现有的Soul字段存储Instructions

3. **Worker模板**:
   - ✅ 系统提供3个默认模板（Code Reviewer、Researcher、Tester）
   - ✅ 用户可以创建自定义模板
   - ✅ 用户可以删除自定义模板
   - ✅ 选择模板后自动填充表单

4. **Task手动分配**:
   - ✅ 创建Task时可以选择Worker
   - ✅ 默认为"Auto-assign by Supervisor"
   - ✅ 只显示idle状态的Worker
   - ✅ 手动分配后Task状态变为"assigned"

### P1验收标准

1. **Task执行控制**:
   - ✅ 用户可以暂停working状态的Task
   - ✅ 用户可以恢复paused状态的Task
   - ✅ 用户可以取消任意状态的Task
   - ✅ Task状态正确流转（working <-> paused -> cancelled）

2. **Worker输出历史**:
   - ✅ 自动记录Worker每次执行的输出
   - ✅ 记录包含Session ID、Title、Content、时间戳
   - ✅ 用户可以查看历史会话列表
   - ✅ 用户可以恢复历史会话到当前Terminal
   - ✅ 用户可以删除历史会话

3. **UI简化**:
   - ✅ AI Panel显示Cowork摘要（active/pending统计）
   - ✅ 点击摘要跳转到完整CoworkView
   - ✅ CoworkView包含完整功能

### P2验收标准

1. **Task依赖关系**:
   - ✅ 用户可以创建子任务
   - ✅ 子任务等待父任务完成后才能开始
   - ✅ 用户可以查看Task依赖树

2. **Worker能力标签**:
   - ✅ Worker可以有多个标签
   - ✅ Task可以有标签要求
   - ✅ Supervisor根据标签匹配分配Task

3. **Task重试机制**:
   - ✅ Task失败后自动重试
   - ✅ 达到max_attempts后标记为最终失败
   - ✅ 重试时分配给不同的Worker

---

## 附录

### A. 与参考系统的对比

| 特性 | OpenFang | Multica | WaveAI Cowork (优化后) |
|------|----------|---------|----------------------|
| **执行模式** | Daemon后台 | Daemon后台 | **Term块可见** ✅ |
| **Runtime检测** | 启动时自动 | 启动时自动 | **手动+自动** ✅ |
| **Worker配置** | HAND.toml | CreateAgentRequest | **模板+对话框** ✅ |
| **Task分配** | LLM自动 | 手动+自动 | **手动+自动** ✅ |
| **执行控制** | pause/resume | cancel | **pause/resume/cancel** ✅ |
| **历史输出** | ❌ | ❌ | **✅ 会话恢复** |
| **Task依赖** | ❌ | ❌ | **✅ (P2)** |
| **Worker标签** | ❌ | ❌ | **✅ (P2)** |

### B. 关键技术决策

#### 决策1: Model字段

**问题**: 是否需要为Worker添加Model字段？

**结论**: P0阶段不添加，P1阶段评估

**理由**:
- 当前Worker通过AI CLI工具运行，这些工具内部已处理model选择
- Claude Code使用用户的默认配置
- 如果未来需要强制指定model，可以通过CustomCmd传递参数

#### 决策2: Pause/Resume实现方式

**问题**: 如何实现Task的暂停和恢复？

**结论**: 采用逻辑暂停（推荐）

**理由**:
- 直接发送系统信号（SIGSTOP/SIGCONT）可能影响整个终端会话
- 逻辑暂停更安全：只更新Task状态，Worker继续运行但忽略输出
- 如果AI CLI支持暂停命令（如`claude --pause`），可以结合使用

#### 决策3: Worker输出历史存储方式

**问题**: 如何存储Worker的大量输出？

**结论**: **不存储输出内容**，只存储元数据

**理由**:
- AI CLI工具都有自己的session恢复机制（`claude --restore-session`、`opencode --resume`等）
- 我们只需要记录session_id、title、tool、时间等元数据
- 用户需要查看输出时，创建新term子块并执行恢复命令
- 遵循Unix哲学：做好一件事，依赖其他工具做好它们的事

**优势**:
- 数据库存储减少99%+（从MB级别降到KB级别）
- 实现复杂度大幅降低（不需要流式存储、压缩、清理）
- 恢复机制更可靠（使用CLI原生功能，而不是自实现）

#### 决策4: Session ID生成

**问题**: 如何生成Session ID？

**结论**: 使用UUID

**理由**:
- 全局唯一
- 避免冲突
- 易于追踪和调试

### C. 性能优化建议

1. **Runtime检测缓存**:
   - 缓存检测结果（TTL 1小时）
   - 避免重复检测

2. **Task列表分页**:
   - 如果Task数量很大（>100），考虑分页加载
   - 默认只显示最近50个Task

3. **历史会话定期清理**:
   - 自动删除180天前的会话（元数据很小，可以保留更久）
   - 提供手动清理按钮
   - 避免数据库无限增长

### D. 用户体验优化

1. **引导流程**:
   - 首次使用时显示引导动画
   - 指导用户如何检测Runtime、创建Worker、创建Task

2. **快捷键支持**:
   - `Ctrl+Shift+N`: 新建Task
   - `Ctrl+Shift+W`: 新建Worker
   - `Ctrl+Shift+P`: 暂停/恢复Task

3. **拖拽支持**:
   - 拖拽Task到Worker进行分配
   - 拖拽Worker调整顺序

4. **实时通知**:
   - Task完成时显示通知
   - Worker出错时显示警告

### E. 安全考虑

1. **Runtime检测安全**:
   - 只检测已知的AI CLI工具
   - 不执行任意命令
   - 超时控制（5秒）

2. **Worker命令验证**:
   - 验证CustomCmd字段
   - 防止命令注入
   - 白名单允许的参数

3. **会话恢复权限**:
   - 只能恢复自己创建的会话
   - 不能恢复其他用户的会话（如果有多用户）

### F. 测试计划

#### 单元测试

- [ ] Runtime检测逻辑
- [ ] Worker模板CRUD
- [ ] Task状态流转
- [ ] Session管理

#### 集成测试

- [ ] Runtime检测 -> Worker创建
- [ ] Worker创建 -> Task分配
- [ ] Task执行 -> Session记录
- [ ] Session恢复 -> Terminal显示

#### 手动测试

- [ ] 检测已安装的AI CLI
- [ ] 基于模板创建Worker
- [ ] 手动分配Task
- [ ] 暂停/恢复Task
- [ ] 查看Worker历史
- [ ] 恢复历史会话

---

## 版本历史

| 版本 | 日期 | 变更 |
|------|------|------|
| v2.1 | 2026-04-27 | **简化版**：简化Worker输出历史实现，利用CLI原生session恢复机制；去掉content字段存储；数据库字段从9个减少到7个；RPC命令从6个减少到3个 |
| v2.0 | 2026-04-27 | 优化版：基于架构审查，调整优先级，新增Worker模板、输出历史等功能 |
| v1.0 | 2026-04-27 | 初始版本（docs/cowork-optimization-design.md） |

---

## 文档维护

**作者**: AI (基于设计文档v1.0 + 架构审查)
**状态**: 待实施
**反馈**: 实施过程中请根据实际情况更新本文档
