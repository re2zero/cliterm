// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { globalStore } from "@/app/store/jotaiStore";
import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";
import type { TabModel } from "@/app/store/tab-model";
import * as jotai from "jotai";
import type * as React from "react";
import { AgentManagementView } from "./AgentManagementView";

// Import types from generated bindings
import type { Agent, MCPConnection } from "@/types/gotypes";

// View state types (extends backend Agent with display properties)
export type AgentViewState = Agent & {
    displayColor: string;
    displayIcon: string;
};

export type FormError = {
    field: keyof Agent;
    message: string;
};

// DEFAULT_COLORS for agent display colors (matching agent-list.tsx)
const DEFAULT_COLORS = ["#d97706", "#6366f1", "#2563eb", "#10b981", "#ec4899", "#8b5cf6", "#f59e0b", "#ef4444"];

// DEFAULT_ICON for agent display
const DEFAULT_ICON = "fa-solid fa-robot";

// Dialog state type
export type DialogState = null | {
    mode: "create" | "edit";
    agent?: Agent;
};

// AgentManagementViewModel manages the state for the agent management view
export class AgentManagementViewModel {
    blockId: string;
    viewType = "agent-management";
    viewIcon = jotai.atom("users-gear");
    viewName = jotai.atom("Agent Management");
    viewComponent: React.ComponentType<{ model: AgentManagementViewModel }> = AgentManagementView;
    nodeModel: any;
    tabModel: TabModel;
    env: any;

    // Agent list atoms
    agentsAtom: jotai.PrimitiveAtom<AgentViewState[]>;
    loadingAtom: jotai.PrimitiveAtom<boolean>;
    errorAtom: jotai.PrimitiveAtom<string | null>;

    // Dialog state atom
    dialogStateAtom: jotai.PrimitiveAtom<DialogState>;

    // Selected agent atom
    selectedAgentIdAtom: jotai.PrimitiveAtom<string | null>;

    constructor({ blockId, nodeModel, tabModel, waveEnv }: any) {
        this.blockId = blockId;
        this.nodeModel = nodeModel;
        this.tabModel = tabModel;
        this.env = waveEnv;

        // Initialize agent list atoms
        this.agentsAtom = jotai.atom([]) as jotai.PrimitiveAtom<AgentViewState[]>;
        this.loadingAtom = jotai.atom(false) as jotai.PrimitiveAtom<boolean>;
        this.errorAtom = jotai.atom(null) as jotai.PrimitiveAtom<string | null>;

        // Initialize dialog state atom
        this.dialogStateAtom = jotai.atom(null) as jotai.PrimitiveAtom<DialogState>;

        // Initialize selected agent atom
        this.selectedAgentIdAtom = jotai.atom(null) as jotai.PrimitiveAtom<string | null>;
    }

    // Private instance for singleton pattern
    private static instance: AgentManagementViewModel | null = null;

    // Get singleton instance
    static getInstance(config: any): AgentManagementViewModel {
        if (!AgentManagementViewModel.instance) {
            AgentManagementViewModel.instance = new AgentManagementViewModel(config);
        }
        return AgentManagementViewModel.instance;
    }

    // Map backend Agent to view state with display properties
    private mapAgentToViewState(agent: Agent): AgentViewState {
        // Use consistent color based on agent id
        const colorIndex = agent.id.split("").reduce((acc, char) => acc + char.charCodeAt(0), 0) % DEFAULT_COLORS.length;
        const displayColor = DEFAULT_COLORS[colorIndex];

        return {
            ...agent,
            displayColor,
            displayIcon: DEFAULT_ICON,
        };
    }

    // Load all agents from backend
    public async loadAgents(): Promise<void> {
        globalStore.set(this.loadingAtom, true);
        globalStore.set(this.errorAtom, null);

        try {
            const result = await RpcApi.AgentListCommand(TabRpcClient, {});
            const agents = result.agents || [];

            // Map agents to view state
            const viewAgents: AgentViewState[] = agents.map((agent) => this.mapAgentToViewState(agent));

            globalStore.set(this.agentsAtom, viewAgents);
            console.log("[agent-management] loaded agents:", viewAgents.length, "agents");
        } catch (error) {
            const errorMsg = error instanceof Error ? error.message : "Failed to load agents";
            console.error("[agent-management] load agents error:", error);
            globalStore.set(this.errorAtom, errorMsg);
        } finally {
            globalStore.set(this.loadingAtom, false);
        }
    }

    // Load single agent from backend by id
    public async loadAgent(id: string): Promise<Agent | null> {
        try {
            const result = await RpcApi.AgentGetCommand(TabRpcClient, { id });
            const agent = result.agent || null;

            if (agent) {
                console.log("[agent-management] loaded agent:", id);
            }

            return agent;
        } catch (error) {
            const errorMsg = error instanceof Error ? error.message : "Failed to load agent";
            console.error("[agent-management] load agent error:", id, error);
            throw new Error(errorMsg);
        }
    }

    // Create new agent
    public async createAgent(data: Omit<Agent, "id" | "createdAt" | "updatedAt">): Promise<string> {
        try {
            const result = await RpcApi.AgentRegisterCommand(TabRpcClient, data);
            const agentId = result.id;

            // Refresh agent list after creation
            await this.loadAgents();

            console.log("[agent-management] agent created:", agentId);
            return agentId;
        } catch (error) {
            const errorMsg = error instanceof Error ? error.message : "Failed to create agent";
            console.error("[agent-management] create agent error:", error);
            throw new Error(errorMsg);
        }
    }

    // Update existing agent
    public async updateAgent(id: string, data: Partial<Agent>): Promise<void> {
        try {
            await RpcApi.AgentUpdateCommand(TabRpcClient, { id, ...data });

            // Refresh agent list after update
            await this.loadAgents();

            console.log("[agent-management] agent updated:", id);
        } catch (error) {
            const errorMsg = error instanceof Error ? error.message : "Failed to update agent";
            console.error("[agent-management] update agent error:", id, error);
            throw new Error(errorMsg);
        }
    }

    // Delete agent
    public async deleteAgent(id: string): Promise<void> {
        try {
            await RpcApi.AgentDeleteCommand(TabRpcClient, { id });

            // Refresh agent list after deletion
            await this.loadAgents();

            console.log("[agent-management] agent deleted:", id);
        } catch (error) {
            const errorMsg = error instanceof Error ? error.message : "Failed to delete agent";
            console.error("[agent-management] delete agent error:", id, error);
            throw new Error(errorMsg);
        }
    }

    // Open create dialog
    public openCreateDialog(): void {
        globalStore.set(this.dialogStateAtom, { mode: "create" });
    }

    // Open edit dialog for existing agent
    public openEditDialog(agent: Agent): void {
        globalStore.set(this.dialogStateAtom, { mode: "edit", agent });
    }

    // Close dialog
    public closeDialog(): void {
        globalStore.set(this.dialogStateAtom, null);
    }

    // Select agent
    public selectAgent(id: string | null): void {
        globalStore.set(this.selectedAgentIdAtom, id);
    }
}
