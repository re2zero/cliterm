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

    if (diffMinutes < 1) return "now";
    if (diffMinutes < 60) return `${diffMinutes}m`;
    if (diffMinutes < 1440) return `${Math.floor(diffMinutes / 60)}h`;
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
                <div className="chat-msg-meta">
                    <span className="chat-msg-time">{formatTimestamp(message.createdAt)}</span>
                    <span className="chat-msg-role">You</span>
                </div>
                <div className="chat-msg-bubble">
                    <div className="chat-msg-text">{message.content}</div>
                </div>
                <div className="chat-msg-actions" style={{ justifyContent: "flex-end" }}>
                    <button onClick={handleCopy} title="Copy">
                        <i className={makeIconClass(copied ? "fa-solid fa-check" : "fa-regular fa-copy", false)} />
                    </button>
                </div>
            </div>
        </div>
    );
});
UserMessage.displayName = "UserMessage";

const AssistantMessage = React.memo(({ message, isStreaming }: { message: ZeroAiMessage; isStreaming?: boolean }) => {
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
                {isStreaming && (
                    <span className="chat-avatar-streaming">
                        <span className="chat-avatar-streaming-dot" />
                    </span>
                )}
            </div>
            <div className="chat-msg-body">
                <div className="chat-msg-meta">
                    <span className="chat-msg-role">Assistant</span>
                    {isStreaming && <span className="chat-msg-status-streaming">typing</span>}
                    {thinking && (
                        <button className="chat-thinking-toggle" onClick={() => setThinkingOpen(!thinkingOpen)}>
                            <i className={makeIconClass("fa-solid fa-brain", false)} />
                            <span>Think</span>
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

                <div className="chat-msg-bubble">
                    {content && (
                        <div className="chat-msg-markdown">
                            <WaveStreamdown text={content} parseIncompleteMarkdown={false} />
                        </div>
                    )}
                </div>

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
    const toolName =
        (toolData?.toolName as string) || (toolData?.title as string) || (toolData?.kind as string) || "tool";
    const rawInput = toolData?.rawInput as Record<string, unknown> | undefined;
    const command = (rawInput?.command as string) || "";
    const description = (rawInput?.description as string) || (toolData?.title as string) || "";
    const status = (toolData?.status as string) || "running";
    const sessionUpdate = (toolData?.sessionUpdate as string) || "";
    const rawOutput = (toolData?.rawOutput as string) || "";

    const statusIcon =
        status === "completed"
            ? "fa-solid fa-check-circle"
            : status === "failed"
              ? "fa-solid fa-times-circle"
              : "fa-solid fa-spinner fa-spin";

    const isUpdate = sessionUpdate === "tool_call_update";

    return (
        <div className={clsx("chat-tool", `status-${status}`, { "chat-tool-update": isUpdate })}>
            <button className="chat-tool-header" onClick={() => setExpanded(!expanded)}>
                <i className={makeIconClass("fa-solid fa-wrench", false)} />
                <span className="chat-tool-name">{toolName}</span>
                {!isUpdate && <i className={makeIconClass(statusIcon, false)} />}
                <i
                    className={clsx(makeIconClass("fa-solid fa-chevron-down", false), "chat-tool-chevron", {
                        expanded,
                    })}
                />
            </button>
            {expanded && (
                <div className="chat-tool-body">
                    {command && (
                        <div className="chat-tool-section">
                            <span className="chat-tool-section-label">Command</span>
                            <pre className="chat-tool-code">{command}</pre>
                        </div>
                    )}
                    {description && (
                        <div className="chat-tool-section">
                            <span className="chat-tool-section-label">Description</span>
                            <pre className="chat-tool-code">{description}</pre>
                        </div>
                    )}
                    {rawOutput && (
                        <div className="chat-tool-section">
                            <span className="chat-tool-section-label">Output</span>
                            <pre
                                className={`chat-tool-code chat-tool-output ${status === "failed" ? "status-failed" : status === "completed" ? "status-completed" : ""}`}
                            >
                                {rawOutput}
                            </pre>
                        </div>
                    )}
                </div>
            )}
        </div>
    );
});
ToolCallMessage.displayName = "ToolCallMessage";

const PermissionMessage = React.memo(
    ({
        message,
        onConfirm,
    }: {
        message: ZeroAiMessage;
        onConfirm?: (sessionId: string, callId: string, optionId: string, confirmAll: boolean) => void;
    }) => {
        const toolData = message.metadata as Record<string, unknown> | undefined;
        const rawInput = (toolData?.rawInput as Record<string, unknown>) || undefined;
        const toolName =
            (toolData?.toolName as string) ||
            (rawInput?.command as string) ||
            (toolData?.title as string) ||
            "unknown tool";
        const description = (rawInput?.description as string) || (toolData?.title as string) || "";
        const callId = (toolData?.toolCallId as string) || (message.metadata?.callId as string) || "";
        const sessionId = message.sessionId;

        const rawOptions = (message.metadata?.options as ZeroAiPermissionRequest["options"]) || [];
        const options =
            rawOptions.length > 0
                ? rawOptions
                : [
                      { id: "allow", label: "Allow", description: "" },
                      { id: "reject", label: "Reject", description: "" },
                  ];

        const handleConfirm = React.useCallback(
            async (optionId: string, confirmAll: boolean) => {
                if (onConfirm && sessionId && callId) {
                    onConfirm(sessionId, callId, optionId, confirmAll);
                } else {
                    console.log("Confirm permission (no handler):", { sessionId, callId, optionId, confirmAll });
                }
            },
            [sessionId, callId, onConfirm]
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
    }
);
PermissionMessage.displayName = "PermissionMessage";

const PlanUpdateMessage = React.memo(({ message }: { message: ZeroAiMessage }) => {
    const [expanded, setExpanded] = React.useState(false);

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
                    <WaveStreamdown text={message.content} parseIncompleteMarkdown={false} />
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

const MessageRenderer = React.memo(
    ({
        message,
        isLastAssistant,
        isStreaming,
        onConfirm,
    }: {
        message: ZeroAiMessage;
        isLastAssistant?: boolean;
        isStreaming?: boolean;
        onConfirm?: (sessionId: string, callId: string, optionId: string, confirmAll: boolean) => void;
    }) => {
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
            return <PermissionMessage message={message} onConfirm={onConfirm} />;
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
        return <AssistantMessage message={message} isStreaming={isLastAssistant && isStreaming} />;
    }
);
MessageRenderer.displayName = "MessageRenderer";

const SUGGESTED_PROMPTS = [
    { icon: "fa-solid fa-code", text: "Explain this codebase", hint: "Analyze the current project" },
    { icon: "fa-solid fa-bug", text: "Find and fix bugs", hint: "Debug issues in the code" },
    { icon: "fa-solid fa-pen-ruler", text: "Refactor this module", hint: "Improve code structure" },
    { icon: "fa-solid fa-file-lines", text: "Write documentation", hint: "Generate docs for the code" },
];

export const ChatArea = React.memo(
    ({
        messages,
        isStreaming,
        className,
        onConfirm,
    }: {
        messages: ZeroAiMessage[];
        isStreaming?: boolean;
        className?: string;
        onConfirm?: (sessionId: string, callId: string, optionId: string, confirmAll: boolean) => void;
    }) => {
        const scrollRef = React.useRef<HTMLDivElement>(null);
        const [autoScroll, setAutoScroll] = React.useState(true);
        const rafIdRef = React.useRef<number>(0);

        const lastAssistantIdx = React.useMemo(() => {
            for (let i = messages.length - 1; i >= 0; i--) {
                if (messages[i].role === "assistant" && !messages[i].eventType) return i;
            }
            return -1;
        }, [messages]);

        const contentFingerprint = React.useMemo(() => {
            const last = messages[messages.length - 1];
            if (!last) return "";
            return `${last.role}:${last.content?.length ?? 0}:${last.eventType ?? ""}`;
        }, [messages]);

        const handleScroll = React.useCallback(() => {
            if (!scrollRef.current) return;
            const { scrollTop, scrollHeight, clientHeight } = scrollRef.current;
            setAutoScroll(scrollHeight - scrollTop - clientHeight < 80);
        }, []);

        React.useEffect(() => {
            if (!autoScroll || !scrollRef.current) return;
            cancelAnimationFrame(rafIdRef.current);
            rafIdRef.current = requestAnimationFrame(() => {
                if (scrollRef.current) {
                    scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
                }
            });
        }, [contentFingerprint, messages.length, autoScroll]);

        React.useEffect(() => {
            return () => cancelAnimationFrame(rafIdRef.current);
        }, []);

        return (
            <div className={clsx("chat-area", className)}>
                <div ref={scrollRef} className="chat-area-content" onScroll={handleScroll}>
                    {messages.length === 0 ? (
                        <div className="chat-area-empty">
                            {isStreaming ? (
                                <>
                                    <div className="chat-empty-brand">
                                        <i className="fa-solid fa-spinner fa-spin" />
                                        <h2>Connecting...</h2>
                                    </div>
                                    <p className="chat-empty-desc">Starting agent, please wait</p>
                                </>
                            ) : (
                                <>
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
                                </>
                            )}
                        </div>
                    ) : (
                        <div className="chat-messages">
                            {messages.map((msg, idx) => (
                                <MessageRenderer
                                    key={msg.id}
                                    message={msg}
                                    isLastAssistant={idx === lastAssistantIdx}
                                    isStreaming={isStreaming}
                                    onConfirm={onConfirm}
                                />
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
    }
);
ChatArea.displayName = "ChatArea";
