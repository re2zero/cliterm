# WaveAI Cowork Mode 优化设计方案

> **版本**: v1.0
> **日期**: 2026-04-27
> **状态**: 设计阶段
> **参考**: OpenFang (HAND.toml、Hands Dashboard)、Multica (Agent注册、Task Board)

---

## 📋 目录

1. [架构概述](#架构概述)
2. [当前问题分析](#当前问题分析)
3. [优化方案总览](#优化方案总览)
4. [Phase 1: Runtime检测与Worker注册](#phase-1-runtime检测与worker注册)
5. [Phase 2: Task管理增强](#phase-2-task管理增强)
6. [Phase 3: UI整合与体验优化](#phase-3-ui整合与体验优化)
7. [数据结构定义](#数据结构定义)
8. [RPC API设计](#rpc-api设计)
9. [实现路线图](#实现路线图)

---

## 架构概述

### 当前架构

```
┌─────────────────────────────────────────────┐
│  CoworkView (Block View)                     │
│  - Task Board (4列: pending/working/done/failed) │
│  - Workers List (显示注册的workers)          │
│  - Activity Log                              │
│  - Supervision (LLM自动分配)                 │
└─────────────────────────────────────────────┘
           ↓ (通过RPC)
┌─────────────────────────────────────────────┐
│  Go Backend (pkg/wshrpc)                    │
│  - CoworkListTasks                          │
│  - CoworkCreateTask                         │
│  - CoworkRegisterWorker                     │
└─────────────────────────────────────────────┘
           ↓ (spawn term子块)
┌─────────────────────────────────────────────┐
│  Worker执行 (term view)                      │
│  - 在terminal中运行AI CLI命令                │
│  - 实时输出可见                              │
│  - 用户可直接交互                            │
└─────────────────────────────────────────────┘
```

### 与参考系统的关键差异

| 特性 | OpenFang | Multica | WaveAI Cowork |
|------|----------|---------|---------------|
| **执行模式** | Daemon后台运行 | Daemon后台运行 | **前端term块** |
| **Worker可见性** | 无（后台进程） | 无（后台进程） | **可见（terminal）** |
| **用户交互** | 通过Dashboard | 通过Web UI | **直接在terminal** |
| **目标场景** | 团队协作 | 团队协作 | **个人使用** |
| **Runtime检测** | 启动时自动 | 启动时自动 | **手动触发** |

**结论**: Cowork的独特优势是**用户能看到Worker执行过程**，这是OpenFang/Multica不具备的。

---

## 当前问题分析

### 问题1: Runtime检测缺失

**现象**: 用户不知道系统中有哪些可用的AI CLI

**影响**: 无法创建Worker（因为不知道有哪些runtime可用）

**根因**: 代码中硬编码了4种CLI（claude/opencode/cursor/aider），但没有检测机制

```typescript
// cowork-model.ts:451-464
private getWorkerStartCommand(tool: string): string {
    switch (tool) {
        case "claude": return "claude";
        case "opencode": return "opencode";
        case "cursor": return "cursor-agent";
        case "aider": return "aider";
        default: return tool;
    }
}
```

### 问题2: Worker配置过于简单

**现象**: Worker只有name/tool/status三个字段

**影响**:
- 无法配置model（如claude-sonnet-4-20250514）
- 无法设置system prompt（每个Worker都用相同指令）
- 无法传递环境变量（如API keys）

**当前数据结构**:
```typescript
interface CoworkWorker {
    workerid: string;
    name: string;
    tool: string;      // "claude" | "opencode" | "cursor" | "aider"
    status: string;    // "idle" | "working" | "offline" | "error"
    assignedtask?: string;
}
```

### 问题3: Task分配只能自动

**现象**: 创建Task后只能等待Supervisor自动分配

**影响**:
- 用户想指定某个Worker处理某个Task时做不到
- Supervisor逻辑不透明，用户不知道为什么选这个Worker

**当前Supervision流程**:
```
每10秒 → 收集pending/working tasks → 调用LLM → LLM返回JSON actions → 执行
```

### 问题4: 执行干预不足

**现象**: 运行中的Task只能删除Worker，无法暂停/恢复

**影响**:
- Task执行错误时无法暂停
- 无法临时中断后再继续

### 问题5: UI功能分散

**现象**: CoworkView和AI Panel的CoworkWorkersPanel功能重复

**影响**:
- 用户困惑不知道在哪里管理
- 维护两处代码

---

## 优化方案总览

### 核心原则

1. **保持term块执行模式**: 这是Cowork的独特优势，不改为后台运行
2. **个人使用场景**: 无需workspace/team权限概念
3. **渐进式增强**: 分阶段实现，P0 → P1 → P2

### 优化目标

| 阶段 | 目标 | 优先级 |
|------|------|--------|
| **Phase 1** | Runtime检测 + Worker配置增强 | P0 |
| **Phase 2** | Task手动分配 + 执行干预 | P1 |
| **Phase 3** | UI整合 + 体验优化 | P1 |

---

## Phase 1: Runtime检测与Worker注册

### 1.1 Runtime检测面板

#### UI设计

**位置**: CoworkView顶部新增"Runtimes"区域

```
┌─────────────────────────────────────────────┐
│ Cowork Mode                                 │
├─────────────────────────────────────────────┤
│ 🔍 AI CLI Detection                         │
│ [Detect AI CLIs]                            │
│ ┌───────────────────────────────────────┐  │
│ │ ● Claude Code   1.2.3  (claude)      │  │
│ │ ● OpenCode     2.4.5  (opencode)     │  │
│ │ ○ Cursor Agent N/A    (not found)   │  │
│ └───────────────────────────────────────┘  │
├─────────────────────────────────────────────┤
│ 👥 Workers Registry                         │
│ [ + New Worker ]                            │
│ ┌──────────┐ ┌──────────┐ ┌──────────┐  │
│ │Code      │ │Researcher│ │Tester    │  │
│ │Reviewer  │ │          │ │          │  │
│ │[Activate]│ │[Activate]│ │[Activate]│  │
│ └──────────┘ └──────────┘ └──────────┘  │
└─────────────────────────────────────────────┘
```

#### 交互流程

```
用户点击"Detect AI CLIs"
    ↓
后端执行检测 (分别运行: claude --version, opencode --version, ...)
    ↓
返回检测到的Runtime列表
    ↓
用户查看可用的AI CLI
    ↓
创建Worker时，Runtime选择器自动填充检测到的结果
```

#### 实现代码

**前端组件** (`frontend/app/view/cowork/cowork-runtime-panel.tsx`):

```typescript
import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";
import { useState } from "react";

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

            <button
                onClick={handleDetect}
                disabled={isDetecting}
                className="px-3 py-1.5 bg-accent/80 text-primary rounded hover:bg-accent transition-colors cursor-pointer text-sm"
            >
                {isDetecting ? "Detecting..." : "Detect AI CLIs"}
            </button>

            {runtimes.length > 0 && (
                <div className="mt-3 space-y-2">
                    {runtimes.map(runtime => (
                        <div key={runtime.name} className="flex items-center gap-2 text-sm">
                            <span className={`w-2 h-2 rounded-full ${runtime.status === "online" ? "bg-green-500" : "bg-gray-400"}`} />
                            <span className="font-medium">{runtime.displayName}</span>
                            <span className="text-gray-400 text-xs">{runtime.version || "N/A"}</span>
                            <span className="text-gray-500 text-xs">({runtime.command})</span>
                        </div>
                    ))}
                </div>
            )}

            {runtimes.length === 0 && !isDetecting && (
                <p className="text-xs text-gray-500 mt-2">
                    No AI CLIs detected. Install Claude Code, OpenCode, or Cursor Agent.
                </p>
            )}
        </div>
    );
}

interface AIRuntime {
    name: string;        // "claude", "opencode", "cursor", "aider"
    displayName: string; // "Claude Code", "OpenCode", "Cursor Agent", "Aider"
    command: string;     // 检测到的完整路径或命令名
    version: string;     // 版本号
    status: string;      // "online" | "offline"
}
```

**后端RPC实现** (`pkg/wshrpc/wshrpctypes.go`):

```go
// CoworkDetectRuntimesCommand 检测系统中可用的AI CLI
type CoworkDetectRuntimesCommand struct {
    Command
}

type AIRuntime struct {
    Name        string `json:"name"`
    DisplayName string `json:"display_name"`
    Command     string `json:"command"`
    Version     string `json:"version"`
    Status      string `json:"status"`
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
            // 未安装，跳过
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
        // 清理版本字符串（移除多余输出）
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

**RPC注册** (`pkg/wshrpc/wshserver/wshserver.go`):

```go
func (s *WshServer) handleCoworkDetectRuntimes(m *Mailbox, msg *Message) error {
    cmd := &CoworkDetectRuntimesCommand{}
    return cmd.Run(s.ctx, m, &cmd.CommandReturn)
}
```

---

### 1.2 Worker配置增强

#### 新增字段

基于Multica的CreateAgentRequest，为CoworkWorker增加以下字段：

```typescript
interface CoworkWorker {
    // 现有字段（保留）
    workerid: string;
    name: string;
    tool: string;
    status: string;
    assignedtask?: string;

    // 新增字段
    model?: string;              // 使用的模型，如 "claude-sonnet-4-20250514"
    instructions?: string;       // System prompt（角色指令）
    customEnv?: Record<string, string>;  // 环境变量，如 API keys
    customArgs?: string[];       // 额外的CLI参数
    description?: string;        // Worker描述
}
```

#### Worker创建/编辑对话框

**UI设计** (基于OpenFang的Hand配置 + Multica的CreateAgentDialog):

```
┌─────────────────────────────────────────────┐
│ Create New Worker                           │
├─────────────────────────────────────────────┤
│ Name: [Code Reviewer              ]         │
│                                             │
│ Description: [Reviews code for bugs...]    │
│                                             │
│ Runtime: [Claude Code (1.2.3)      ▼]      │
│                                             │
│ Model: [claude-sonnet-4-20250514    ▼]     │
│        (optional, overrides default)        │
│                                             │
│ Instructions:                               │
│ ┌─────────────────────────────────────┐    │
│ │ You are a code reviewer. Focus on:  │    │
│ │ 1. Logic errors                     │    │
│ │ 2. Security issues                  │    │
│ │ 3. Performance problems             │    │
│ │                                     │    │
│ │ Provide specific, actionable feedback. │  │
│ └─────────────────────────────────────┘    │
│                                             │
│ ▼ Advanced Settings (optional)              │
│   Custom Env (JSON):                        │
│   {"ANTHROPIC_API_KEY": "sk-xxx"}           │
│                                             │
│   Custom Args:                              │
│   [--max-tokens 200000]                     │
│                                             │
│                        [Cancel] [Create]   │
└─────────────────────────────────────────────┘
```

**实现代码** (`frontend/app/view/cowork/cowork-worker-dialog.tsx`):

```typescript
import { useState } from "react";
import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";

interface WorkerCreateDialogProps {
    runtimes: AIRuntime[];
    onSave: (worker: Partial<CoworkWorker>) => void;
    onCancel: () => void;
}

export function WorkerCreateDialog({ runtimes, onSave, onCancel }: WorkerCreateDialogProps) {
    const [name, setName] = useState("");
    const [description, setDescription] = useState("");
    const [selectedRuntime, setSelectedRuntime] = useState("");
    const [model, setModel] = useState("");
    const [instructions, setInstructions] = useState("");
    const [customEnv, setCustomEnv] = useState("");
    const [customArgs, setCustomArgs] = useState("");
    const [showAdvanced, setShowAdvanced] = useState(false);

    const handleSave = async () => {
        if (!name.trim() || !selectedRuntime) {
            return;  // 验证失败
        }

        const worker: Partial<CoworkWorker> = {
            name: name.trim(),
            description: description.trim(),
            tool: selectedRuntime,
            model: model.trim() || undefined,
            instructions: instructions.trim() || undefined,
            customEnv: customEnv.trim() ? JSON.parse(customEnv) : undefined,
            customArgs: customArgs.trim() ? customArgs.split(/\s+/) : undefined,
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

                    {/* Model选择器（可选） */}
                    {selectedRuntime && (
                        <div>
                            <label className="block text-sm font-medium mb-1">Model (Optional)</label>
                            <ModelDropdown
                                runtime={selectedRuntime}
                                value={model}
                                onChange={setModel}
                            />
                            <p className="text-xs text-gray-500 mt-1">
                                Leave empty to use default model
                            </p>
                        </div>
                    )}

                    {/* Instructions */}
                    <div>
                        <label className="block text-sm font-medium mb-1">Instructions</label>
                        <textarea
                            value={instructions}
                            onChange={e => setInstructions(e.target.value)}
                            placeholder="You are a code reviewer. Focus on logic errors, security issues, and performance problems..."
                            rows={6}
                            className="w-full bg-base/50 border border-border/50 rounded px-3 py-2 text-sm font-mono text-xs"
                        />
                    </div>

                    {/* Advanced Settings（折叠） */}
                    <details className="border border-border/50 rounded">
                        <summary
                            className="px-3 py-2 cursor-pointer text-sm font-medium hover:bg-base/50 flex items-center justify-between"
                            onClick={() => setShowAdvanced(!showAdvanced)}
                        >
                            Advanced Settings
                            <span className="text-xs">{showAdvanced ? "▼" : "▶"}</span>
                        </summary>
                        <div className="p-3 space-y-3">
                            {/* Custom Env */}
                            <div>
                                <label className="block text-xs text-gray-500 mb-1">Custom Env (JSON format)</label>
                                <textarea
                                    value={customEnv}
                                    onChange={e => setCustomEnv(e.target.value)}
                                    placeholder='{"ANTHROPIC_API_KEY": "sk-xxx"}'
                                    rows={3}
                                    className="w-full bg-base/50 border border-border/50 rounded px-2 py-1.5 text-xs font-mono"
                                />
                            </div>

                            {/* Custom Args */}
                            <div>
                                <label className="block text-xs text-gray-500 mb-1">Custom Args (space-separated)</label>
                                <input
                                    type="text"
                                    value={customArgs}
                                    onChange={e => setCustomArgs(e.target.value)}
                                    placeholder="--max-tokens 200000"
                                    className="w-full bg-base/50 border border-border/50 rounded px-2 py-1.5 text-xs font-mono"
                                />
                            </div>
                        </div>
                    </details>
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

// Model选择器组件（简化版）
function ModelDropdown({ runtime, value, onChange }: { runtime: string; value: string; onChange: (v: string) => void }) {
    // 基于runtime返回常见模型列表
    const modelsByRuntime: Record<string, string[]> = {
        claude: ["claude-sonnet-4-20250514", "claude-3-5-sonnet-20241022"],
        opencode: ["gpt-4", "gpt-3.5-turbo"],
        cursor: ["gpt-4", "claude-3-5-sonnet-20241022"],
        aider: ["gpt-4", "claude-3-5-sonnet-20241022"],
    };

    const models = modelsByRuntime[runtime] ?? [];

    if (models.length === 0) {
        return (
            <input
                type="text"
                value={value}
                onChange={e => onChange(e.target.value)}
                placeholder="Enter model name..."
                className="w-full bg-base/50 border border-border/50 rounded px-3 py-2 text-sm"
            />
        );
    }

    return (
        <select
            value={value}
            onChange={e => onChange(e.target.value)}
            className="w-full bg-base/50 border border-border/50 rounded px-3 py-2 text-sm"
        >
            <option value="">Default model</option>
            {models.map(m => (
                <option key={m} value={m}>{m}</option>
            ))}
        </select>
    );
}
```

#### Worker注册表UI

**参考**: OpenFang的Hands Catalog grid（3列布局）

```
┌─────────────────────────────────────────────┐
│ 👥 Workers Registry                         │
│ [ + New Worker ]                            │
│                                             │
│ ┌──────────┐ ┌──────────┐ ┌──────────┐    │
│ │Code      │ │Researcher│ │Tester    │    │
│ │Reviewer  │ │          │ │          │    │
│ │          │ │          │ │          │    │
│ │Claude    │ │OpenCode  │ │Claude    │    │
│ │Sonnet-4  │ │GPT-4     │ │Haiku     │    │
│ │          │ │          │ │          │    │
│ │[Activate]│ │[Activate]│ │[Activate]│    │
│ └──────────┘ └──────────┘ └──────────┘    │
└─────────────────────────────────────────────┘
```

**实现代码** (`frontend/app/view/cowork/cowork-workers-registry.tsx`):

```typescript
import { useAtomValue } from "jotai";
import { CoworkViewModel } from "./cowork-model";

export function WorkersRegistryPanel({ model }: { model: CoworkViewModel }) {
    const workers = useAtomValue(model.workersAtom) ?? [];
    const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false);

    return (
        <div className="space-y-3">
            {/* 顶部操作栏 */}
            <div className="flex items-center justify-between">
                <h3 className="text-sm font-semibold">👥 Workers Registry</h3>
                <button
                    onClick={() => setIsCreateDialogOpen(true)}
                    className="px-3 py-1 bg-accent/80 text-primary rounded text-sm hover:bg-accent transition-colors cursor-pointer"
                >
                    + New Worker
                </button>
            </div>

            {/* Workers Grid - 3列布局 */}
            <div className="grid grid-cols-3 gap-3">
                {workers.map(worker => (
                    <WorkerCard
                        key={worker.workerid}
                        worker={worker}
                        onActivate={() => activateWorker(worker.workerid)}
                        onEdit={() => editWorker(worker.workerid)}
                    />
                ))}
            </div>

            {/* 空状态 */}
            {workers.length === 0 && (
                <div className="text-center py-8 border border-dashed border-border/50 rounded">
                    <p className="text-sm text-gray-500 mb-2">No workers configured</p>
                    <button
                        onClick={() => setIsCreateDialogOpen(true)}
                        className="text-sm text-accent hover:underline cursor-pointer"
                    >
                        Create your first worker →
                    </button>
                </div>
            )}

            {/* 创建对话框 */}
            {isCreateDialogOpen && (
                <WorkerCreateDialog
                    runtimes={detectedRuntimes}
                    onSave={handleCreateWorker}
                    onCancel={() => setIsCreateDialogOpen(false)}
                />
            )}
        </div>
    );
}

function WorkerCard({ worker, onActivate, onEdit }: { worker: CoworkWorker; onActivate: () => void; onEdit: () => void }) {
    const statusColors = {
        idle: "bg-gray-400",
        working: "bg-green-500",
        offline: "bg-red-500",
    };

    return (
        <div className="border border-border/50 rounded bg-base p-3 hover:border-border transition-colors">
            {/* 头部：状态 + 名称 */}
            <div className="flex items-start gap-2 mb-2">
                <span
                    className={`w-2 h-2 rounded-full ${statusColors[worker.status as keyof typeof statusColors] ?? "bg-gray-400"}`}
                />
                <div className="flex-1">
                    <div className="font-medium text-sm">{worker.name}</div>
                    {worker.description && (
                        <p className="text-xs text-gray-500 mt-0.5">{worker.description}</p>
                    )}
                </div>
            </div>

            {/* 中部：Runtime + Model */}
            <div className="flex flex-wrap gap-1.5 mb-3">
                <span className="px-1.5 py-0.5 bg-purple-900/50 text-purple-300 rounded text-xs capitalize">
                    {worker.tool}
                </span>
                {worker.model && (
                    <span className="px-1.5 py-0.5 bg-blue-900/50 text-blue-300 rounded text-xs">
                        {worker.model}
                    </span>
                )}
            </div>

            {/* 当前任务 */}
            {worker.assignedtask && (
                <div className="text-xs text-blue-400 mb-3 truncate">
                    → {worker.assignedtask}
                </div>
            )}

            {/* 底部：操作按钮 */}
            <div className="flex gap-2">
                {worker.status === "idle" ? (
                    <button
                        onClick={onActivate}
                        className="flex-1 px-2 py-1 bg-green-600/80 text-white rounded text-xs hover:bg-green-600 transition-colors cursor-pointer"
                    >
                        Activate
                    </button>
                ) : (
                    <button
                        onClick={() => pauseWorker(worker.workerid)}
                        className="flex-1 px-2 py-1 bg-yellow-600/80 text-white rounded text-xs hover:bg-yellow-600 transition-colors cursor-pointer"
                    >
                        Pause
                    </button>
                )}
                <button
                    onClick={onEdit}
                    className="px-2 py-1 bg-base/50 border border-border/50 rounded text-xs hover:bg-base/70 transition-colors cursor-pointer"
                >
                    ⚙️
                </button>
            </div>
        </div>
    );
}
```

---

## Phase 2: Task管理增强

### 2.1 Task手动分配

**现状**: 创建Task时只能等待Supervisor自动分配

**改进**: 创建Task时可以选择手动分配给特定Worker

#### UI设计

```
┌─────────────────────────────────────────────┐
│ Create New Task                             │
├─────────────────────────────────────────────┤
│ Title: [Fix login bug               ]       │
│                                             │
│ Description:                                │
│ ┌─────────────────────────────────────┐    │
│ │ Users report login fails when...    │    │
│ └─────────────────────────────────────┘    │
│                                             │
│ Priority: [Medium ▼]                        │
│                                             │
│ Assign to: [Auto-assign by Supervisor ▼]   │
│            ───────────────────────────     │
│            Code Reviewer (idle)            │
│            Researcher (working)             │
│            Tester (idle)                    │
│                                             │
│              [Cancel] [Create Task]         │
└─────────────────────────────────────────────┘
```

#### 实现代码

**修改现有的Task创建表单** (`frontend/app/view/cowork/cowork.tsx`):

```typescript
// 在CoworkView组件中修改
export function CoworkView({ model }: CoworkViewProps) {
    const workers = jotai.useAtomValue(model.workersAtom) ?? [];

    // 新增state
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

        // 重置表单
        setNewTaskTitle("");
        setNewTaskDesc("");
        setNewTaskPriority("medium");
        setAssignTo("");
    };

    return (
        <div className="flex flex-col h-full p-3 gap-3 overflow-auto">
            {/* ... 其他UI ... */}

            <div className="rounded border border-border/50 bg-base p-3">
                <h3 className="text-sm font-semibold mb-2">New Task</h3>

                {/* Title */}
                <input
                    className="w-full bg-base/50 border border-border/50 rounded px-2 py-1 text-sm mb-2"
                    placeholder="Task title"
                    value={newTaskTitle}
                    onChange={e => setNewTaskTitle(e.target.value)}
                />

                {/* Priority + Assign to */}
                <div className="flex gap-2 mb-2">
                    <select
                        className="flex-1 bg-base/50 border border-border/50 rounded px-2 py-1 text-sm"
                        value={newTaskPriority}
                        onChange={e => setNewTaskPriority(e.target.value)}
                    >
                        <option value="low">Low</option>
                        <option value="medium">Medium</option>
                        <option value="high">High</option>
                        <option value="urgent">Urgent</option>
                    </select>

                    {/* 新增：Assign to选择器 */}
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
                </div>

                {/* Description */}
                <textarea
                    className="w-full bg-base/50 border border-border/50 rounded px-2 py-1 text-sm mb-2"
                    placeholder="Description (optional)"
                    value={newTaskDesc}
                    onChange={e => setNewTaskDesc(e.target.value)}
                />

                {/* Create按钮 */}
                <button
                    className="w-full px-3 py-1.5 rounded bg-accent/80 text-primary hover:bg-accent transition-colors cursor-pointer text-sm"
                    onClick={handleCreateTask}
                >
                    Create Task
                </button>
            </div>
        </div>
    );
}
```

**修改createTask方法** (`frontend/app/view/cowork/cowork-model.ts`):

```typescript
async createTask(
    title: string,
    description: string,
    priority: string,
    assignedworker?: string  // 新增参数
): Promise<void> {
    await RpcApi.CoworkCreateTaskCommand(TabRpcClient, {
        title,
        description,
        priority,
        ...(assignedworker && { assignedworker })  // 可选字段
    });
    await this.refreshAllData();
}
```

---

### 2.2 Task执行干预

**现状**: 只能删除Worker，无法暂停/恢复Task

**改进**: 支持Pause/Resume/Cancel三个操作

#### UI设计

在Task Card上添加操作按钮：

```
┌─────────────────────────────────────────────┐
│ Task: Fix login bug                         │
│ ━━━━━━━━━━━━━━━━━━━━━━━━━━ 60%             │
│ → Code Reviewer (running)                   │
│                                             │
│ [⏸ Pause] [✕ Cancel]                       │
└─────────────────────────────────────────────┘
```

#### 实现代码

**修改TaskCard组件** (`frontend/app/view/cowork/cowork.tsx`):

```typescript
function TaskColumn({
    title,
    tasks,
    onDelete,
    onPause,
    onResume,
    onCancel,
}: {
    title: string;
    tasks: CoworkTask[];
    onDelete: (id: string) => void;
    onPause: (id: string) => void;   // 新增
    onResume: (id: string) => void;  // 新增
    onCancel: (id: string) => void;  // 新增
    priorityColors: Record<string, string>;
}) {
    return (
        <div className="rounded border border-border/50 bg-base p-2">
            <h4 className="text-xs font-semibold mb-2">
                {title} ({tasks.length})
            </h4>
            <div className="flex flex-col gap-1">
                {tasks.map((t) => (
                    <div key={t.taskid} className="text-xs p-1 rounded bg-base/30">
                        <div className="flex items-center gap-1">
                            <span className={priorityColors[t.priority] ?? ""}>{t.title}</span>
                        </div>

                        {/* 进度条 */}
                        {t.status === "working" && t.progress && (
                            <div className="mt-1">
                                <div className="h-1 bg-gray-800 rounded-full overflow-hidden">
                                    <div
                                        className="h-full bg-green-500 transition-all duration-300"
                                        style={{ width: `${t.progress}%` }}
                                    />
                                </div>
                                <p className="text-gray-500 text-[10px] mt-0.5">{t.progress}%</p>
                            </div>
                        )}

                        {/* 操作按钮 */}
                        <div className="flex gap-1 mt-1">
                            {t.status === "working" && (
                                <>
                                    <button
                                        onClick={() => onPause(t.taskid)}
                                        className="px-1.5 py-0.5 bg-yellow-600/80 text-white rounded text-[10px] hover:bg-yellow-600 transition-colors cursor-pointer"
                                    >
                                        ⏸
                                    </button>
                                    <button
                                        onClick={() => onCancel(t.taskid)}
                                        className="px-1.5 py-0.5 bg-red-600/80 text-white rounded text-[10px] hover:bg-red-600 transition-colors cursor-pointer"
                                    >
                                        ✕
                                    </button>
                                </>
                            )}
                            {t.status === "paused" && (
                                <>
                                    <button
                                        onClick={() => onResume(t.taskid)}
                                        className="px-1.5 py-0.5 bg-green-600/80 text-white rounded text-[10px] hover:bg-green-600 transition-colors cursor-pointer"
                                    >
                                        ▶
                                    </button>
                                    <button
                                        onClick={() => onCancel(t.taskid)}
                                        className="px-1.5 py-0.5 bg-red-600/80 text-white rounded text-[10px] hover:bg-red-600 transition-colors cursor-pointer"
                                    >
                                        ✕
                                    </button>
                                </>
                            )}
                            <button
                                onClick={() => onDelete(t.taskid)}
                                className="ml-auto text-gray-500 hover:text-red-400 cursor-pointer"
                            >
                                ×
                            </button>
                        </div>
                    </div>
                ))}
            </div>
        </div>
    );
}
```

**在CoworkView中添加handler**:

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
```

**后端RPC实现** (`pkg/wshrpc/wshrpctypes.go`):

```go
// CoworkPauseTaskCommand 暂停Task
type CoworkPauseTaskCommand struct {
    Command
    TaskID string `json:"taskid"`
}

func (c *CoworkPauseTaskCommand) Run(ctx context.Context, m *Mailbox) error {
    // 1. 查找Task对应的Worker
    // 2. 发送SIGSTOP到Worker进程
    // 3. 更新Task状态为"paused"
    // 4. 发送cowork:taskupdate事件
    return nil
}

// CoworkResumeTaskCommand 恢复Task
type CoworkResumeTaskCommand struct {
    Command
    TaskID string `json:"taskid"`
}

func (c *CoworkResumeTaskCommand) Run(ctx context.Context, m *Mailbox) error {
    // 1. 查找Task对应的Worker
    // 2. 发送SIGCONT到Worker进程
    // 3. 更新Task状态为"working"
    // 4. 发送cowork:taskupdate事件
    return nil
}

// CoworkCancelTaskCommand 取消Task
type CoworkCancelTaskCommand struct {
    Command
    TaskID string `json:"taskid"`
}

func (c *CoworkCancelTaskCommand) Run(ctx context.Context, m *Mailbox) error {
    // 1. 更新Task状态为"cancelled"
    // 2. 如果Worker正在运行，发送SIGTERM
    // 3. 发送cowork:taskupdate事件
    return nil
}
```

---

## Phase 3: UI整合与体验优化

### 3.1 简化AI Panel的Cowork入口

**现状**: CoworkWorkersPanel在AI Panel显示完整状态，与CoworkView重复

**改进**: AI Panel只显示摘要 + 快速跳转按钮

#### UI设计

**AI Panel** (简化后):

```
┌─────────────────────────────────────────────┐
│ AI Panel                                    │
├─────────────────────────────────────────────┤
│ [Chat] [Settings]                           │
├─────────────────────────────────────────────┤
│ 👥 Cowork Mode        (2 active, 3 pending) │
│                     [Open Cowork View →]    │
└─────────────────────────────────────────────┘
```

**实现代码** (`frontend/app/aipanel/cowork-summary-panel.tsx`):

```typescript
import { useNavigate } from "react-router-dom";

export function CoworkSummaryPanel() {
    const navigate = useNavigate();
    const { data } = useCoworkSummary();  // 自定义hook，只获取摘要数据

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
```

---

### 3.2 CoworkView布局优化

**参考**: OpenFang的Dashboard布局

#### 新布局设计

```
┌─────────────────────────────────────────────┐
│ Cowork Mode                                 │
├─────────────────────────────────────────────┤
│ 🔍 AI CLI Detection   👥 Workers Registry  │
│ [Detect]              [+ New Worker]        │
│ ● Claude (1.2.3)      [Card Grid - 3列]     │
│ ● OpenCode (2.4.5)                           │
├─────────────────────────────────────────────┤
│ 🎯 Task Board                               │
│ ┌─────────┬─────────┬─────────┬─────────┐ │
│ │ Pending │ Working │   Done  │ Failed  │ │
│ │   (3)   │   (2)   │  (10)   │  (1)    │ │
│ └─────────┴─────────┴─────────┴─────────┘ │
├─────────────────────────────────────────────┤
│ 📊 Activity Log                             │
│ [2026-04-27 10:30] Task assigned to...     │
└─────────────────────────────────────────────┘
```

#### 实现代码

**重新组织CoworkView** (`frontend/app/view/cowork/cowork.tsx`):

```typescript
export function CoworkView({ model }: CoworkViewProps) {
    return (
        <div className="flex flex-col h-full p-3 gap-4 overflow-auto">
            {/* 顶部：Runtime检测 + Workers注册表（两列布局） */}
            <div className="grid grid-cols-2 gap-4">
                <RuntimeDetectionPanel />
                <WorkersRegistryPanel model={model} />
            </div>

            {/* 中部：Task Board */}
            <div>
                <h3 className="text-sm font-semibold mb-2">🎯 Task Board</h3>
                <div className="grid grid-cols-4 gap-2">
                    <TaskColumn
                        title="Pending"
                        tasks={pendingTasks}
                        statusColors={priorityColors}
                    />
                    <TaskColumn
                        title="Working"
                        tasks={workingTasks}
                        statusColors={priorityColors}
                    />
                    <TaskColumn
                        title="Done"
                        tasks={doneTasks}
                        statusColors={priorityColors}
                    />
                    <TaskColumn
                        title="Failed"
                        tasks={failedTasks}
                        statusColors={priorityColors}
                    />
                </div>
            </div>

            {/* 底部：Activity Log */}
            <div>
                <h3 className="text-sm font-semibold mb-2">📊 Activity Log</h3>
                <ActivityLog activities={activities} />
            </div>
        </div>
    );
}
```

---

### 3.3 新任务创建快速入口

在Task Board上方添加快速创建表单：

```
┌─────────────────────────────────────────────┐
│ Quick Create Task                           │
│ ┌──────────────────────────────────────┐   │
│ │ Title: [Fix login bug           ]    │   │
│ │ Priority: [Medium ▼] Assign: [Auto ▼]│   │
│ │ [Create Task]                        │   │
│ └──────────────────────────────────────┘   │
└─────────────────────────────────────────────┘
```

---

## 数据结构定义

### 前端类型定义

```typescript
// frontend/app/view/cowork/cowork-types.ts

// Runtime检测结果
interface AIRuntime {
    name: string;        // "claude", "opencode", "cursor", "aider"
    displayName: string; // "Claude Code", "OpenCode", "Cursor Agent", "Aider"
    command: string;     // 检测到的完整路径或命令名
    version: string;     // 版本号
    status: string;      // "online" | "offline"
}

// Worker定义（增强版）
interface CoworkWorker {
    // 现有字段
    workerid: string;
    name: string;
    tool: string;          // "claude" | "opencode" | "cursor" | "aider"
    status: string;        // "idle" | "working" | "offline" | "error" | "paused"
    assignedtask?: string;
    role?: string;         // 保留（未来扩展）
    desc?: string;         // 保留（未来扩展）

    // 新增字段
    model?: string;              // 使用的模型
    instructions?: string;       // System prompt
    customEnv?: Record<string, string>;  // 环境变量
    customArgs?: string[];       // CLI参数
    description?: string;        // Worker描述
    createdAt?: number;          // 创建时间戳
    lastActiveAt?: number;       // 最后活跃时间
}

// Task定义（增强版）
interface CoworkTask {
    taskid: string;
    title: string;
    description?: string;
    priority: string;      // "low" | "medium" | "high" | "urgent"
    status: string;        // "pending" | "assigned" | "working" | "done" | "failed" | "paused" | "cancelled"
    assignedworker?: string;
    progress?: number;     // 0-100
    result?: string;
    error?: string;
    createdat: number;
    updatedat: number;

    // 新增字段
    attempt?: number;      // 当前尝试次数
    maxAttempts?: number;  // 最大重试次数
    pausedAt?: number;     // 暂停时间
    resumedAt?: number;    // 恢复时间
    cancelledAt?: number;  // 取消时间
}

// Activity Log
interface CoworkActivity {
    id: string;
    type: string;          // "task_assign" | "worker_create" | "task_update" | "error"
    description: string;
    createdat: number;
}
```

### 后端Go结构体

```go
// pkg/wshrpc/wshrpctypes.go

// AIRuntime AI运行时信息
type AIRuntime struct {
    Name        string `json:"name"`
    DisplayName string `json:"display_name"`
    Command     string `json:"command"`
    Version     string `json:"version"`
    Status      string `json:"status"`
}

// CoworkWorkerUpdate 更新Worker配置
type CoworkWorkerUpdate struct {
    WorkerID     string            `json:"workerid"`
    Name         string            `json:"name,omitempty"`
    Description  string            `json:"description,omitempty"`
    Model        string            `json:"model,omitempty"`
    Instructions string            `json:"instructions,omitempty"`
    CustomEnv    map[string]string `json:"custom_env,omitempty"`
    CustomArgs   []string          `json:"custom_args,omitempty"`
}

// CoworkTaskUpdate 更新Task状态
type CoworkTaskUpdate struct {
    TaskID    string `json:"taskid"`
    Status    string `json:"status,omitempty"`
    Progress  int    `json:"progress,omitempty"`
    Result    string `json:"result,omitempty"`
    Error     string `json:"error,omitempty"`
    Attempt   int32  `json:"attempt,omitempty"`
}
```

---

## RPC API设计

### 新增RPC命令

| 命令 | 功能 | 参数 | 返回值 |
|------|------|------|--------|
| `CoworkDetectRuntimesCommand` | 检测AI CLI | 无 | `[]AIRuntime` |
| `CoworkUpdateWorkerCommand` | 更新Worker配置 | `CoworkWorkerUpdate` | 无 |
| `CoworkPauseTaskCommand` | 暂停Task | `taskid` | 无 |
| `CoworkResumeTaskCommand` | 恢复Task | `taskid` | 无 |
| `CoworkCancelTaskCommand` | 取消Task | `taskid` | 无 |
| `CoworkGetWorkerConfigCommand` | 获取Worker配置 | `workerid` | `CoworkWorker` |

### 修改现有RPC命令

| 命令 | 修改内容 |
|------|----------|
| `CoworkCreateTaskCommand` | 增加`assignedworker`可选参数 |
| `CoworkRegisterWorkerCommand` | 增加`model/instructions/customEnv/customArgs`字段 |

### RPC示例

```go
// CoworkDetectRuntimesCommand 检测系统中可用的AI CLI
type CoworkDetectRuntimesCommand struct {
    Command
}

type CoworkDetectRuntimesCommandReturn struct {
    Runtimes []AIRuntime `json:"runtimes"`
}

func (c *CoworkDetectRuntimesCommand) Run(ctx context.Context, m *Mailbox, returnValue *CoworkDetectRuntimesCommandReturn) error {
    // 实现见上文
    return nil
}

// CoworkUpdateWorkerCommand 更新Worker配置
type CoworkUpdateWorkerCommand struct {
    Command
    WorkerID     string            `json:"workerid"`
    Name         string            `json:"name,omitempty"`
    Description  string            `json:"description,omitempty"`
    Model        string            `json:"model,omitempty"`
    Instructions string            `json:"instructions,omitempty"`
    CustomEnv    map[string]string `json:"custom_env,omitempty"`
    CustomArgs   []string          `json:"custom_args,omitempty"`
}

func (c *CoworkUpdateWorkerCommand) Run(ctx context.Context, m *Mailbox) error {
    // 更新Worker配置
    return nil
}
```

---

## 实现路线图

### Phase 1 (P0) - Runtime检测与Worker注册

**目标**: 用户能检测AI CLI并创建配置丰富的Worker

**任务**:
1. ✅ 实现`CoworkDetectRuntimesCommand` RPC
2. ✅ 创建`RuntimeDetectionPanel`组件
3. ✅ 创建`WorkerCreateDialog`组件
4. ✅ 创建`WorkersRegistryPanel`组件
5. ✅ 扩展`CoworkWorker`类型定义
6. ✅ 修改`CoworkRegisterWorkerCommand`支持新字段
7. ✅ 集成到`CoworkView`

**预计工作量**: 2-3天

---

### Phase 2 (P1) - Task管理增强

**目标**: 用户能手动分配Task并干预执行

**任务**:
1. ✅ 修改`CoworkCreateTaskCommand`增加`assignedworker`参数
2. ✅ 修改Task创建表单，添加Worker选择器
3. ✅ 实现`CoworkPauseTaskCommand` RPC
4. ✅ 实现`CoworkResumeTaskCommand` RPC
5. ✅ 实现`CoworkCancelTaskCommand` RPC
6. ✅ 在TaskCard添加Pause/Resume/Cancel按钮
7. ✅ 扩展`CoworkTask`类型定义

**预计工作量**: 2天

---

### Phase 3 (P1) - UI整合与体验优化

**目标**: 简化入口，统一布局

**任务**:
1. ✅ 简化AI Panel的`CoworkWorkersPanel`为摘要面板
2. ✅ 重新组织`CoworkView`布局（两列顶部区域）
3. ✅ 添加快速创建Task表单
4. ✅ 优化空状态展示
5. ✅ 添加加载状态

**预计工作量**: 1-2天

---

### 总计

**总工作量**: 5-7天

**实施建议**:
1. 先完成Phase 1，建立基础设施
2. 再完成Phase 2，增强Task控制
3. 最后完成Phase 3，优化用户体验

---

## 附录：参考系统对比

### OpenFang关键设计

| 特性 | 实现 | 可借鉴点 |
|------|------|----------|
| HAND.toml配置 | 三层结构（Metadata/Settings/Agent） | Worker配置结构化 |
| Hands Catalog | 3列grid卡片布局 | Workers Registry UI |
| Running Now strip | 顶部活跃实例横条 | 实时状态展示 |
| Autonomous执行 | max_iterations触发Continuous | **不适用**（Cowork是term块执行）|
| 暂停/恢复 | hand pause/resume | Task干预操作 |

### Multica关键设计

| 特性 | 实现 | 可借鉴点 |
|------|------|----------|
| Runtime检测 | Daemon启动时检测 | 手动触发检测 |
| Agent模板 | CreateAgentRequest | Worker配置字段 |
| Task Board | 状态流转清晰 | Task状态机 |
| 手动分配 | create agent时可选择 | 创建Task时可选择Worker |
| Cancel Task | cancel机制 | Task取消操作 |

### 过滤不适用的设计

| 设计 | 原系统 | 为何不适用 |
|------|--------|------------|
| Workspace可见性 | Multica | WaveAI是个人工具 |
| Daemon后台执行 | OpenFang/Multica | Cowork使用term块执行 |
| 多实例管理 | OpenFang | 个人使用场景不需要 |
| 心跳保持在线 | Multica | Cowork不常驻进程 |

---

## 版本历史

| 版本 | 日期 | 变更 |
|------|------|------|
| v1.0 | 2026-04-27 | 初始版本 |

---

## 文档维护

**作者**: AI (基于OpenFang和Multica研究)
**状态**: 设计阶段，待实施
**反馈**: 请在实际实施后更新本文档

