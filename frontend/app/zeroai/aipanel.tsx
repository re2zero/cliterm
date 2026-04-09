// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { globalStore } from "@/app/store/jotaiStore";
import { useAtomValue } from "jotai";
import { useCallback, useEffect, useRef, useState } from "react";
import { AgentList, ChatArea, ChatInput, ProviderSettings, StatusBar, ZeroAIHeader } from "./components";
import "./index.scss";
import { activeAgentIdAtom, agentsAtom, setActiveAgent } from "./models/agent-model";
import {
    dispatchMessageAction,
    getNextMsgId,
    getStreamingMessage,
    messagesAtom,
    streamingMessageAtom,
} from "./models/message-model";
import { activeModelAtom, activeProviderAtom, activeProviderIdAtom } from "./models/provider-model";
import { activeSessionIdAtom, dispatchSessionAction, removeSession, sessionsAtom } from "./models/session-model";
import {
    inputHeightAtom,
    isStreamingAtom,
    sessionListCollapsedAtom,
    setThinking,
    showProviderSettingsAtom,
    toggleProviderSettings,
    toggleSessionListCollapsed,
} from "./models/ui-model";
import { ZeroAiClient } from "./store/zeroai-client";
import type { CreateSessionRequest, ZeroAiAgentInfo, ZeroAiMessage, ZeroAiSession, ZeroAiSessionInfo } from "./types";

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
    const agents = useAtomValue(agentsAtom);
    const activeAgentId = useAtomValue(activeAgentIdAtom);

    const [inputValue, setInputValue] = useState("");
    const isStreaming = useAtomValue(isStreamingAtom);
    const cancelRef = useRef<AbortController | null>(null);

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
    }, []);

    const currentMessages = activeSessionId ? messagesMap[activeSessionId] || [] : [];
    const streamingMessage = useAtomValue(streamingMessageAtom);
    const displayMessages = activeSessionId
        ? [...currentMessages, ...(streamingMessage[activeSessionId] ? [streamingMessage[activeSessionId]] : [])]
        : currentMessages;
    const currentSession = sessions.find((s) => s.sessionId === activeSessionId);

    const handleSelectSession = (sessionId: string) => {
        dispatchSessionAction({ type: "setActiveSession", sessionId });
    };

    const handleCreateSession = async () => {
        if (!clientRef.current) return;

        try {
            globalStore.set(isStreamingAtom, true);
            setThinking(true);

            const backend = (activeProviderId as CreateSessionRequest["backend"]) || "claude";
            const model = activeModel || "claude-sonnet-4-5";

            const request: CreateSessionRequest = {
                backend,
                model,
                provider: activeProviderId,
                yoloMode: true,
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
            globalStore.set(isStreamingAtom, false);
            setThinking(false);
        }
    };

    const handleDeleteSession = async (sessionId: string) => {
        if (!clientRef.current) return;

        try {
            globalStore.set(isStreamingAtom, true);
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
            globalStore.set(isStreamingAtom, false);
        }
    };

    const handleSendMessage = async () => {
        if (!inputValue.trim() || !clientRef.current || isStreaming) {
            return;
        }

        const content = inputValue.trim();
        setInputValue("");

        let sessionId = activeSessionId;
        if (!sessionId) {
            try {
                globalStore.set(isStreamingAtom, true);
                setThinking(true);

                const backend = (activeProviderId as CreateSessionRequest["backend"]) || "claude";
                const model = activeModel || "claude-sonnet-4-5";

                const request: CreateSessionRequest = {
                    backend,
                    model,
                    provider: activeProviderId,
                    yoloMode: true,
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
                sessionId = result.sessionId;
            } catch (error) {
                console.error("[ZeroAI] Failed to create session:", error);
                globalStore.set(isStreamingAtom, false);
                setThinking(false);
                return;
            }
        }

        const userMsg: ZeroAiMessage = {
            id: getNextMsgId(),
            sessionId,
            role: "user",
            content,
            createdAt: Date.now() / 1000,
        };
        dispatchMessageAction({ type: "addMessage", sessionId, message: userMsg });

        globalStore.set(isStreamingAtom, true);
        setThinking(true);

        dispatchMessageAction({
            type: "startStream",
            sessionId,
            message: {
                role: "assistant",
                content: "",
                sessionId,
                createdAt: Date.now() / 1000,
            },
        });

        const abortController = new AbortController();
        cancelRef.current = abortController;

        const abortPromise = new Promise<IteratorResult<ZeroAiStreamMessageEvent>>((resolve) => {
            abortController.signal.addEventListener("abort", () => resolve({ value: undefined, done: true }), {
                once: true,
            });
        });

        let streamFinished = false;

        try {
            const stream = clientRef.current.streamMessage({
                sessionId,
                role: "user",
                content,
            });

            let iterResult = await stream.next();
            while (!iterResult.done && !abortController.signal.aborted) {
                const event = iterResult.value as ZeroAiStreamMessageEvent | undefined;
                if (event?.message) {
                    const msg = event.message;
                    const eventType = msg.eventType || (msg.metadata?.type as string | undefined);

                    if (eventType === "end_turn") {
                        streamFinished = true;
                        break;
                    }
                    if (
                        eventType === "tool_call" ||
                        eventType === "tool_started" ||
                        eventType === "tool_completed" ||
                        eventType === "tool_failed"
                    ) {
                        dispatchMessageAction({ type: "addMessage", sessionId, message: msg });
                    } else if (eventType === "permission" || eventType === "permission_request") {
                        dispatchMessageAction({ type: "addMessage", sessionId, message: msg });
                    } else if (eventType === "plan_update" || eventType === "plan") {
                        dispatchMessageAction({ type: "addMessage", sessionId, message: msg });
                    } else if (eventType === "error") {
                        dispatchMessageAction({ type: "addMessage", sessionId, message: msg });
                        break;
                    } else if (msg.content) {
                        dispatchMessageAction({ type: "appendChunk", sessionId, chunk: msg });
                    }
                }

                iterResult = await Promise.race([stream.next(), abortPromise]);
            }

            if (!abortController.signal.aborted) {
                streamFinished = true;
            }
        } catch (error) {
            console.error("[ZeroAI] Stream error:", error);
        } finally {
            cancelRef.current = null;

            if (abortController.signal.aborted) {
                dispatchMessageAction({ type: "cancelStream", sessionId });
            } else if (streamFinished) {
                dispatchMessageAction({ type: "finalizeStream", sessionId });
            } else {
                const streaming = getStreamingMessage(sessionId);
                if (streaming && streaming.content) {
                    dispatchMessageAction({ type: "finalizeStream", sessionId });
                } else {
                    dispatchMessageAction({ type: "cancelStream", sessionId });
                }
            }

            globalStore.set(isStreamingAtom, false);
            setThinking(false);
        }
    };

    const handleStopStreaming = () => {
        globalStore.set(isStreamingAtom, false);
        setThinking(false);

        if (cancelRef.current) {
            cancelRef.current.abort();
            cancelRef.current = null;
        }

        const sessionId = activeSessionId;
        if (sessionId && clientRef.current) {
            clientRef.current.cancelStream(sessionId).catch((err) => {
                console.error("[ZeroAI] cancelStream error:", err);
            });
        }
    };

    const handlePermissionConfirm = useCallback(
        async (sessionId: string, callId: string, optionId: string, confirmAll: boolean) => {
            if (!clientRef.current) return;
            try {
                await clientRef.current.confirmPermission(sessionId, callId, optionId, confirmAll);
            } catch (error) {
                console.error("Failed to confirm permission:", error);
            }
        },
        []
    );

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
            />
            <ZeroAIHeader showSettings={showProviderSettings} onToggleSettings={toggleProviderSettings} />
            <div className="zeroai-content">
                {showProviderSettings ? (
                    <div className="zeroai-settings-inline">
                        <ProviderSettings className="zeroai-settings-inline-content" />
                    </div>
                ) : (
                    <>
                        <AgentList
                            agents={agents}
                            activeAgentId={activeAgentId}
                            onSelectAgent={(agentId) => {
                                setActiveAgent(agentId);
                                const agent = agents.find((a) => a.id === agentId);
                                if (agent) {
                                    globalStore.set(activeProviderIdAtom, agent.provider);
                                    globalStore.set(activeModelAtom, agent.model);
                                }
                            }}
                            collapsed={sessionListCollapsed}
                            onToggleCollapse={toggleSessionListCollapsed}
                        />
                        <div className="chat-area-wrapper">
                            <ChatArea
                                messages={displayMessages}
                                isStreaming={isStreaming}
                                onConfirm={handlePermissionConfirm}
                            />
                            <ChatInput
                                value={inputValue}
                                onChange={setInputValue}
                                onSend={handleSendMessage}
                                onStop={handleStopStreaming}
                                isSending={isStreaming}
                            />
                        </div>
                    </>
                )}
            </div>
        </div>
    );
}
