// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { useAtomValue } from "jotai";
import { useEffect, useRef, useState } from "react";
import { ChatArea, ChatInput, ProviderSettings, SessionList, StatusBar, ZeroAIHeader } from "./components";
import "./index.scss";
import { dispatchMessageAction, messagesAtom } from "./models/message-model";
import { activeModelAtom, activeProviderAtom, activeProviderIdAtom } from "./models/provider-model";
import { activeSessionIdAtom, dispatchSessionAction, removeSession, sessionsAtom } from "./models/session-model";
import {
    inputHeightAtom,
    sessionListCollapsedAtom,
    setThinking,
    showProviderSettingsAtom,
    toggleProviderSettings,
    toggleSessionListCollapsed,
} from "./models/ui-model";
import { ZeroAiClient } from "./store/zeroai-client";
import type { CreateSessionRequest, ZeroAiAgentInfo, ZeroAiSession, ZeroAiSessionInfo } from "./types";

export function AIPanel(_props: { roundTopLeft?: boolean }) {
    const sessions = useAtomValue(sessionsAtom);
    const activeSessionId = useAtomValue(activeSessionIdAtom);
    const messagesMap = useAtomValue(messagesAtom);
    const inputHeight = useAtomValue(inputHeightAtom);
    const showProviderSettings = useAtomValue(showProviderSettingsAtom);
    const sessionListCollapsed = useAtomValue(sessionListCollapsedAtom);
    const activeProviderId = useAtomValue(activeProviderIdAtom);
    const activeModel = useAtomValue(activeModelAtom);
    const activeProvider = useAtomValue(activeProviderAtom);

    const [inputValue, setInputValue] = useState("");
    const [isStreaming, setIsStreaming] = useState(false);

    const clientRef = useRef<ZeroAiClient | null>(null);

    useEffect(() => {
        const client = new ZeroAiClient();
        clientRef.current = client;

        const initializeSessions = async () => {
            try {
                const sessionList = await client.listSessions();
                dispatchSessionAction({ type: "setSessions", sessions: sessionList });
            } catch (error) {
                console.error("Failed to load sessions:", error);
            }
        };

        initializeSessions();

        return () => {
            setIsStreaming(false);
        };
    }, []);

    const currentMessages = activeSessionId ? messagesMap[activeSessionId] || [] : [];
    const currentSession = sessions.find((s) => s.sessionId === activeSessionId);

    const handleSelectSession = (sessionId: string) => {
        dispatchSessionAction({ type: "setActiveSession", sessionId });
    };

    const handleCreateSession = async () => {
        if (!clientRef.current) return;

        try {
            setIsStreaming(true);
            setThinking(true);

            const backend = (activeProviderId as CreateSessionRequest["backend"]) || "claude";
            const model = activeModel || "claude-sonnet-4-5";

            const request: CreateSessionRequest = {
                backend,
                model,
                provider: activeProviderId,
            };

            const result = await clientRef.current.createSession(request);

            const newSession: ZeroAiSessionInfo = {
                sessionId: result.sessionId,
                provider: activeProviderId,
                model,
                workDir: null,
                createdAt: Date.now() / 1000,
                lastMessageAt: Date.now() / 1000,
            };

            dispatchSessionAction({ type: "addSession", session: newSession, setActive: true });
        } catch (error) {
            console.error("Failed to create session:", error);
        } finally {
            setIsStreaming(false);
            setThinking(false);
        }
    };

    const handleDeleteSession = async (sessionId: string) => {
        if (!clientRef.current) return;

        try {
            setIsStreaming(true);
            await clientRef.current.deleteSession(sessionId);
            removeSession(sessionId);

            if (activeSessionId === sessionId) {
                const remaining = sessions.filter((s) => s.sessionId !== sessionId);
                dispatchSessionAction({
                    type: "setActiveSession",
                    sessionId: remaining.length > 0 ? remaining[0].sessionId : null,
                });
            }

            dispatchMessageAction({ type: "deleteSession", sessionId });
        } catch (error) {
            console.error("Failed to delete session:", error);
        } finally {
            setIsStreaming(false);
        }
    };

    const handleSendMessage = async () => {
        if (!inputValue.trim() || !activeSessionId || !clientRef.current || isStreaming) {
            return;
        }

        const content = inputValue.trim();
        setInputValue("");
        setIsStreaming(true);
        setThinking(true);

        try {
            const stream = clientRef.current.streamMessage({
                sessionId: activeSessionId,
                role: "user",
                content,
            });

            for await (const event of stream) {
                if (event.message) {
                    const msg = event.message;
                    const eventType = msg.eventType || (msg.metadata?.type as string | undefined);

                    if (
                        eventType === "tool_call" ||
                        eventType === "tool_started" ||
                        eventType === "tool_completed" ||
                        eventType === "tool_failed"
                    ) {
                        dispatchMessageAction({ type: "addMessage", sessionId: activeSessionId, message: msg });
                    } else if (eventType === "permission" || eventType === "permission_request") {
                        dispatchMessageAction({ type: "addMessage", sessionId: activeSessionId, message: msg });
                    } else if (eventType === "plan_update" || eventType === "plan") {
                        dispatchMessageAction({ type: "addMessage", sessionId: activeSessionId, message: msg });
                    } else if (eventType === "error") {
                        dispatchMessageAction({ type: "addMessage", sessionId: activeSessionId, message: msg });
                    } else if (eventType === "end_turn") {
                        // End of assistant response
                    } else if (msg.content) {
                        dispatchMessageAction({ type: "addMessage", sessionId: activeSessionId, message: msg });
                    }
                }
            }
        } catch (error) {
            console.error("Failed to send message:", error);
        } finally {
            setIsStreaming(false);
            setThinking(false);
        }
    };

    const minHeight = typeof inputHeight === "number" ? inputHeight : 100;

    const agentInfo: ZeroAiAgentInfo | undefined = currentSession
        ? {
              backend: "claude",
              model: currentSession.model,
              provider: currentSession.provider,
              displayName: "Claude Code",
              description: "AI coding assistant",
              enabled: true,
              supportedOps: ["chat", "edit"],
          }
        : undefined;

    return (
        <div className="zeroai-panel">
            <StatusBar
                session={currentSession as unknown as ZeroAiSession}
                agentInfo={agentInfo}
                onWorkDirClick={() => console.log("Change work dir")}
                isStreaming={isStreaming}
            />
            <ZeroAIHeader showSettings={showProviderSettings} onToggleSettings={toggleProviderSettings} />
            <div className="zeroai-content">
                {showProviderSettings ? (
                    <div className="zeroai-settings-inline">
                        <ProviderSettings className="zeroai-settings-inline-content" />
                    </div>
                ) : (
                    <>
                        <SessionList
                            sessions={sessions as unknown as ZeroAiSession[]}
                            currentSessionId={activeSessionId || undefined}
                            onSelectSession={handleSelectSession}
                            onCreateSession={handleCreateSession}
                            onDeleteSession={handleDeleteSession}
                            collapsed={sessionListCollapsed}
                            onToggleCollapse={toggleSessionListCollapsed}
                        />
                        <div className="chat-area-wrapper">
                            <ChatArea messages={currentMessages} />
                            <ChatInput
                                value={inputValue}
                                onChange={setInputValue}
                                onSend={handleSendMessage}
                                isSending={isStreaming}
                            />
                        </div>
                    </>
                )}
            </div>
        </div>
    );
}
