# ZeroAI Panel — 完整调查分析与实施计划 (v2)

## 一、当前状态调查总结

### 1.1 代码架构全景

ZeroAI 已经有一套相对完整的独立实现,分布在以下层次:

| 层次              | 路径                                                 | 状态                                                                    |
| ----------------- | ---------------------------------------------------- | ----------------------------------------------------------------------- |
| **前端面板**      | `frontend/app/zeroai/aipanel.tsx`                    | ✅ 已有基础UI                                                           |
| **前端组件**      | `frontend/app/zeroai/components/`                    | ✅ Header, ChatArea, ChatInput, AgentList, StatusBar                    |
| **前端Store**     | `frontend/app/zeroai/models/`                        | ✅ provider-model, message-model, session-model, ui-model, agent-model  |
| **前端RPC客户端** | `frontend/app/zeroai/store/zeroai-client.ts`         | ✅ 完整RPC封装                                                          |
| **前端View**      | `frontend/app/view/zeroai/zeroai.tsx`                | ✅ Block注册                                                            |
| **后端RPC类型**   | `pkg/wshrpc/wshrpctypes.go`, `wshrpctypes_zeroai.go` | ✅ 类型定义完整                                                         |
| **后端RPC客户端** | `pkg/wshrpc/wshclient/wshclient.go`                  | ✅ 客户端stub                                                           |
| **后端RPC服务端** | `pkg/zeroai/rpc/wshserver-zeroai.go`                 | ✅ 完整RPC handler (含 streaming)                                       |
| **ACP协议层**     | `pkg/zeroai/protocol/`                               | ✅ 12个文件: acp-connection, acp-adapter, acp-config, acp-factory, etc. |
| **Agent层**       | `pkg/zeroai/agent/acp-agent.go`                      | ✅ ACP agent封装 (start/stop/create-session/send-message)               |
| **Service层**     | `pkg/zeroai/service/`                                | ✅ agent-service, message-service, session-service, provider-service    |
| **Store层**       | `pkg/zeroai/store/`                                  | ✅ session-store, message-store, team-store, db-migrations              |
| **Process层**     | `pkg/zeroai/process/`                                | ✅ process-manager, process-spawner                                     |
| **Team层**        | `pkg/zeroai/team/`                                   | ✅ coordinator, block-manager, message-router, prompt-builder           |
| **数据库迁移**    | `db/migrations-zeroai/`                              | ✅ init + team tables                                                   |
| **配置Schema**    | `schema/zeroai.json`                                 | ✅ 已有                                                                 |

**关键发现**: ZeroAI 已经是一个独立模块,与 WaveAI 几乎没有代码交叉。`pkg/zeroai/` 是独立目录,前端在 `frontend/app/zeroai/` 独立目录。隔离性已经很好。

### 1.2 WaveAI vs ZeroAI 对比

| 特性         | WaveAI                                           | ZeroAI                                        |
| ------------ | ------------------------------------------------ | --------------------------------------------- |
| **架构**     | `@ai-sdk/react` 的 `useChat()` hook, 直接LLM API | 自定义 ACP 协议 + Agent CLI 进程管理          |
| **流式**     | useChat() 内置 stop()                            | AbortController + cancelStream + CancelPrompt |
| **状态**     | SDK 自动管理                                     | 手动 globalStore.set(isStreamingAtom)         |
| **集成方式** | 内置 Wave 侧边栏面板                             | 独立 Block View, 可配置替换 WaveAI            |

### 1.3 "Stuck Typing" Bug 根因分析

**问题**: 输入消息后AI一直处于"typing"状态,停止按钮无响应。

**根因定位 — 三层问题**:

#### Bug #1: 后端 `ZeroAiSendStreamMessageCommand` 的 goroutine 在 context cancel 后不优雅退出

当前代码（已修复为健壮版本）:

```go
// 当前 wshserver-zeroai.go 中已有 select 保护
for {
    select {
    case <-ctx.Done():
        return  // ✅ 已有
    case event, ok := <-eventCh:
        if !ok {
            return  // ✅ 已有
        }
        // ... 发送 event 到 rtn ...
        if event.Type == agent.EventTypeEndTurn {
            return  // ✅ 已有
        }
    }
}
```

**但仍有隐患**: `eventCh` 如果因为 ACP connection 异常关闭(非 cancel),可能导致 panic。需要确认 `eventCh` 在所有路径下都被正确关闭。

#### Bug #2: 前端 `handleStopStreaming` 缺少 `cancelRef.current = null`

已在当前代码中修复:

```typescript
const handleStopStreaming = async () => {
  if (cancelRef.current) {
    cancelRef.current.abort();
    cancelRef.current = null; // ✅ 已补充
  }
  // ...
};
```

#### Bug #3: 后端 `CancelPrompt` 后 done watcher goroutine 可能卡在 sendEvent

`acp-agent.go` 的 done watcher goroutine 在 `CancelPrompt` 后仍会发送 `end_turn` 事件,但此时 `eventCh` 可能已被主流程关闭,导致 `sendEvent` 的 `ch <- event` 阻塞(有100ms超时保护,但非零风险)。

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
7. Agent → Client: session/update notifications (多次 text_chunk, tool_call, etc.)
8. Agent → Client: {"sessionUpdate":"end_turn"}
9. Agent → Client: {"jsonrpc":"2.0","id":3,"result":{...}}  (RPC响应)

取消流程:
- 通过关闭进程或 context cancel 实现,没有专用 cancel RPC
```

**当前 Go 实现与 AionUi TypeScript 实现的协议层面完全一致**。差异在于:

- AionUi 在前端直接通过 stdio 连接 agent
- ZeroAI 在后端管理 agent 进程,前端通过 WSH RPC 间接通信

### 1.5 已有后端配置 (acp-config.go)

已有完整的 Agent CLI 配置:

| Backend  | CLI Command | ACP Args            | Transport |
| -------- | ----------- | ------------------- | --------- |
| claude   | `claude`    | (无, 走 npx bridge) | ACP       |
| gemini   | `gemini`    | `acp`               | ACP       |
| qwen     | `qwen`      | `--acp`             | ACP       |
| codex    | `codex`     | `mcp-server`        | ACP       |
| opencode | `opencode`  | `acp`               | ACP       |
| custom   | 自定义      | 自定义              | ACP       |

**关键**: 所有主流 Agent CLI 已有配置,Phase 1 "Agent CLI 对接完善" 的工作量比预期小。

### 1.6 ClawTeam 协作模式 vs 现有 ZeroAI Team 层

ClawTeam (Python) 的核心:

```
Leader Agent (Claude Code / Codex等)
    ├── 通过 `clawteam spawn` 创建 Worker Agents (tmux + git worktree)
    ├── 任务管理: clawteam task create/update/list (依赖链)
    ├── 消息系统: clawteam inbox send/receive (ZeroMQ P2P)
    └── 监控: clawteam board show/attach/serve
```

**ZeroAI 已有 Go 替代模块** (`pkg/zeroai/team/`):

| ClawTeam 功能 | ZeroAI Go 替代                                   | 状态    |
| ------------- | ------------------------------------------------ | ------- |
| tmux 会话管理 | `block-manager.go` (wcore.CreateBlock)           | ✅ 已有 |
| 任务状态机    | `coordinator.go` (pending→in_progress→completed) | ✅ 已有 |
| 消息路由      | `message-router.go`                              | ✅ 已有 |
| Prompt构建    | `prompt-builder.go`                              | ✅ 已有 |
| 持久化        | `team-store.go` + `team-store-memory.go`         | ✅ 已有 |
| 心跳/超时检测 | ❌ 缺失 (仅有 LastActive 字段,无定时扫描)        | ⚠️ 需要 |
| 终端输出读取  | ❌ 缺失 (只能 SendInput,不能 ReadOutput)         | ⚠️ 需要 |

### 1.7 Ralphy 架构分析

Ralphy 是 TypeScript/Node.js CLI,核心模块:

```
engines/ (引擎抽象)
    ├── base.ts          → BaseAIEngine 抽象类
    ├── claude.ts        → Claude 引擎 (stream-json 格式)
    ├── opencode.ts      → OpenCode 引擎
    ├── codex.ts         → Codex 引擎
    ├── qwen.ts          → Qwen 引擎
    ├── gemini.ts        → Gemini 引擎
    ├── copilot.ts       → Copilot 引擎
    ├── cursor.ts        → Cursor 引擎
    └── droid.ts         → Factory Droid 引擎

execution/ (执行引擎)
    ├── sequential.ts    → 顺序执行任务循环
    ├── parallel.ts      → 并行执行 (worktree/sandbox 隔离)
    ├── retry.ts         → 指数退避重试 (jitter + 错误分类)
    └── prompt.ts        → Prompt 构建器 (project context + rules + boundaries)
```

**能否直接套用 Ralphy CLI?**

- ❌ **不能直接复用代码** — TypeScript vs Go,完全不同的语言
- ❌ **不能直接复用架构** — Ralphy 是 CLI 子进程模式,ZeroAI 是 ACP 协议模式,集成范式不同
- ✅ **可移植设计模式**:
  - Engine 抽象 → 映射到 `pkg/zeroai/protocol/acp-config.go` + `acp-factory.go` (已有,但可借鉴 Ralphy 的引擎参数细节)
  - 错误分类 (retryable vs fatal) → **需要移植到 Go** (ralphy 的 12 条 retryable patterns + 10 条 fatal patterns)
  - 指数退避 + jitter → **需要移植到 Go** (ralphy 的 `calculateBackoffDelay` 函数)
  - Step 检测 (`detectStepFromOutput`) → **可借鉴** 用于"假死检测"
  - Prompt 构建 → 已有 `prompt-builder.go`,可借鉴 ralphy 的 project context + rules 模式

### 1.8 Ralphy 核心可移植清单

| Ralphy 模块                | 移植价值   | 移植方式                             | 目标文件                            |
| -------------------------- | ---------- | ------------------------------------ | ----------------------------------- |
| `retry.ts` (错误分类)      | ⭐⭐⭐⭐⭐ | 移植为 Go 函数                       | `pkg/zeroai/team/errors.go`         |
| `retry.ts` (退避逻辑)      | ⭐⭐⭐⭐⭐ | 移植为 Go 函数                       | `pkg/zeroai/team/retry.go`          |
| `base.ts` (引擎参数)       | ⭐⭐⭐⭐   | 补充到 acp-config.go                 | `pkg/zeroai/protocol/acp-config.go` |
| `detectStepFromOutput`     | ⭐⭐⭐⭐   | 移植为 Go 函数                       | `pkg/zeroai/team/step-detector.go`  |
| `prompt.ts` (prompt构建)   | ⭐⭐⭐     | 合并到 prompt-builder.go             | `pkg/zeroai/team/prompt-builder.go` |
| `sequential.ts` (执行循环) | ⭐⭐⭐     | 合并到 coordinator.go                | `pkg/zeroai/team/coordinator.go`    |
| `parallel.ts` (隔离执行)   | ⭐⭐       | 部分借鉴 (隔离由 block-manager 处理) | N/A                                 |

---

## 二、功能需求可行性分析

### 需求1: 对接各种 Agent CLI (claude code, opencode, codex, gemini, qwen)

**可行性**: ✅ 完全可行

**方案**:

- 已有 ACP 协议层 + 完整后端配置
- Ralphy 的价值: 补充各引擎的具体 CLI 参数细节 (如 `--dangerously-skip-permissions`, `--yolo`, `--output-format stream-json`)
- 这些参数已部分存在于 `acp-config.go`,但 Ralphy 有更完整的 per-engine 配置

**需要补充**:

- 从 Ralphy 移植每个引擎的完整 CLI 参数 (见 1.8 表格)
- 非 ACP 模式 Agent 的 fallback (通过终端输入/输出解析) — 这是 Ralphy 的核心能力

### 需求2: 自定义 LLM 提供商接入 (OpenAI 兼容)

**可行性**: ✅ 完全可行,已有基础

**方案**:

- 复用 `provider-service.go` 的自定义 Provider 系统
- 已有的 `ZeroAiSaveProviderCommand`, `ZeroAiTestProviderCommand` 支持

### 需求3: 多 Agent 协同

**可行性**: ✅ 可行,现有基础比预期好

**关键调整**: 不是"新建"模块,而是**完善现有模块** + **移植 Ralphy 核心逻辑**:

```
ZeroAI Coordinator (已有 Go 基础)
    │
    ├── [已有] AgentRole → team-types.go (TeamMember + MemberRole)
    ├── [已有] TerminalSession → block-manager.go (wcore.CreateBlock)
    ├── [已有] TaskManager → coordinator.go (Task CRUD + 状态机)
    ├── [已有] MessageRouter → message-router.go
    ├── [缺失] Supervisor (心跳检测 + 自动恢复) ← 需要新增
    ├── [缺失] 终端输出读取 ← 需要新增 (WPS 事件订阅)
    ├── [移植] Error 分类 → 从 Ralphy retry.ts 移植到 Go
    ├── [移植] Retry 逻辑 → 从 Ralphy retry.ts 移植到 Go
    └── [移植] Step 检测 → 从 Ralphy base.ts 移植到 Go (假死检测)
```

**前端 UI 架构选择**:

由于 Wave Terminal 的 terminal block 通过 `BlockRegistry` + `ViewModel` 注册,不能简单"嵌入"到 ZeroAI panel。推荐:

**方案B (推荐)**: ZeroAI panel 左侧管理角色,点击时通过 `wcore.CreateBlock()` 在 workspace 创建/聚焦 terminal block。ZeroAI 侧边栏通过 `MessageRouter` + `BlockManager` 与终端通信。

**方案A (嵌入)**: 在 ZeroAI panel 内嵌入终端组件 — 需要大幅改造 terminal 组件,风险高。

---

## 三、实施计划

### Phase 0: 修复 "Stuck Typing" Bug (最高优先级)

**目标**: 确保流式输出可靠,停止按钮响应正常

#### Task 0.1: 增强后端 eventCh 生命周期保护

**文件**: `pkg/zeroai/rpc/wshserver-zeroai.go`

当前 streaming goroutine 已有基本的 context cancel 保护,但需要确保:

1. `eventCh` 在所有退出路径下都正确关闭
2. `rtn` channel 不会因 `eventCh` 关闭而泄漏

```go
func (zs *WshRpcZeroaiServer) ZeroAiSendStreamMessageCommand(ctx context.Context, req wshrpc.CommandZeroAiSendMessageData) chan wshrpc.RespOrErrorUnion[wshrpc.ZeroAiStreamMessageEvent] {
    rtn := make(chan wshrpc.RespOrErrorUnion[wshrpc.ZeroAiStreamMessageEvent])

    go func() {
        defer close(rtn)  // 始终确保关闭

        streamCtx, streamCancel := context.WithCancel(ctx)
        defer streamCancel()

        backend := zs.getBackendForSession(streamCtx, req.SessionID)
        ag, err := zs.agentService.GetAgent(streamCtx, agent.AgentConfig{Backend: backend})
        if err != nil {
            sendError(rtn, err)
            return
        }

        input := agent.SendMessageInput{Content: req.Content}
        eventCh, err := ag.SendMessage(streamCtx, req.SessionID, input)
        if err != nil {
            sendError(rtn, err)
            return
        }

        for {
            select {
            case <-streamCtx.Done():
                return
            case event, ok := <-eventCh:
                if !ok {
                    return // eventCh 关闭,优雅退出
                }
                // ... 发送 event 到 rtn ...
                if event.Type == agent.EventTypeEndTurn {
                    return
                }
            }
        }
    }()

    return rtn
}
```

#### Task 0.2: 后端 CancelPrompt 增强 — 确保 done watcher 不卡

**文件**: `pkg/zeroai/agent/acp-agent.go`

当前 `CancelPrompt` 仅调用 `cancelCtx()`,但 done watcher goroutine 仍会执行 `sendEvent`。需要确保即使 `eventCh` 已关闭,`sendEvent` 也不会 panic (已有 recover 保护,但需增强)。

**修复**: 在 `sendEvent` 的 recover 基础上,增加 eventCh 存活检查:

```go
func (a *AcpAgent) sendEvent(sessionID string, event AgentEvent) bool {
    a.mu.Lock()
    ch, exists := a.eventChs[sessionID]
    a.mu.Unlock()

    if !exists {
        return false // eventCh 已不存在,跳过
    }

    defer func() {
        recover() // 防止 channel closed panic
    }()

    select {
    case ch <- event:
        return true
    case <-time.After(100 * time.Millisecond):
        return false // channel 满了,放弃
    }
}
```

**结论**: 当前代码已有 recover 保护 + 100ms 超时,风险可控。Task 0.2 的优先级低于 Task 0.1。

### Phase 1: Agent CLI 对接完善

**目标**: 利用 Ralphy 的引擎参数知识,完善已有 ACP 配置

#### Task 1.1: 从 Ralphy 移植引擎参数细节

**文件**: `pkg/zeroai/protocol/acp-config.go`

当前配置已有基本 CLI 参数,但缺少 Ralphy 中的引擎细节。补充:

```go
// 补充各引擎的完整配置 (参考 Ralphy engines/)
var backendConfigs = map[AcpBackend]AcpBackendConfig{
    AcpBackendClaude: {
        // 已有: CliCommand, DefaultCliPath, AcpArgs, Transport, Env
        // 补充: 从 Ralphy claude.ts 移植的权限参数
        PermissionsArg:  "--dangerously-skip-permissions",
        OutputFormatArg: "--output-format",
        OutputFormat:    "stream-json",
        PromptArg:       "-p",
        UseStdinOnWin:   true,
    },
    // ... opencode, codex, gemini, qwen 同理
}
```

#### Task 1.2: 从 Ralphy 移植错误分类系统

**新建文件**: `pkg/zeroai/team/errors.go`

直接移植 Ralphy `retry.ts` 中的错误分类模式:

```go
// RetryableErrorPatterns — from Ralphy retry.ts
var retryablePatterns = []string{
    `(?i)rate limit`, `(?i)rate_limit`, `(?i)hit your limit`,
    `(?i)quota`, `(?i)too many requests`, `429`,
    `(?i)timeout`, `(?i)network`, `(?i)connection`,
    `ECONNRESET`, `ETIMEDOUT`, `ENOTFOUND`, `(?i)overloaded`,
}

// FatalErrorPatterns — from Ralphy retry.ts
var fatalPatterns = []string{
    `(?i)not authenticated`, `(?i)no authentication`,
    `(?i)authentication failed`, `(?i)invalid.*token`,
    `(?i)invalid.*api.?key`, `(?i)unauthorized`, `\b401\b`, `\b403\b`,
    `(?i)command not found`, `(?i)not installed`, `(?i)is not recognized`,
}

func IsRetryableError(err string) bool { /* ... */ }
func IsFatalError(err string) bool     { /* ... */ }
```

#### Task 1.3: 从 Ralphy 移植指数退避 + Jitter

**新建文件**: `pkg/zeroai/team/retry.go`

```go
// CalculateBackoffDelay — from Ralphy retry.ts
func CalculateBackoffDelay(attempt int, baseDelayMs, maxDelayMs int, useJitter bool) int {
    delay := baseDelayMs * int(math.Pow(2, float64(attempt-1)))
    if delay > maxDelayMs {
        delay = maxDelayMs
    }
    if useJitter {
        jitter := int(float64(delay) * 0.25 * rand.Float64())
        delay += jitter
    }
    return delay
}
```

#### Task 1.4: 自定义 LLM Provider UI 完善

**文件**: `frontend/app/zeroai/components/provider-settings.tsx`

已有 `ProviderSettings` 组件,确保:

- OpenAI 兼容 API endpoint 配置
- API key, model list 配置
- 测试连接功能 (`testProvider`)

### Phase 2: 多 Agent 协同系统 (核心)

**目标**: 完善现有 team 层,移植 Ralphy 执行引擎,补齐 Supervisor

#### Task 2.1: 从 Ralphy 移植 Step 检测 (假死检测核心)

**新建文件**: `pkg/zeroai/team/step-detector.go`

移植 Ralphy `base.ts` 中的 `detectStepFromOutput` 逻辑,用于:

1. 分析终端/ACP 输出,判断 Agent 当前在做什么
2. 检测"假死"状态 — 长时间没有 step 变化 = 可能假死

```go
type AgentStep string

const (
    StepReadingCode    AgentStep = "Reading code"
    StepImplementing   AgentStep = "Implementing"
    StepTesting        AgentStep = "Testing"
    StepLinting        AgentStep = "Linting"
    StepCommitting     AgentStep = "Committing"
    StepIdle           AgentStep = "Idle"
)

// DetectStepFromOutput — from Ralphy base.ts
func DetectStepFromOutput(line string) AgentStep {
    // 解析 JSON 输出,判断 tool name / command / file path
    // 返回对应的 step
}

// IsStuck — 判断 Agent 是否假死
func IsStuck(lastStep AgentStep, lastStepTime time.Time, threshold time.Duration) bool {
    return time.Since(lastStepTime) > threshold
}
```

#### Task 2.2: 完善 BlockManager — 终端输出读取

**文件**: `pkg/zeroai/team/block-manager.go` (修改)

当前只能 `SendToBlock()` (写入),需要增加 `ReadFromBlock()` (读取):

```go
// ReadFromBlock — 读取终端最后 N 行输出 (用于假死检测)
func (bm *BlockManager) ReadFromBlock(blockID string, lines int) (string, error) {
    // 方案: 通过 WPS (Wave PubSub) 事件系统订阅终端输出
    // 或者通过 blockcontroller 的 scrollback 接口
}
```

**技术风险**: WSH RPC 目前没有直接的"读取终端输出"API。需要通过 WPS 事件订阅 (`wps/subscribe`) 或 `blockcontroller` 的 scrollback buffer 实现。这是**整个计划的关键未验证依赖**。

#### Task 2.3: Agent 角色系统 (已有基础,扩展)

**文件**: 扩展 `pkg/zeroai/team/team-types.go`

在现有 `TeamMember` 基础上扩展角色属性:

```go
type AgentRole struct {
    TeamMember             // 嵌入已有的 TeamMember
    SystemPrompt string    // AGENT.md 内容
    MemoryPrompt string    // MEMORY.md 内容
    SoulPrompt   string    // SOUL.md 内容
    Skills       []string  // 已选技能
    MCPServers   []string  // 已选 MCP
    BoundCLI     string    // 绑定的 CLI
    BlockID      string    // 关联的 terminal block
}
```

#### Task 2.4: 新增 Supervisor (心跳 + 自动恢复)

**新建文件**: `pkg/zeroai/team/supervisor.go`

在现有 `coordinator.go` + `team-types.go` 基础上增加定时扫描:

```go
type Supervisor struct {
    coordinator     *Coordinator
    blockManager    *BlockManager
    heartbeatTicker *time.Ticker
    staleThreshold  time.Duration  // 默认 5min
    maxRetries      int            // 默认 3
}

func (s *Supervisor) Start() {
    go func() {
        for range s.heartbeatTicker.C {
            s.checkAllMembers()
        }
    }()
}

func (s *Supervisor) checkAllMembers() {
    // 对每个 team member:
    // 1. 检查 LastActive 时间
    // 2. 如果超过 staleThreshold → 读取终端输出分析
    // 3. 使用 StepDetector 判断当前状态
    // 4. 如果是假死 → 发送唤醒消息
    // 5. 使用 Retry 逻辑 (指数退避) 重试
    // 6. 如果重试超限 → 标记为 failed,通知 coordinator
}
```

#### Task 2.5: 从 Ralphy 移植执行循环到 Coordinator

**文件**: `pkg/zeroai/team/coordinator.go` (修改)

在现有 `Coordinator` 中增加任务执行循环:

```go
// StartWorkerLoop — 类似 Ralphy runSequential
func (c *Coordinator) StartWorkerLoop(teamID string) error {
    // 1. 获取下一个 pending task
    // 2. 获取 assigned agent
    // 3. 通过 BlockManager 发送 prompt 到终端
    // 4. 监控执行状态 (通过 Supervisor)
    // 5. 任务完成/失败后更新状态
    // 6. 检查依赖,自动解锁 blocked tasks
}

// StartParallelLoop — 类似 Ralphy runParallel
func (c *Coordinator) StartParallelLoop(teamID string, maxAgents int) error {
    // 1. 获取 parallel_group 相同的 tasks
    // 2. 为每个 task 创建独立 block (BlockManager)
    // 3. 并行启动
    // 4. 监控所有 agent 状态
    // 5. 完成后清理 blocks
}
```

### Phase 3: 前端 UI

#### Task 3.1: 左侧 Agent 角色面板

**新建文件**: `frontend/app/zeroai/components/agent-role-panel.tsx`

- 显示角色列表 (名称, 状态: 运行中/空闲/已停止)
- 新建角色按钮 → 打开创建对话框
- 角色配置: 名称, 描述, 提示词编辑, 技能选择, CLI 绑定

#### Task 3.2: 终端集成 — 方案B (Workspace Block)

**文件**: 修改 `frontend/app/zeroai/aipanel.tsx`

采用方案B: 点击角色时,通过 RPC 调用 `BlockManager.SpawnAgentBlock()` 在 workspace 创建/聚焦 terminal block:

```typescript
const handleActivateRole = async (roleId: string) => {
  // 1. 检查角色是否已有活跃 block
  // 2. 如果没有 → 调用 RPC 创建新 block
  // 3. 如果有 → 聚焦已有 block
  // 4. ZeroAI 侧边栏保持角色管理状态
};
```

#### Task 3.3: AI 智能创建角色工具

**新建 RPC endpoint**: `ZeroAiCreateAgentRole`

- 暴露给 AI 使用的工具接口
- AI 可以通过 RPC 创建角色,配置提示词,绑定 CLI

### Phase 4: 配置与开关

#### Task 4.1: 配置项

**文件**: `schema/zeroai.json`, `pkg/wconfig/settingsconfig.go`

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

### 关键未验证依赖 (RED FLAGS)

1. **终端输出读取 API** 🔴
   - `blockcontroller.SendInput()` 已有,但 `ReadOutput()` 没有
   - 可能方案: WPS 事件订阅终端输出,或 blockcontroller scrollback buffer
   - 需要在 Phase 0 后立即验证

2. **ACP 协议兼容性** 🟡
   - 不同 Agent CLI 的 ACP 实现可能有差异
   - claude 使用 npx bridge,非原生 ACP,需要额外配置
   - 需要逐个测试适配

3. **资源管理** 🟡
   - 多个 Agent 进程同时运行可能占用大量系统资源
   - 需要 session 超时自动清理

### 风险缓解

1. **终端输出读取**: Phase 0 后立即验证 WPS 事件系统是否能订阅终端输出。如果不行,退化为"仅通过 ACP 回调检测状态"
2. **ACP 兼容性**: 先完成 claude + opencode 两个最常用的,其他逐步扩展
3. **资源管理**: Supervisor 的 staleThreshold 自动清理超时 session

---

## 五、实施顺序建议

```
Phase 0 (Bug 修复)     → 1-2天   [最高优先级]
    ↓
Phase 1 (Agent CLI)    → 2-3天   [移植 Ralphy 错误分类 + 退避逻辑]
    ↓
Phase 2.1 (Step 检测)  → 1-2天   [移植 Ralphy detectStepFromOutput]
    ↓
Phase 2.2 (终端读取)   → 2-3天   [关键未验证依赖,需要先行验证]
    ↓
Phase 2.3 (角色系统)   → 1-2天   [扩展现有 team-types.go]
    ↓
Phase 2.4 (Supervisor) → 2-3天   [心跳 + 自动恢复]
    ↓
Phase 2.5 (执行循环)   → 2-3天   [移植 Ralphy sequential/parallel]
    ↓
Phase 3 (UI)           → 3-4天   [角色面板 + 终端集成方案B]
    ↓
Phase 4 (配置)         → 1-2天   [配置项 + 替换开关]
```

**总预估**: 15-24 天

**关键路径**:

1. Phase 0 是所有后续工作的基础
2. Phase 2.2 (终端输出读取) 是关键阻塞点 — 如果无法实现,Supervisor 的假死检测将退化为"仅时间阈值"
3. Ralphy 移植 (Phase 1 + 2.1 + 2.5) 可并行进行,因为各模块独立

## 六、Ralph 参考借鉴

`/home/zero/zero/ralphy` 的核心借鉴价值:

1. **7x24 自主管理**: Ralphy 的 retry 循环 + 指数退避 + jitter 是 7x24 运行的核心保障
2. **Engine 抽象**: 每个 Agent CLI 独立的参数/输出格式处理 — 映射到 ZeroAI 的 ACP 配置层
3. **错误分类**: retryable vs fatal 的分类决定了"假死"后是否自动恢复
4. **Prompt 构建**: project context + rules + boundaries 模式 — 可用于 Agent 角色的初始化 prompt

**不建议直接套用的部分**:

- Git worktree/sandbox 隔离 — Wave Terminal 已有 Block 隔离机制
- PR 创建/分支合并 — 不是 ZeroAI 的核心需求
- CLI 子进程模式 — ZeroAI 使用 ACP 协议,范式不同
