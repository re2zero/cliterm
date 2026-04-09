# ZeroAI 重构设计文档 (v3 - 可行方案)

## 关键发现

经过深入分析，发现 WaveAI 和 ZeroAI 的架构存在根本差异：

| 方面         | WaveAI                        | ZeroAI                  |
| ------------ | ----------------------------- | ----------------------- |
| **后端**     | AI SDK (OpenAI 兼容)          | ACP 协议 (Agent CLI)    |
| **消息格式** | `UIMessage` / `UIMessagePart` | 自定义 `ZeroAiMessage`  |
| **流式处理** | `@ai-sdk/react` useChat       | 自定义 `for await...of` |

**结论**: 无法直接复用 `@ai-sdk/react`，因为后端协议不兼容。

---

## 可行方案

采用 **"UI 模式复用"** 方案：

- **保持 ACP 后端协议不变** (ZeroAI 的核心价值)
- **复用 WaveAI 的前端 UI 模式** (滚动、消息显示、状态管理)
- **改进现有的流式处理** (修复已识别的 bug)

---

## 重构任务清单

### 1. 改进滚动处理 (参考 WaveAI)

**文件**: `frontend/app/zeroai/components/chat-area.tsx`

改动:

```typescript
// 主要改动:
const prevMessagesLen = React.useRef(0);

// 依赖 messages.length 而非 totalContentLen
React.useEffect(() => {
  if (autoScroll && scrollRef.current) {
    if (messages.length !== prevMessagesLen.current) {
      requestAnimationFrame(() => {
        scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
      });
      prevMessagesLen.current = messages.length;
    }
  }
}, [messages.length, autoScroll]);
```

### 2. 改进流式消息处理

**文件**: `frontend/app/zeroai/models/message-model.ts`

改动:

```typescript
// 去掉 _seenLen 去重逻辑,直接追加
export function appendStreamChunk(sessionId: string, chunk: ZeroAiMessage): void {
  globalStore.set(streamingMessageAtom, (prev) => {
    const current = prev[sessionId];
    if (!current) return prev;

    const newContent = chunk.content || "";
    if (!newContent) return prev;

    return {
      ...prev,
      [sessionId]: {
        ...current,
        content: current.content + newContent,
      },
    };
  });
}
```

### 3. 改进错误处理和取消

**文件**: `frontend/app/zeroai/aipanel.tsx`

改动:

```typescript
// 区分正常完成、异常中断、用户取消
let streamFinished = false;

try {
  for await (const event of stream) {
    if (abortController.signal.aborted) break;
    // 处理事件...
    if (event.message?.eventType === "end_turn") {
      streamFinished = true;
      break;
    }
  }
} catch (error) {
  console.error("[ZeroAI] Stream error:", error);
  streamFinished = false;
} finally {
  cancelRef.current = null;

  // 根据实际情况处理
  if (abortController.signal.aborted) {
    dispatchMessageAction({ type: "cancelStream", sessionId });
  } else if (!streamFinished) {
    // 异常中断 - 检查是否有内容
    const streaming = getStreamingMessage(sessionId);
    if (streaming && streaming.content) {
      dispatchMessageAction({ type: "finalizeStream", sessionId });
    } else {
      dispatchMessageAction({ type: "cancelStream", sessionId });
    }
  } else {
    dispatchMessageAction({ type: "finalizeStream", sessionId });
  }

  globalStore.set(isStreamingAtom, false);
  setThinking(false);
}
```

### 4. 改进取消操作

**文件**: `frontend/app/zeroai/aipanel.tsx`

```typescript
const handleStopStreaming = async () => {
  // 1. 先设置状态 (立即反馈)
  globalStore.set(isStreamingAtom, false);
  setThinking(false);

  // 2. 中止前端循环
  if (cancelRef.current) {
    cancelRef.current.abort();
    cancelRef.current = null;
  }

  // 3. 调用后端取消
  if (sessionId && clientRef.current) {
    try {
      await clientRef.current.cancelStream(sessionId);
    } catch (error) {
      console.error("Failed to cancel stream:", error);
    }
  }

  // 4. 清空流式消息
  dispatchMessageAction({ type: "cancelStream", sessionId });
};
```

### 5. 改进消息显示动画

**文件**: `frontend/app/zeroai/components/chat-area.tsx`

```scss
// 为新消息添加出现动画
.chat-msg {
  animation: chatFadeIn 0.2s ease;
}

@keyframes chatFadeIn {
  from {
    opacity: 0;
    transform: translateY(4px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
```

### 6. 修复 UI 布局问题

**文件**: `frontend/app/zeroai/index.scss`

```scss
.chat-area-wrapper {
  display: flex;
  flex-direction: column;
  flex: 1;
  overflow: hidden;
  min-height: 0; // 关键: 允许 flex 子元素收缩
}

.chat-area {
  flex: 1;
  overflow-y: auto;
  min-height: 0; // 关键: 允许滚动
}
```

---

## 实施顺序

1. **先修复滚动问题** - 改动最小,最容易验证
2. **修复流式消息处理** - 去掉有问题的去重逻辑
3. **修复错误处理** - 区分三种完成状态
4. **修复取消操作** - 确保立即反馈
5. **修复 UI 布局** - 确保消息不被遮挡
6. **全面测试** - 验证所有场景

---

## 风险

1. 需要充分测试各种边界情况
2. ACP 协议本身可能还有其他问题
3. 可能需要多轮迭代

---

## 替代方案: 双面板

如果 ZeroAI 修复后仍不稳定，可以考虑让 ZeroAI 面板使用 WaveAI 后端：

- ZeroAI Panel UI (新改进的滚动/消息显示)
- WaveAI 后端 (通过切换 backend 配置)

但这需要用户有 WaveAI 的 API key，失去了 ZeroAI 的本地 Agent CLI 优势。
