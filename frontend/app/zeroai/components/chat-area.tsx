// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { WaveStreamdown } from "@/app/element/streamdown";
import { makeIconClass } from "@/util/util";
import clsx from "clsx";
import * as React from "react";
import type { ZeroAiMessage, ZeroAiPermissionRequest } from "../types";
import { parseThinking } from "../util/thinking-parser";
import "./chat-area.scss";

const formatTimestamp = (timestamp: number): string => {
    const date = new Date(timestamp * 1000);
    const now = new Date();
    const diffMinutes = Math.floor((now.getTime() - date.getTime()) / 60000);

    if (diffMinutes < 1) return "Just now";
    if (diffMinutes < 60) return `${diffMinutes}m ago`;
    if (diffMinutes < 1440) return `${Math.floor(diffMinutes / 60)}h ago`;
    return date.toLocaleDateString([], { month: "short", day: "numeric" });
};

const UserMessage = React.memo(({ message }: { message: ZeroAiMessage }) => {
    const [copied, setCopied] = React.useState(false);

    const handleCopy = React.useCallback(() => {
        navigator.clipboard.writeText(message.content);
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
    }, [message.content]);

    return (
        <div className="chat-msg chat-msg-user">
            <div className="chat-msg-avatar">
                <i className="fa-solid fa-user" />
            </div>
            <div className="chat-msg-body">
                <div className="chat-msg-header">
                    <span className="chat-msg-role">You</span>
                    <span className="chat-msg-time">{formatTimestamp(message.createdAt)}</span>
                </div>
                <div className="chat-msg-text">{message.content}</div>
                <div className="chat-msg-actions">
                    <button onClick={handleCopy} title="Copy">
                        <i className={makeIconClass(copied ? "fa-solid fa-check" : "fa-regular fa-copy", false)} />
                    </button>
                </div>
            </div>
        </div>
    );
});
UserMessage.displayName = "UserMessage";

const AssistantMessage = React.memo(({ message }: { message: ZeroAiMessage }) => {
    const [copied, setCopied] = React.useState(false);
    const [thinkingOpen, setThinkingOpen] = React.useState(false);

    const handleCopy = React.useCallback(() => {
        navigator.clipboard.writeText(message.content);
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
    }, [message.content]);

    const { thinking, content } = parseThinking(message.content);

    return (
        <div className="chat-msg chat-msg-assistant">
            <div className="chat-msg-avatar">
                <i className="fa-solid fa-robot" />
            </div>
            <div className="chat-msg-body">
                <div className="chat-msg-header">
                    <span className="chat-msg-role">Assistant</span>
                    {thinking && (
                        <button className="chat-thinking-toggle" onClick={() => setThinkingOpen(!thinkingOpen)}>
                            <i className={makeIconClass("fa-solid fa-brain", false)} />
                            <span>Thinking</span>
                            <i
                                className={clsx(
                                    makeIconClass("fa-solid fa-chevron-down", false),
                                    "chat-thinking-chevron",
                                    { expanded: thinkingOpen }
                                )}
                            />
                        </button>
                    )}
                    <span className="chat-msg-time">{formatTimestamp(message.createdAt)}</span>
                </div>

                {thinkingOpen && thinking && (
                    <div className="chat-thinking-block">
                        <WaveStreamdown text={thinking} parseIncompleteMarkdown={false} />
                    </div>
                )}

                {content && (
                    <div className="chat-msg-markdown">
                        <WaveStreamdown text={content} parseIncompleteMarkdown={false} />
                    </div>
                )}

                <div className="chat-msg-actions">
                    <button onClick={handleCopy} title="Copy">
                        <i className={makeIconClass(copied ? "fa-solid fa-check" : "fa-regular fa-copy", false)} />
                    </button>
                </div>
            </div>
        </div>
    );
});
AssistantMessage.displayName = "AssistantMessage";

const ToolCallMessage = React.memo(({ message }: { message: ZeroAiMessage }) => {
    const [expanded, setExpanded] = React.useState(false);
    const toolData = message.metadata as Record<string, unknown> | undefined;
    const toolName = (toolData?.toolName as string) || (toolData?.name as string) || "tool";
    const toolInput = toolData?.input ? JSON.stringify(toolData.input, null, 2) : "";
    const toolOutput = toolData?.output ? JSON.stringify(toolData.output, null, 2) : "";
    const status = (toolData?.status as string) || "running";

    const statusIcon =
        status === "completed"
            ? "fa-solid fa-check-circle"
            : status === "failed"
              ? "fa-solid fa-times-circle"
              : "fa-solid fa-spinner fa-spin";

    return (
        <div className={clsx("chat-tool", `status-${status}`)}>
            <button className="chat-tool-header" onClick={() => setExpanded(!expanded)}>
                <i className={makeIconClass("fa-solid fa-wrench", false)} />
                <span className="chat-tool-name">{toolName}</span>
                <i className={makeIconClass(statusIcon, false)} />
                <i
                    className={clsx(makeIconClass("fa-solid fa-chevron-down", false), "chat-tool-chevron", {
                        expanded,
                    })}
                />
            </button>
            {expanded && (
                <div className="chat-tool-body">
                    {toolInput && (
                        <div className="chat-tool-section">
                            <span className="chat-tool-section-label">Input</span>
                            <pre className="chat-tool-code">{toolInput}</pre>
                        </div>
                    )}
                    {toolOutput && (
                        <div className="chat-tool-section">
                            <span className="chat-tool-section-label">Output</span>
                            <pre className="chat-tool-code">{toolOutput}</pre>
                        </div>
                    )}
                </div>
            )}
        </div>
    );
});
ToolCallMessage.displayName = "ToolCallMessage";

const PermissionMessage = React.memo(({ message }: { message: ZeroAiMessage }) => {
    const permData = message.metadata as Record<string, unknown> | undefined;
    const toolName = (permData?.toolName as string) || "unknown tool";
    const description = (permData?.description as string) || "";
    const options = (permData?.options as ZeroAiPermissionRequest["options"]) || [];
    const callId = (permData?.callId as string) || "";
    const sessionId = message.sessionId;

    const handleConfirm = React.useCallback(
        async (optionId: string, confirmAll: boolean) => {
            // TODO: wire to zeroAiClient.confirmPermission
            console.log("Confirm permission:", { sessionId, callId, optionId, confirmAll });
        },
        [sessionId, callId]
    );

    return (
        <div className="chat-permission">
            <div className="chat-permission-header">
                <i className={makeIconClass("fa-solid fa-shield-halved", false)} />
                <span>Permission Required</span>
            </div>
            <div className="chat-permission-body">
                <p>
                    <strong>{toolName}</strong>
                </p>
                {description && <p className="chat-permission-desc">{description}</p>}
            </div>
            {options.length > 0 && (
                <div className="chat-permission-actions">
                    {options.map((opt) => (
                        <button
                            key={opt.id}
                            className={clsx(
                                "chat-perm-btn",
                                opt.id === "allow" && "allow",
                                opt.id === "deny" && "deny"
                            )}
                            onClick={() => handleConfirm(opt.id, false)}
                        >
                            {opt.label}
                        </button>
                    ))}
                </div>
            )}
        </div>
    );
});
PermissionMessage.displayName = "PermissionMessage";

const PlanUpdateMessage = React.memo(({ message }: { message: ZeroAiMessage }) => {
    const [expanded, setExpanded] = React.useState(false);
    const planData = message.metadata as Record<string, unknown> | undefined;
    const planContent = message.content;

    return (
        <div className="chat-plan">
            <button className="chat-plan-header" onClick={() => setExpanded(!expanded)}>
                <i className={makeIconClass("fa-solid fa-list-check", false)} />
                <span>Plan</span>
                <i
                    className={clsx(makeIconClass("fa-solid fa-chevron-down", false), "chat-plan-chevron", {
                        expanded,
                    })}
                />
            </button>
            {expanded && (
                <div className="chat-plan-body">
                    <WaveStreamdown text={planContent} parseIncompleteMarkdown={false} />
                </div>
            )}
        </div>
    );
});
PlanUpdateMessage.displayName = "PlanUpdateMessage";

const ErrorMessage = React.memo(({ message }: { message: ZeroAiMessage }) => (
    <div className="chat-error">
        <i className={makeIconClass("fa-solid fa-circle-exclamation", false)} />
        <span>{message.content}</span>
    </div>
));
ErrorMessage.displayName = "ErrorMessage";

const MessageRenderer = React.memo(({ message }: { message: ZeroAiMessage }) => {
    const eventType = message.eventType || (message.metadata?.type as string | undefined);

    if (
        eventType === "tool_call" ||
        eventType === "tool_started" ||
        eventType === "tool_completed" ||
        eventType === "tool_failed"
    ) {
        return <ToolCallMessage message={message} />;
    }
    if (eventType === "permission" || eventType === "permission_request") {
        return <PermissionMessage message={message} />;
    }
    if (eventType === "plan_update" || eventType === "plan") {
        return <PlanUpdateMessage message={message} />;
    }
    if (eventType === "error") {
        return <ErrorMessage message={message} />;
    }

    if (message.role === "user") {
        return <UserMessage message={message} />;
    }
    return <AssistantMessage message={message} />;
});
MessageRenderer.displayName = "MessageRenderer";

const SUGGESTED_PROMPTS = [
    { icon: "fa-solid fa-code", text: "Explain this codebase", hint: "Analyze the current project" },
    { icon: "fa-solid fa-bug", text: "Find and fix bugs", hint: "Debug issues in the code" },
    { icon: "fa-solid fa-pen-ruler", text: "Refactor this module", hint: "Improve code structure" },
    { icon: "fa-solid fa-file-lines", text: "Write documentation", hint: "Generate docs for the code" },
];

export const ChatArea = React.memo(({ messages, className }: { messages: ZeroAiMessage[]; className?: string }) => {
    const scrollRef = React.useRef<HTMLDivElement>(null);
    const [autoScroll, setAutoScroll] = React.useState(true);
    const prevLen = React.useRef(messages.length);

    const handleScroll = React.useCallback(() => {
        if (!scrollRef.current) return;
        const { scrollTop, scrollHeight, clientHeight } = scrollRef.current;
        setAutoScroll(scrollHeight - scrollTop - clientHeight < 50);
    }, []);

    React.useEffect(() => {
        if (messages.length !== prevLen.current && autoScroll && scrollRef.current) {
            scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
        }
        prevLen.current = messages.length;
    }, [messages, autoScroll]);

    return (
        <div className={clsx("chat-area", className)}>
            <div ref={scrollRef} className="chat-area-content" onScroll={handleScroll}>
                {messages.length === 0 ? (
                    <div className="chat-area-empty">
                        <div className="chat-empty-brand">
                            <i className="fa-solid fa-robot" />
                            <h2>ZeroAI</h2>
                        </div>
                        <p className="chat-empty-desc">Your AI coding assistant</p>
                        <div className="chat-empty-prompts">
                            {SUGGESTED_PROMPTS.map((p) => (
                                <button key={p.text} className="chat-empty-prompt">
                                    <i className={makeIconClass(p.icon, false)} />
                                    <div>
                                        <span className="chat-empty-prompt-text">{p.text}</span>
                                        <span className="chat-empty-prompt-hint">{p.hint}</span>
                                    </div>
                                </button>
                            ))}
                        </div>
                    </div>
                ) : (
                    <div className="chat-messages">
                        {messages.map((msg) => (
                            <MessageRenderer key={msg.id} message={msg} />
                        ))}
                    </div>
                )}
            </div>

            {!autoScroll && messages.length > 0 && (
                <button
                    className="chat-scroll-bottom"
                    onClick={() => {
                        scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: "smooth" });
                        setAutoScroll(true);
                    }}
                >
                    <i className="fa-solid fa-arrow-down" />
                </button>
            )}
        </div>
    );
});
ChatArea.displayName = "ChatArea";
