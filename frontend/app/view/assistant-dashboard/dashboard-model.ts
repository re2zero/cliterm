// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { BlockNodeModel } from "@/app/block/blocktypes";
import { globalStore } from "@/app/store/jotaiStore";
import type { TabModel } from "@/app/store/tab-model";
import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";
import { DashboardView } from "./DashboardView";
import * as jotai from "jotai";
import type * as React from "react";

// TypeScript types for worker information derived from task metadata
export type WorkerInfo = {
    taskId: string;
    taskDescription: string;
    workerType?: string;
    status: string;
    blockId?: string;
    createdAt: number;
};

export type AssistantStatus = {
    running: boolean;
    taskCount: number;
    lastUpdated: number;
};

export type AssistantTaskInfo = {
    taskId: string;
    description: string;
    status: string;
    createdAt: number;
    assignedAgentId?: string;
};

// DashboardViewModel manages the state for the assistant dashboard view
export class DashboardViewModel {
    blockId: string;
    viewType = "assistant-dashboard";
    viewIcon = jotai.atom("robot");
    viewName = jotai.atom("Assistant Dashboard");
    viewComponent: React.ComponentType<{ model: DashboardViewModel }> = DashboardView;
    nodeModel: BlockNodeModel;
    tabModel: TabModel;
    env: any;

    // Assistant status atoms
    statusAtom: jotai.PrimitiveAtom<AssistantStatus>;
    statusLoadingAtom: jotai.PrimitiveAtom<boolean>;
    statusErrorAtom: jotai.PrimitiveAtom<string | null>;

    // Task list atoms
    tasksAtom: jotai.PrimitiveAtom<AssistantTaskInfo[]>;
    tasksLoadingAtom: jotai.PrimitiveAtom<boolean>;
    tasksErrorAtom: jotai.PrimitiveAtom<string | null>;

    // Worker list atoms (derived from tasks)
    workersAtom: jotai.Atom<WorkerInfo[]>;
    workersLoadingAtom: jotai.PrimitiveAtom<boolean>;

    // Refresh control atom
    refreshAtom: jotai.WritableAtom<unknown, [], Promise<void>>;
    autoRefreshEnabledAtom: jotai.PrimitiveAtom<boolean>;
    lastRefreshAtom: jotai.PrimitiveAtom<number>;

    // Auto-refresh interval
    private autoRefreshInterval: number | null = null;
    private readonly AUTO_REFRESH_MS = 5000; // Refresh every 5 seconds

    constructor({ blockId, nodeModel, tabModel, waveEnv }: any) {
        this.blockId = blockId;
        this.nodeModel = nodeModel;
        this.tabModel = tabModel;
        this.env = waveEnv;

        // Initialize status atoms
        this.statusAtom = jotai.atom({
            running: false,
            taskCount: 0,
            lastUpdated: Date.now(),
        }) as jotai.PrimitiveAtom<AssistantStatus>;
        this.statusLoadingAtom = jotai.atom(false) as jotai.PrimitiveAtom<boolean>;
        this.statusErrorAtom = jotai.atom(null) as jotai.PrimitiveAtom<string | null>;

        // Initialize task list atoms
        this.tasksAtom = jotai.atom([]) as jotai.PrimitiveAtom<AssistantTaskInfo[]>;
        this.tasksLoadingAtom = jotai.atom(false) as jotai.PrimitiveAtom<boolean>;
        this.tasksErrorAtom = jotai.atom(null) as jotai.PrimitiveAtom<string | null>;

        // Initialize worker list atom (derived from tasks)
        this.workersAtom = jotai.atom((get) => {
            const tasks = get(this.tasksAtom);
            return this.deriveWorkersFromTasks(tasks);
        });

        // Workers use the same loading state as tasks
        this.workersLoadingAtom = this.tasksLoadingAtom;

        // Initialize refresh control
        this.lastRefreshAtom = jotai.atom(Date.now()) as jotai.PrimitiveAtom<number>;
        this.autoRefreshEnabledAtom = jotai.atom(true) as jotai.PrimitiveAtom<boolean>;

        // Refresh atom that triggers status and tasks refresh
        this.refreshAtom = jotai.atom(null, async (get, set) => {
            await this.refreshAll();
        });
    }

    // Derive worker information from task metadata
    private deriveWorkersFromTasks(tasks: AssistantTaskInfo[]): WorkerInfo[] {
        // Workers are tasks that have a worker_id in their metadata
        // For now, we'll identify workers by checking if they're "in_progress" and look for patterns
        // In a full implementation, we'd have explicit worker_id metadata
        
        const workers: WorkerInfo[] = [];
        
        for (const task of tasks) {
            // For now, infer workers based on task status and patterns
            // In production, tasks would have explicit worker metadata
            if (task.status === "in_progress" || task.status === "pending") {
                // Tasks in progress may have associated workers
                // For S04/T01, we create placeholder worker info
                const worker: WorkerInfo = {
                    taskId: task.taskId,
                    taskDescription: task.description,
                    workerType: "default", // Would be derived from task.metadata.worker_type
                    status: task.status,
                    blockId: undefined, // Would be derived from task.metadata.block_id
                    createdAt: task.createdAt,
                };
                workers.push(worker);
            }
        }
        
        return workers;
    }

    // Refresh all data (status and tasks)
    public async refreshAll(): Promise<void> {
        try {
            await Promise.all([
                this.fetchStatus(),
                this.fetchTasks(),
            ]);
            globalStore.set(this.lastRefreshAtom, Date.now());
            console.log("[dashboard] refresh completed successfully");
        } catch (error) {
            console.error("[dashboard] refresh failed:", error);
        }
    }

    // Fetch assistant status from backend
    public async fetchStatus(): Promise<void> {
        globalStore.set(this.statusLoadingAtom, true);
        globalStore.set(this.statusErrorAtom, null);

        try {
            const statusData = await RpcApi.AssistantStatusCommand(TabRpcClient, {});
            
            const status: AssistantStatus = {
                running: statusData.running ?? false,
                taskCount: statusData.taskCount ?? 0,
                lastUpdated: Date.now(),
            };

            globalStore.set(this.statusAtom, status);
            console.log("[dashboard] status fetched:", status);
        } catch (error) {
            const errorMsg = error instanceof Error ? error.message : "Failed to fetch status";
            console.error("[dashboard] status fetch error:", error);
            globalStore.set(this.statusErrorAtom, errorMsg);
        } finally {
            globalStore.set(this.statusLoadingAtom, false);
        }
    }

    // Fetch task list from backend
    public async fetchTasks(): Promise<void> {
        globalStore.set(this.tasksLoadingAtom, true);
        globalStore.set(this.tasksErrorAtom, null);

        try {
            const tasksData = await RpcApi.AssistantListTasksCommand(TabRpcClient, {});
            
            const tasks = tasksData.tasks ?? [];
            globalStore.set(this.tasksAtom, tasks);
            console.log("[dashboard] tasks fetched:", tasks.length, "tasks");
        } catch (error) {
            const errorMsg = error instanceof Error ? error.message : "Failed to fetch tasks";
            console.error("[dashboard] tasks fetch error:", error);
            globalStore.set(this.tasksErrorAtom, errorMsg);
        } finally {
            globalStore.set(this.tasksLoadingAtom, false);
        }
    }

    // Start auto-refresh loop
    public startAutoRefresh(): void {
        if (this.autoRefreshInterval !== null) {
            console.log("[dashboard] auto-refresh already running");
            return;
        }

        const enabled = globalStore.get(this.autoRefreshEnabledAtom);
        if (!enabled) {
            console.log("[dashboard] auto-refresh disabled");
            return;
        }

        console.log("[dashboard] starting auto-refresh (interval:", this.AUTO_REFRESH_MS, "ms)");
        
        // Initial fetch
        this.refreshAll();

        this.autoRefreshInterval = window.setInterval(() => {
            const enabled = globalStore.get(this.autoRefreshEnabledAtom);
            if (enabled) {
                this.refreshAll();
            }
        }, this.AUTO_REFRESH_MS);
    }

    // Stop auto-refresh loop
    public stopAutoRefresh(): void {
        if (this.autoRefreshInterval !== null) {
            clearInterval(this.autoRefreshInterval);
            this.autoRefreshInterval = null;
            console.log("[dashboard] auto-refresh stopped");
        }
    }

    // Toggle auto-refresh
    public toggleAutoRefresh(): void {
        const enabled = globalStore.get(this.autoRefreshEnabledAtom);
        globalStore.set(this.autoRefreshEnabledAtom, !enabled);

        if (!enabled) {
            // Was disabled, now enabling
            if (this.autoRefreshInterval === null) {
                this.startAutoRefresh();
            }
        } else {
            // Was enabled, now disabling - stop the interval
            this.stopAutoRefresh();
        }
    }

    // Add a new task
    public async addTask(description: string): Promise<string> {
        if (!description || description.trim() === "") {
            throw new Error("Task description cannot be empty");
        }

        try {
            const result = await RpcApi.AssistantAddTaskCommand(TabRpcClient, {
                description: description.trim(),
            });

            // Refresh task list after adding
            await this.fetchTasks();
            
            console.log("[dashboard] task added:", result.taskId);
            return result.taskId;
        } catch (error) {
            const errorMsg = error instanceof Error ? error.message : "Failed to add task";
            console.error("[dashboard] add task error:", error);
            throw new Error(errorMsg);
        }
    }

    // Start the assistant
    public async startAssistant(): Promise<void> {
        try {
            await RpcApi.AssistantStartCommand(TabRpcClient, {});
            // Refresh status after starting
            await this.fetchStatus();
            console.log("[dashboard] assistant started");
        } catch (error) {
            const errorMsg = error instanceof Error ? error.message : "Failed to start assistant";
            console.error("[dashboard] start assistant error:", error);
            throw new Error(errorMsg);
        }
    }

    // Stop the assistant
    public async stopAssistant(): Promise<void> {
        try {
            await RpcApi.AssistantStopCommand(TabRpcClient, {});
            // Refresh status after stopping
            await this.fetchStatus();
            console.log("[dashboard] assistant stopped");
        } catch (error) {
            const errorMsg = error instanceof Error ? error.message : "Failed to stop assistant";
            console.error("[dashboard] stop assistant error:", error);
            throw new Error(errorMsg);
        }
    }

    // Cleanup method to stop auto-refresh when dashboard is closed
    public dispose(): void {
        this.stopAutoRefresh();
    }
}
