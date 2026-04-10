// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect, beforeEach, vi } from "vitest";
import { DashboardViewModel, type AssistantStatus, type WorkerInfo } from "@/app/view/assistant-dashboard/dashboard-model";

// Mock the global window object for Node.js environment
global.window = global.window || {
    setInterval: vi.fn(() => 123),
    clearInterval: vi.fn(),
};

// Mock the RPC API
vi.mock("@/app/store/wshclientapi", () => ({
    RpcApi: {
        AssistantStatusCommand: vi.fn(),
        AssistantListTasksCommand: vi.fn(),
        AssistantAddTaskCommand: vi.fn(),
        AssistantStartCommand: vi.fn(),
        AssistantStopCommand: vi.fn(),
    },
}));

// Mock the TabRpcClient
vi.mock("@/app/store/wshrpcutil", () => ({
    TabRpcClient: {},
}));

// Mock the globalStore
vi.mock("@/app/store/jotaiStore", () => ({
    globalStore: {
        get: vi.fn((atom) => {
            // Return default values for atoms
            if (typeof atom === "object" && atom !== null && "init" in atom) {
                return atom.init;
            }
            return undefined;
        }),
        set: vi.fn(),
        sub: vi.fn(() => () => {}),
    },
}));

import { RpcApi } from "@/app/store/wshclientapi";
import { globalStore } from "@/app/store/jotaiStore";

describe("DashboardViewModel", () => {
    let model: DashboardViewModel;

    beforeEach(() => {
        vi.clearAllMocks();
        // Get a singleton instance for testing
        model = DashboardViewModel.getInstance("test-block-id");
        // Clear the singleton between tests
        (DashboardViewModel as any).instance = null;
    });

    describe("getInstance", () => {
        it("should return singleton instance", () => {
            const instance1 = DashboardViewModel.getInstance("block1");
            const instance2 = DashboardViewModel.getInstance("block1");
            expect(instance1).toBe(instance2);
        });

        it("should create new instance if none exists", () => {
            (DashboardViewModel as any).instance = null;
            const instance = DashboardViewModel.getInstance("block1");
            expect(instance).toBeInstanceOf(DashboardViewModel);
        });
    });

    describe("deriveWorkersFromTasks", () => {
        it("should return workers from in-progress and pending tasks", () => {
            const tasks = [
                {
                    taskId: "task1",
                    description: "Task 1",
                    status: "in_progress",
                    assignedAgentId: "agent1",
                    createdAt: Date.now(),
                },
                {
                    taskId: "task2",
                    description: "Task 2",
                    status: "pending",
                    assignedAgentId: "agent2",
                    createdAt: Date.now() - 1000,
                },
                {
                    taskId: "task3",
                    description: "Task 3",
                    status: "completed",
                    assignedAgentId: "agent3",
                    createdAt: Date.now() - 2000,
                },
            ];

            const workers = (model as any).deriveWorkersFromTasks(tasks);
            expect(workers).toHaveLength(2);
            expect(workers[0].taskId).toBe("task1");
            expect(workers[1].taskId).toBe("task2");
        });

        it("should return empty array for no tasks", () => {
            const workers = (model as any).deriveWorkersFromTasks([]);
            expect(workers).toEqual([]);
        });

        it("should return empty array for completed tasks only", () => {
            const tasks = [
                {
                    taskId: "task1",
                    description: "Task 1",
                    status: "completed",
                    assignedAgentId: "agent1",
                    createdAt: Date.now(),
                },
                {
                    taskId: "task2",
                    description: "Task 2",
                    status: "failed",
                    assignedAgentId: "agent2",
                    createdAt: Date.now() - 1000,
                },
            ];

            const workers = (model as any).deriveWorkersFromTasks(tasks);
            expect(workers).toEqual([]);
        });
    });

    describe("addTask", () => {
        it("should add a task and refresh task list", async () => {
            const mockTaskId = "new-task-id";
            (RpcApi.AssistantAddTaskCommand as any).mockResolvedValue({
                taskId: mockTaskId,
                status: "pending",
            });

            (RpcApi.AssistantListTasksCommand as any).mockResolvedValue({
                tasks: [],
            });

            const taskId = await model.addTask("Test task");
            expect(taskId).toBe(mockTaskId);
            expect(RpcApi.AssistantAddTaskCommand).toHaveBeenCalledWith(
                expect.anything(),
                { description: "Test task" }
            );
            expect(RpcApi.AssistantListTasksCommand).toHaveBeenCalled();
        });

        it("should throw error for empty description", async () => {
            await expect(model.addTask("")).rejects.toThrow("Task description cannot be empty");
        });

        it("should throw error for whitespace-only description", async () => {
            await expect(model.addTask("   ")).rejects.toThrow("Task description cannot be empty");
        });
    });

    describe("startAutoRefresh", () => {
        it("should start auto-refresh interval", () => {
            const setIntervalSpy = vi.spyOn(window, "setInterval");
            (globalStore.get as any).mockReturnValue(true);

            model.startAutoRefresh();
            expect(setIntervalSpy).toHaveBeenCalledWith(
                expect.any(Function),
                5000
            );
            setIntervalSpy.mockRestore();
        });

        it("should not start if already running", () => {
            const setIntervalSpy = vi.spyOn(window, "setInterval");
            (globalStore.get as any).mockReturnValue(true);

            model.startAutoRefresh();
            const firstCallCount = setIntervalSpy.mock.calls.length;
            model.startAutoRefresh();
            const secondCallCount = setIntervalSpy.mock.calls.length;
            expect(secondCallCount).toBe(firstCallCount);
            setIntervalSpy.mockRestore();
        });
    });

    describe("stopAutoRefresh", () => {
        it("should stop auto-refresh interval", () => {
            const clearIntervalSpy = vi.spyOn(window, "clearInterval");
            (globalStore.get as any).mockReturnValue(true);

            model.startAutoRefresh();
            model.stopAutoRefresh();
            expect(clearIntervalSpy).toHaveBeenCalled();
            clearIntervalSpy.mockRestore();
        });
    });

    describe("toggleAutoRefresh", () => {
        it("should toggle enabled state", () => {
            (globalStore.get as any).mockReturnValueOnce(true);
            model.toggleAutoRefresh();
            expect(globalStore.set).toHaveBeenCalled();
        });
    });

    describe("startAssistant", () => {
        it("should call assistant start RPC", async () => {
            (RpcApi.AssistantStartCommand as any).mockResolvedValue({ running: true });
            (RpcApi.AssistantStatusCommand as any).mockResolvedValue({
                running: true,
                taskCount: 0,
            });

            await model.startAssistant();
            expect(RpcApi.AssistantStartCommand).toHaveBeenCalled();
            expect(RpcApi.AssistantStatusCommand).toHaveBeenCalled();
        });

        it("should throw error on RPC failure", async () => {
            const error = new Error("RPC error");
            (RpcApi.AssistantStartCommand as any).mockRejectedValue(error);

            await expect(model.startAssistant()).rejects.toThrow("RPC error");
        });
    });

    describe("stopAssistant", () => {
        it("should call assistant stop RPC", async () => {
            (RpcApi.AssistantStopCommand as any).mockResolvedValue(undefined);
            (RpcApi.AssistantStatusCommand as any).mockResolvedValue({
                running: false,
                taskCount: 0,
            });

            await model.stopAssistant();
            expect(RpcApi.AssistantStopCommand).toHaveBeenCalled();
            expect(RpcApi.AssistantStatusCommand).toHaveBeenCalled();
        });

        it("should throw error on RPC failure", async () => {
            const error = new Error("RPC error");
            (RpcApi.AssistantStopCommand as any).mockRejectedValue(error);

            await expect(model.stopAssistant()).rejects.toThrow("RPC error");
        });
    });

    describe("dispose", () => {
        it("should stop auto-refresh", () => {
            const clearIntervalSpy = vi.spyOn(window, "clearInterval");
            (globalStore.get as any).mockReturnValue(true);
            model.startAutoRefresh();

            model.dispose();
            expect(clearIntervalSpy).toHaveBeenCalled();
            clearIntervalSpy.mockRestore();
        });
    });
});
