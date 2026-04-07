# ZeroAI Panel — 完整调查分析与实施计划

## 一、当前状态调查总结

### 1.1 代码架构全景

ZeroAI 已经有一套相对完整的独立实现,分布在以下层次:

| 层次              | 路径                                                 | 状态                                                                   |
| ----------------- | ---------------------------------------------------- | ---------------------------------------------------------------------- |
| **前端面板**      | `frontend/app/zeroai/aipanel.tsx`                    | ✅ 已有基础UI                                                          |
| **前端组件**      | `frontend/app/zeroai/components/`                    | ✅ Header, ChatArea, ChatInput, AgentList, StatusBar                   |
| **前端Store**     | `frontend/app/zeroai/models/`                        | ✅ provider-model, message-model, session-model, ui-model, agent-model |
| **前端RPC客户端** | `frontend/app/zeroai/store/zeroai-client.ts`         | ✅ 完整RPC封装                                                         |
| **前端View**      | `frontend/app/view/zeroai/zeroai.tsx`                | ✅ Block注册                                                           |
| **后端RPC类型**   | `pkg/wshrpc/wshrpctypes.go`, `wshrpctypes_zeroai.go` | ✅ 类型定义完整                                                        |
| **后端RPC客户端** | `pkg/wshrpc/wshclient/wshclient.go`                  | ✅ 客户端stub                                                          |
| **后端RPC服务端** | `pkg/zeroai/rpc/wshserver-zeroai.go`                 | ✅ 完整RPC handler                                                     |
| **ACP协议层**     | `pkg/zeroai/protocol/`                               | ✅ acp-connection.go, acp-adapter.go, acp-config.go                    |
| **Agent层**       | `pkg/zeroai/agent/acp-agent.go`                      | ✅ ACP agent封装                                                       |
| **Service层**     | `pkg/zeroai/service/`                                | ✅ agent-service, message-service, session-service, provider-service   |
| **Store层**       | `pkg/zeroai/store/`                                  | ✅ session-store, message-store, team-store, db-migrations             |
| **Process层**     | `pkg/zeroai/process/`                                | ✅ process-manager, process-spawner                                    |
| **数据库迁移**    | `db/migrations-zeroai/`                              | ✅ init + team tables                                                  |
| **配置Schema**    | `schema/zeroai.json`                                 | ✅ 已有                                                                |

**关键发现**: ZeroAI 已经是一个独立模块,与 WaveAI 几乎没有代码交叉。`pkg/zeroai/` 是独立目录,前端在 `frontend/app/zeroai/` 独立目录。隔离性已经很好。

### 1.2 WaveAI vs ZeroAI 流式输出对比

| 特性         | WaveAI                                                                    | ZeroAI                                                                                          |
| ------------ | ------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------- |
| **流式框架** | `@ai-sdk/react` 的 `useChat()` hook                                       | 自定义 `for await` 循环                                                                         |
| **stop功能** | `useChat()` 内置 `stop()` 函数                                            | `AbortController.abort()` + 后端 `cancelStream()`                                               |
| **状态管理** | `status` 由 `useChat()` 自动管理 (`"streaming"`, `"submitted"`, `"idle"`) | 手动 `globalStore.set(isStreamingAtom, true/false)`                                             |
| **取消机制** | SDK内部处理                                                               | `cancelRef.current.abort()` → `clientRef.current.cancelStream(sessionId)` → `ag.CancelPrompt()` |

### 1.3 "Stuck Typing" Bug 根因分析

**问题**: 输入消息后AI一直处于"typing"状态,停止按钮无响应。

**根因定位 — 三层问题**:

#### Bug #1: 前端 `handleStopStreaming` 与 `handleSendMessage` 的竞态条件

在 `aipanel.tsx` 中:

```typescript
// handleSendMessage 中
try {
  for await (const event of stream) {
    // ...处理事件
  }
} catch (error) {
  // ...
} finally {
  // finally 块会执行
  dispatchMessageAction({ type: "finalizeStream", sessionId });
  globalStore.set(isStreamingAtom, false); // ← 重置状态
  setThinking(false);
}
```

**问题**: `for await` 循环在以下情况会**永久阻塞**:

1. 后端 goroutine 没有正确关闭 channel
2. 后端的 `eventCh` 没有发送 `end_turn` 事件
3. 后端 `SendMessage` 返回的 channel 没有被关闭

查看 `wshserver-zeroai.go` 的 `ZeroAiSendStreamMessageCommand`:

```go
for event := range eventCh {
    select {
    case <-ctx.Done():
        return  // ← 这里return后,channel rtn没有关闭!
    default:
    }
    // ...
    if event.Type == agent.EventTypeEndTurn {
        break  // ← break后,goroutine继续执行defer close(rtn)
    }
}
```

**关键问题**: 当 `ctx.Done()` 被触发(cancelStream调用),goroutine 直接 `return`,但 **`defer close(rtn)` 仍然会执行**。然而前端 `for await` 循环可能在 abort 后已经跳出,但 **`isStreamingAtom` 可能没有被正确重置**因为:

#### Bug #2: 前端 `handleStopStreaming` 没有设置 `cancelRef.current = null`

```typescript
const handleStopStreaming = async () => {
  if (cancelRef.current) {
    cancelRef.current.abort(); // ← 仅abort
  }
  // ... 调用后端cancel
  // ← 但没有设置 cancelRef.current = null!
  globalStore.set(isStreamingAtom, false);
  setThinking(false);
};
```

而 `handleSendMessage` 的 finally 块中:

```typescript
finally {
    clearTimeout(streamTimeout);
    cancelRef.current = null;  // ← 只有这里会清理
    // ...
}
```

**时序问题**: 用户点停止 → `handleStopStreaming` abort → `handleSendMessage` 的 `for await` 因 `abortController.signal.aborted` break → finally 执行 → 重置状态。这个路径**理论上是通的**,但存在:

#### Bug #3: 后端 `CancelPrompt` 不保证 goroutine 立即退出

`acp-agent.go`:

```go
func (a *AcpAgent) CancelPrompt() {
    a.mu.Lock()
    if a.cancelCtx != nil {
        a.cancelCtx()  // ← 取消promptCtx
        a.cancelCtx = nil
    }
    a.status.IsStreaming = false
    a.mu.Unlock()
}
```

但 `SendMessage` 中的 done watcher:

```go
go func() {
    defer func() {
        // ... close eventCh
    }()
    doneCh := a.conn.WaitForDone()
    select {
    case <-promptCtx.Done():  // ← CancelPrompt触发这里
    case <-doneCh:
    }
    a.sendEvent(sessionID, AgentEvent{Type: EventTypeEndTurn, ...})
    // ← 发送end_turn后,defer才关闭eventCh
}()
```

**问题**: `CancelPrompt` 后,done watcher goroutine 仍然会发送 `end_turn` 事件,但此时 `eventCh` 可能已经不存在(被主流程关闭),导致 `sendEvent` 的 `ch <- event` 阻塞(虽然有100ms超时保护)。

#### Bug #4: RPC channel 关闭时序

在 `ZeroAiSendStreamMessageCommand` 中,当 cancel 发生时:

1. 前端 `AbortController.abort()` 导致前端 `for await` 跳出
2. 前端调用 `cancelStream(sessionId)` → 后端 `ZeroAiCancelStreamCommand` → `ag.CancelPrompt()`
3. 但此时 `ZeroAiSendStreamMessageCommand` 的 goroutine **可能还在往 `rtn` channel 写数据**
4. 前端已经断开 `for await`,没人读取 `rtn` channel
5. `rtn` channel 满了(缓冲区)→ goroutine 阻塞 → **永远不会执行 `defer close(rtn)`**

**结论**: 最核心的 bug 是 **后端 streaming goroutine 在 context cancel 后没有优雅退出**,导致 channel 永远不会被关闭,前端的 `for await` 虽然因 abort 跳出,但后端资源泄漏,下一次发送时状态混乱。

### 1.4 ACP 协议理解

ACP (Agent Control Protocol) 是一个基于 JSON-RPC 的 stdio 协议:

```
初始化流程:
1. Client → Agent: {"jsonrpc":"2.0","id":1,"method":"initialize","params":{...}}
2. Agent → Client: {"jsonrpc":"2.0","id":1,"result":{...}}
3. Client → Agent: {"jsonrpc":"2.0","method":"notifications/initialized"}

会话流程:
4. Client → Agent: {"jsonrpc":"2.0","id":2,"method":"session/new","params":{...}}
5. Agent → Client: {"jsonrpc":"2.0","id":2,"result":{"sessionId":"xxx"}}

对话流程:
6. Client → Agent: {"jsonrpc":"2.0","id":3,"method":"session/prompt","params":{"sessionId":"xxx","prompt":[...]}}
7. Agent → Client: {"jsonrpc":"2.0","method":"session/update","params":{"update":{"sessionUpdate":"text_chunk","content":"..."}}}  (多次)
8. Agent → Client: {"jsonrpc":"2.0","method":"session/update","params":{"update":{"sessionUpdate":"end_turn"}}}
9. Agent → Client: {"jsonrpc":"2.0","id":3,"result":{...}}  (RPC响应,表示prompt完成)

取消流程:
- 没有专门的 cancel RPC,通过关闭连接/进程或忽略后续update实现
```

**与AionUi参考实现的对比**: AionUi 的 TypeScript 实现(`AcpConnection.ts`)与当前 Go 实现(`acp-connection.go`)在协议层面完全一致,都是 `initialize → session/new → session/prompt → session/update notifications → end_turn`。差异在于:

- AionUi 在前端直接通过 stdio 连接 agent
- 当前 Go 实现在后端管理 agent 进程,前端通过 RPC 间接通信

### 1.5 ClawTeam 协作模式分析

ClawTeam 的核心架构:

```
Leader Agent (Claude Code / Codex等)
    │
    ├── 通过 `clawteam spawn` 创建 Worker Agents
    │   └── 每个 Worker = 独立 tmux 窗口 + git worktree + 协调prompt
    │
    ├── 任务管理: clawteam task create/update/list
    │   └── 支持依赖链: --blocked-by → 自动解锁
    │
    ├── 消息系统: clawteam inbox send/receive
    │   └── 基于文件系统或 ZeroMQ P2P
    │
    └── 监控: clawteam board show/attach/serve
```

**关键可复刻的模式**:

1. **协调Prompt**: 每个 Worker 启动时注入一段 prompt,教它如何用 CLI 命令检查任务、更新状态、发消息
2. **任务状态机**: `pending → in_progress → completed/blocked`,依赖自动解锁
3. **心跳/超时检测**: 通过 `clawteam lifecycle idle` 报告空闲状态
4. **终端集成**: 使用 tmux 管理独立会话

**在 Wave Terminal 中的替代方案**:

- tmux → Wave Terminal 的 `wsh terminal` 命令创建新终端 block
- 文件系统消息 → SQLite 数据库 (已有 `migrations-zeroai`)
- `clawteam` CLI → 内置到 zeroai 服务中,通过 RPC 暴露

---

## 二、功能需求可行性分析

### 需求1: 对接各种 Agent CLI (claude code, opencode, codex, gemini, qwen)

**可行性**: ✅ 完全可行

**方案**:

- 利用已有的 ACP 协议层 (`pkg/zeroai/protocol/`)
- 为每个 Agent CLI 配置 ACP 连接参数 (CLI路径, 启动参数, 环境变量)
- 参考 AionUi 的 agent 适配层,在 `pkg/zeroai/protocol/acp-config.go` 中扩展配置

**已有基础**: `acp-config.go` 中已有 `BackendConfig` 结构,支持 `CliCommand`, `DefaultCliPath`, `Transport`, `AcpArgs`。

**需要补充**:

- 各 Agent CLI 的 ACP 适配配置 (claude code 的 `claude acp` 子命令等)
- 非 ACP 协议 Agent 的 fallback (通过终端输入/输出解析)

### 需求2: 自定义 LLM 提供商接入 (OpenAI 兼容)

**可行性**: ✅ 完全可行,已有基础

**方案**:

- 复用 `pkg/zeroai/service/provider-service.go` 的自定义 Provider 系统
- OpenAI 兼容的 LLM 通过标准 OpenAI API 调用,不走 ACP 进程
- 已有的 `ZeroAiSaveProviderCommand`, `ZeroAiTestProviderCommand` 支持

### 需求3: 多 Agent 协同

**可行性**: ✅ 可行,但需要新建

**方案 — 在 Wave Terminal 中复刻 ClawTeam**:

```
ZeroAI Coordinator (Go后端)
    │
    ├── AgentRole (角色定义: 架构师, 开发者, 测试员...)
    │   ├── AGENT.md / MEMORY.md / SOUL.md 提示词
    │   ├── 可选技能/MCP 库
    │   └── 绑定的 CLI (claude, opencode...)
    │
    ├── TerminalSession (终端会话管理)
    │   ├── 通过 wsh 创建新终端 block
    │   ├── 监控终端输出 (判断假死/中断)
    │   └── 向终端输入自然语言唤醒
    │
    ├── TaskManager (任务管理)
    │   ├── Kanban: pending → in_progress → completed
    │   ├── 依赖链支持
    │   └── 自动唤醒/分配
    │
    └── Supervisor (监督器)
        ├── 心跳检测 (最后活动时间)
        ├── 终端输出分析 (检测错误/假死)
        └── 自动恢复 (重试/唤醒/重启)
```

**前端 UI**:

- 左侧: Agent 角色列表 (可创建新角色)
- 右侧: 终端区域 (每个 Agent 对应一个 terminal block)
- 点击角色 → 激活对应终端 (已启动则聚焦,未启动则新建)
- AI 智能创建角色的工具接口 (通过 RPC 暴露 `CreateAgentRole`)

---

## 三、实施计划

### Phase 0: 修复 "Stuck Typing" Bug (最高优先级)

**目标**: 解决流式输出卡死和停止按钮无响应问题

#### Task 0.1: 修复后端 Streaming Channel 泄漏

**文件**: `pkg/zeroai/rpc/wshserver-zeroai.go`

问题: `ZeroAiSendStreamMessageCommand` 的 goroutine 在 context cancel 后不优雅退出。

修复:

```go
func (zs *WshRpcZeroaiServer) ZeroAiSendStreamMessageCommand(ctx context.Context, req ...) chan ... {
    rtn := make(chan wshrpc.RespOrErrorUnion[wshrpc.ZeroAiStreamMessageEvent])

    go func() {
        defer close(rtn)  // ✅ 确保始终关闭

        // 使用 context 派生可取消的上下文
        streamCtx, streamCancel := context.WithCancel(ctx)
        defer streamCancel()

        // ... 获取 agent ...

        eventCh, err := ag.SendMessage(streamCtx, req.SessionID, input)
        if err != nil {
            sendError(rtn, err)
            return
        }

        for {
            select {
            case <-streamCtx.Done():
                return  // defer close(rtn) 会执行
            case event, ok := <-eventCh:
                if !ok {
                    return  // channel 关闭,优雅退出
                }
                // ... 发送 event 到 rtn ...
                if event.Type == agent.EventTypeEndTurn {
                    return  // 正常结束
                }
            }
        }
    }()

    return rtn
}
```

#### Task 0.2: 修复前端 Stop 按钮

**文件**: `frontend/app/zeroai/aipanel.tsx`

修复 `handleStopStreaming`:

```typescript
const handleStopStreaming = async () => {
  if (cancelRef.current) {
    cancelRef.current.abort();
    cancelRef.current = null; // ← 补充这行
  }
  const sessionId = activeSessionId;
  if (sessionId && clientRef.current) {
    try {
      await clientRef.current.cancelStream(sessionId);
    } catch (error) {
      console.error("Failed to cancel stream:", error);
    }
    dispatchMessageAction({ type: "finalizeStream", sessionId });
  }
  globalStore.set(isStreamingAtom, false);
  setThinking(false);
};
```

#### Task 0.3: 增强前端超时保护

**文件**: `frontend/app/zeroai/aipanel.tsx`

当前超时是 10 分钟,改为 3 分钟,并在超时后**强制调用 handleStopStreaming**:

```typescript
const streamTimeout = setTimeout(
  async () => {
    console.log("[ZeroAI] stream timeout safety net, force stopping");
    await handleStopStreaming(); // ← 调用完整的停止流程
  },
  3 * 60 * 1000
);
```

#### Task 0.4: 后端 CancelPrompt 增强

**文件**: `pkg/zeroai/agent/acp-agent.go`

确保 CancelPrompt 后,done watcher goroutine 不会卡在 sendEvent:

```go
func (a *AcpAgent) CancelPrompt() {
    a.mu.Lock()
    if a.cancelCtx != nil {
        a.cancelCtx()
        a.cancelCtx = nil
    }
    a.status.IsStreaming = false
    // 额外: 关闭当前 session 的 eventCh,让 done watcher 的 sendEvent 快速失败
    a.mu.Unlock()
}
```

### Phase 1: Agent CLI 对接完善

**目标**: 支持 claude code, opencode, codex, gemini, qwen

#### Task 1.1: 扩展 ACP 后端配置

**文件**: `pkg/zeroai/protocol/acp-config.go`

为每个 Agent CLI 添加配置:

```go
var builtinBackends = map[string]*BackendConfig{
    "claude": {
        Name:           "Claude Code",
        DefaultCliPath: "claude",
        AcpArgs:        []string{"acp"},  // claude acp 子命令
        Transport:      "stdio",
    },
    "opencode": {
        Name:           "OpenCode",
        DefaultCliPath: "opencode",
        AcpArgs:        []string{"acp"},
        Transport:      "stdio",
    },
    "codex": {
        Name:           "OpenAI Codex",
        DefaultCliPath: "codex",
        AcpArgs:        []string{"acp"},
        Transport:      "stdio",
    },
    "gemini": {
        Name:           "Gemini CLI",
        DefaultCliPath: "gemini",
        AcpArgs:        []string{"acp"},
        Transport:      "stdio",
    },
    "qwen": {
        Name:           "Qwen Code",
        DefaultCliPath: "qwen",
        AcpArgs:        []string{"acp"},
        Transport:      "stdio",
    },
}
```

#### Task 1.2: 自定义 LLM Provider UI

**文件**: `frontend/app/zeroai/components/` (已有 ProviderSettings)

已有 `ProviderSettings` 组件,需要确保:

- 可以配置 OpenAI 兼容的 API endpoint
- 可以配置 API key, model list
- 测试连接功能 (`testProvider`)

### Phase 2: 多 Agent 协同系统

**目标**: 复刻 ClawTeam 核心协作能力

#### Task 2.1: Agent 角色系统

**新建文件**:

- `pkg/zeroai/team/role.go` — AgentRole 定义 (name, system prompt, skills, mcp)
- `pkg/zeroai/team/role-store.go` — 角色存储 (SQLite)
- `frontend/app/zeroai/components/agent-role-panel.tsx` — 左侧角色列表 UI
- `frontend/app/zeroai/components/agent-role-creator.tsx` — 创建角色对话框

**数据结构**:

```go
type AgentRole struct {
    ID           string   `json:"id"`
    Name         string   `json:"name"`           // "架构师", "开发者"
    Description  string   `json:"description"`
    SystemPrompt string   `json:"systemPrompt"`   // AGENT.md 内容
    MemoryPrompt string   `json:"memoryPrompt"`   // MEMORY.md 内容
    SoulPrompt   string   `json:"soulPrompt"`     // SOUL.md 内容
    Skills       []string `json:"skills"`          // 已选技能
    MCPServers   []string `json:"mcpServers"`      // 已选 MCP
    BoundCLI     string   `json:"boundCli"`        // 绑定的 CLI (claude, opencode...)
    BoundModel   string   `json:"boundModel"`      // 绑定的模型
}
```

#### Task 2.2: 终端会话管理

**新建文件**:

- `pkg/zeroai/team/terminal-manager.go` — 通过 wsh 管理终端会话
- `pkg/zeroai/team/terminal-monitor.go` — 监控终端状态 (假死检测)

**核心逻辑**:

```go
// 创建终端会话 (通过 wsh)
func (tm *TerminalManager) CreateSession(role *AgentRole) (*TerminalSession, error) {
    // 1. 通过 wsh 创建新 terminal block
    // 2. 在终端中运行绑定的 CLI (如 claude acp 或直接 claude)
    // 3. 注入协调 prompt (类似 ClawTeam 的 coordination prompt)
    // 4. 返回 session ID 和 block ID
}

// 监控终端状态
func (tm *TerminalManager) MonitorSession(sessionID string) {
    // 1. 定期检查终端最后输出
    // 2. 如果超过阈值无活动 → 标记为可能假死
    // 3. 读取最后一段上下文判断状态
    // 4. 如果假死 → 发送自然语言唤醒 ("请继续你的工作...")
    // 5. 如果错误 → 记录并通知 coordinator
}
```

#### Task 2.3: 任务管理系统

**新建文件**:

- `pkg/zeroai/team/task-manager.go` — 任务 CRUD + 依赖链
- `pkg/zeroai/team/coordinator.go` — Coordinator (类似 ClawTeam leader)

**数据结构**:

```go
type TeamTask struct {
    ID          string    `json:"id"`
    TeamID      string    `json:"teamId"`
    Title       string    `json:"title"`
    Description string    `json:"description"`
    Assignee    string    `json:"assignee"`     // AgentRole ID
    Status      string    `json:"status"`       // pending, in_progress, completed, blocked
    DependsOn   []string  `json:"dependsOn"`    // 依赖的任务ID
    CreatedAt   int64     `json:"createdAt"`
    CompletedAt int64     `json:"completedAt"`
}
```

#### Task 2.4: 监督器 (Supervisor)

**新建文件**:

- `pkg/zeroai/team/supervisor.go` — 心跳检测 + 自动恢复

**逻辑**:

```go
type Supervisor struct {
    heartbeatInterval time.Duration  // 默认 30s
    staleThreshold    time.Duration  // 默认 5min
    maxRetries        int            // 默认 3
}

func (s *Supervisor) Start() {
    ticker := time.NewTicker(s.heartbeatInterval)
    for range ticker.C {
        s.checkAllSessions()
    }
}

func (s *Supervisor) checkAllSessions() {
    // 对每个活跃会话:
    // 1. 检查最后活动时间
    // 2. 如果超过 staleThreshold → 读取终端输出分析
    // 3. 如果是假死 → 发送唤醒消息
    // 4. 如果重试超限 → 标记为 failed,通知 coordinator
}
```

### Phase 3: 前端 UI 完善

#### Task 3.1: 左侧 Agent 角色面板

**文件**: 新建 `frontend/app/zeroai/components/agent-role-panel.tsx`

- 显示角色列表 (名称, 状态: 运行中/空闲/已停止)
- 点击角色 → 激活对应终端
- 新建角色按钮 → 打开创建对话框
- 角色配置: 名称, 描述, 提示词编辑, 技能选择, CLI 绑定

#### Task 3.2: 终端集成

**文件**: 修改 `frontend/app/zeroai/aipanel.tsx`

- 右侧区域嵌入 terminal block (使用 Wave Terminal 已有的 terminal 组件)
- 角色与终端的绑定逻辑
- 聚焦/切换终端的功能

#### Task 3.3: AI 智能创建角色工具

**文件**: 新建 RPC endpoint `ZeroAiCreateAgentRole`

- 暴露给 AI 使用的工具接口
- AI 可以通过 RPC 创建角色,配置提示词,绑定 CLI

### Phase 4: 配置与开关

#### Task 4.1: 配置项

**文件**: `schema/zeroai.json`, `pkg/wconfig/settingsconfig.go`

添加配置:

```json
{
  "zeroai.enabled": { "type": "boolean", "default": false },
  "zeroai.replaceWaveAI": { "type": "boolean", "default": false },
  "zeroai.defaultAgent": { "type": "string", "default": "claude" },
  "zeroai.maxSessionDuration": { "type": "integer", "default": 1800 },
  "zeroai.heartbeatInterval": { "type": "integer", "default": 30 },
  "zeroai.staleThreshold": { "type": "integer", "default": 300 }
}
```

#### Task 4.2: WaveAI 替换开关

**文件**: `frontend/app/workspace/workspace.tsx`

- 当 `zeroai.replaceWaveAI = true` 时,workspace 显示 zeroai 面板而非 waveai
- 保持两个系统完全独立,通过配置切换

---

## 四、风险与依赖

### 风险

1. **ACP 协议兼容性**: 不同 Agent CLI 的 ACP 实现可能有差异,需要逐个测试适配
2. **终端输出分析**: 判断"假死"需要解析终端输出,Agent CLI 的输出格式不统一
3. **资源管理**: 多个 Agent 进程同时运行可能占用大量系统资源

### 依赖

1. Wave Terminal 的 `wsh terminal` API (需要确认如何程序化创建终端 block)
2. 终端输出读取 API (需要确认如何从后端读取终端最后输出)
3. 现有 `pkg/zeroai/` 模块的稳定性

---

## 五、实施顺序建议

```
Phase 0 (Bug 修复) → Phase 1 (Agent CLI) → Phase 2 (协同系统) → Phase 3 (UI) → Phase 4 (配置)
     ↓                      ↓                      ↓                    ↓                ↓
  1-2天                  2-3天                  5-7天                3-4天            1-2天
```

**关键路径**: Phase 0 是所有后续工作的基础,必须优先完成。Phase 2 是最复杂的部分,建议拆分为多个子任务并行开发。
