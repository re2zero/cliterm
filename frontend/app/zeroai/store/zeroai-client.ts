// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";
import type {
    CreateSessionRequest,
    DeleteProviderRequest,
    SaveProviderRequest,
    SendMessageRequest,
    TestProviderResult,
    ZeroAiAgentInfo,
    ZeroAiMessage,
    ZeroAiProviderInfo,
    ZeroAiSession,
    ZeroAiSessionInfo,
    ZeroAiStreamMessageEvent,
} from "../types";

const ZEROAI_ROUTE = "zeroai";

export class ZeroAiClientError extends Error {
    constructor(
        message: string,
        public code?: string,
        public details?: any
    ) {
        super(message);
        this.name = "ZeroAiClientError";
    }
}

export interface ZeroAiClientOpts {
    noresponse?: boolean;
    timeout?: number;
    route?: string;
}

function zeroaiOpts(opts?: ZeroAiClientOpts): ZeroAiClientOpts {
    return { ...opts, route: ZEROAI_ROUTE };
}

export class ZeroAiClient {
    async createSession(request: CreateSessionRequest, opts?: ZeroAiClientOpts): Promise<{ sessionId: string }> {
        try {
            const createRequest = {
                backend: request.backend,
                model: request.model,
                provider: request.provider || "",
                thinkingLevel: request.thinkingLevel || "",
                yoloMode: request.yoloMode ?? false,
                workDir: request.workDir || "",
            };
            return await RpcApi.ZeroAiCreateSessionCommand(TabRpcClient, createRequest, zeroaiOpts(opts));
        } catch (error) {
            throw new ZeroAiClientError(`Failed to create session: ${error}`, "CREATE_SESSION_ERROR", error);
        }
    }

    async getSession(sessionId: string, opts?: ZeroAiClientOpts): Promise<ZeroAiSession> {
        try {
            const result = await RpcApi.ZeroAiGetSessionCommand(TabRpcClient, { sessionId }, zeroaiOpts(opts));
            return result as unknown as ZeroAiSession;
        } catch (error) {
            throw new ZeroAiClientError(`Failed to get session: ${error}`, "GET_SESSION_ERROR", error);
        }
    }

    async listSessions(opts?: ZeroAiClientOpts): Promise<ZeroAiSessionInfo[]> {
        try {
            const result = await RpcApi.ZeroAiListSessionsCommand(TabRpcClient, {}, zeroaiOpts(opts));
            return result.sessions || [];
        } catch (error) {
            throw new ZeroAiClientError(`Failed to list sessions: ${error}`, "LIST_SESSIONS_ERROR", error);
        }
    }

    async deleteSession(sessionId: string, opts?: ZeroAiClientOpts): Promise<void> {
        try {
            await RpcApi.ZeroAiDeleteSessionCommand(TabRpcClient, { sessionId }, zeroaiOpts(opts));
        } catch (error) {
            throw new ZeroAiClientError(`Failed to delete session: ${error}`, "DELETE_SESSION_ERROR", error);
        }
    }

    async setWorkDir(sessionId: string, workDir: string, opts?: ZeroAiClientOpts): Promise<void> {
        try {
            await RpcApi.ZeroAiSetWorkDirCommand(TabRpcClient, { sessionId, workDir }, zeroaiOpts(opts));
        } catch (error) {
            throw new ZeroAiClientError(`Failed to set work directory: ${error}`, "SET_WORKDIR_ERROR", error);
        }
    }

    async sendMessage(request: SendMessageRequest, opts?: ZeroAiClientOpts): Promise<{ messageId: number }> {
        try {
            const sendMessageRequest = {
                sessionId: request.sessionId,
                role: request.role,
                content: request.content,
                eventType: request.eventType || "",
                metadata: request.metadata || {},
            };
            return await RpcApi.ZeroAiSendMessageCommand(TabRpcClient, sendMessageRequest, zeroaiOpts(opts));
        } catch (error) {
            throw new ZeroAiClientError(`Failed to send message: ${error}`, "SEND_MESSAGE_ERROR", error);
        }
    }

    async *streamMessage(
        request: SendMessageRequest,
        opts?: ZeroAiClientOpts
    ): AsyncGenerator<ZeroAiStreamMessageEvent, void, unknown> {
        try {
            const stream = RpcApi.ZeroAiSendStreamMessageCommand(TabRpcClient, request, zeroaiOpts(opts));
            for await (const event of stream) {
                yield event as ZeroAiStreamMessageEvent;
            }
        } catch (error) {
            throw new ZeroAiClientError(`Stream failed: ${error}`, "STREAM_ERROR", error);
        }
    }

    async getMessages(
        sessionId: string,
        opts?: ZeroAiClientOpts & { limit?: number; offset?: number }
    ): Promise<ZeroAiMessage[]> {
        try {
            const result = await RpcApi.ZeroAiGetMessagesCommand(
                TabRpcClient,
                {
                    sessionId,
                    limit: opts?.limit ?? 100,
                    offset: opts?.offset ?? 0,
                },
                zeroaiOpts(opts)
            );
            return result.messages || [];
        } catch (error) {
            throw new ZeroAiClientError(`Failed to get messages: ${error}`, "GET_MESSAGES_ERROR", error);
        }
    }

    async getAgents(opts?: ZeroAiClientOpts): Promise<ZeroAiAgentInfo[]> {
        try {
            return await RpcApi.ZeroAiGetAgentsCommand(TabRpcClient, {}, zeroaiOpts(opts));
        } catch (error) {
            throw new ZeroAiClientError(`Failed to get agents: ${error}`, "GET_AGENTS_ERROR", error);
        }
    }

    async confirmPermission(
        sessionId: string,
        callId: string,
        optionId: string,
        confirmAll: boolean = false,
        opts?: ZeroAiClientOpts
    ): Promise<void> {
        try {
            await RpcApi.ZeroAiConfirmPermissionCommand(
                TabRpcClient,
                {
                    sessionId,
                    callId,
                    optionId,
                    confirmAll,
                },
                zeroaiOpts(opts)
            );
        } catch (error) {
            throw new ZeroAiClientError(`Failed to confirm permission: ${error}`, "CONFIRM_PERMISSION_ERROR", error);
        }
    }

    async cancelStream(sessionId: string, opts?: ZeroAiClientOpts): Promise<void> {
        try {
            await RpcApi.ZeroAiCancelStreamCommand(TabRpcClient, { sessionId }, zeroaiOpts(opts));
        } catch (error) {
            throw new ZeroAiClientError(`Failed to cancel stream: ${error}`, "CANCEL_STREAM_ERROR", error);
        }
    }

    async listProviders(opts?: ZeroAiClientOpts): Promise<ZeroAiProviderInfo[]> {
        const result = await RpcApi.ZeroAiListProvidersCommand(TabRpcClient, {}, zeroaiOpts(opts));
        return result.providers || [];
    }

    async saveProvider(request: SaveProviderRequest, opts?: ZeroAiClientOpts): Promise<void> {
        const saveRequest = {
            providerId: request.providerId,
            displayName: request.displayName,
            displayIcon: request.displayIcon || "",
            cliCommand: request.cliCommand,
            cliPath: request.cliPath || "",
            cliArgs: request.cliArgs || [],
            envVars: request.envVars || {},
            supportsStreaming: request.supportsStreaming ?? false,
            defaultModel: request.defaultModel || "",
            availableModels: request.availableModels || [],
            authRequired: request.authRequired ?? false,
        };
        await RpcApi.ZeroAiSaveProviderCommand(TabRpcClient, saveRequest, zeroaiOpts(opts));
    }

    async deleteProvider(request: DeleteProviderRequest, opts?: ZeroAiClientOpts): Promise<void> {
        await RpcApi.ZeroAiDeleteProviderCommand(TabRpcClient, request, zeroaiOpts(opts));
    }

    async testProvider(providerId: string, opts?: ZeroAiClientOpts): Promise<TestProviderResult> {
        const result = await RpcApi.ZeroAiTestProviderCommand(TabRpcClient, { providerId }, zeroaiOpts(opts));
        return result.result as TestProviderResult;
    }
}

export const zeroAiClient = new ZeroAiClient();

export async function retryRpcCall<T>(
    fn: () => Promise<T>,
    maxRetries: number = 3,
    baseDelay: number = 1000
): Promise<T> {
    for (let attempt = 0; attempt < maxRetries; attempt++) {
        try {
            return await fn();
        } catch (error) {
            const isLastAttempt = attempt === maxRetries - 1;
            if (isLastAttempt) throw error;
            const delay = baseDelay * Math.pow(2, attempt);
            await new Promise((resolve) => setTimeout(resolve, delay));
        }
    }
    throw new Error("Retry failed");
}

export function withRetry<T>(fn: () => Promise<T>, opts: { retries?: number; delay?: number } = {}): Promise<T> {
    return retryRpcCall(fn, opts.retries ?? 3, opts.delay ?? 1000);
}
