// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import clsx from "clsx";
import dayjs from "dayjs";
import * as jotai from "jotai";
import * as React from "react";

import { Button } from "@/app/element/button";
import { DashboardViewModel, type AssistantStatus, type WorkerInfo } from "./dashboard-model";

import "./dashboard.scss";

type DashboardViewProps = {
    model: DashboardViewModel;
};

function DashboardStatusSection({ model }: DashboardViewProps) {
    const status = jotai.useAtomValue(model.statusAtom);
    const statusLoading = jotai.useAtomValue(model.statusLoadingAtom);
    const statusError = jotai.useAtomValue(model.statusErrorAtom);
    const lastRefresh = jotai.useAtomValue(model.lastRefreshAtom);

    const handleStartAssistant = React.useCallback(async () => {
        try {
            await model.startAssistant();
        } catch (error) {
            console.error("[dashboard] Failed to start assistant:", error);
        }
    }, [model]);

    const handleStopAssistant = React.useCallback(async () => {
        try {
            await model.stopAssistant();
        } catch (error) {
            console.error("[dashboard] Failed to stop assistant:", error);
        }
    }, [model]);

    if (statusLoading) {
        return (
            <div className="dashboard-section p-4 border-b border-border-color">
                <div className="flex items-center gap-2 text-sm text-secondary-text-color">
                    <span>Loading...</span>
                </div>
            </div>
        );
    }

    if (statusError) {
        return (
            <div className="dashboard-section p-4 border-b border-border-color">
                <div className="flex items-center gap-2 text-sm text-error-color">
                    <i className="fa-solid fa-circle-exclamation" />
                    <span>Error loading status: {statusError}</span>
                </div>
            </div>
        );
    }

    return (
        <div className="dashboard-section p-4 border-b border-border-color">
            <div className="flex items-center justify-between mb-4">
                <h2 className="text-base font-semibold text-main-text-color">Assistant Status</h2>
                <div className="flex items-center gap-2">
                    {status.running ? (
                        <Button className="solid red text-sm" onClick={handleStopAssistant}>
                            <i className="fa-solid fa-stop" />
                            <span>Stop</span>
                        </Button>
                    ) : (
                        <Button className="solid green text-sm" onClick={handleStartAssistant}>
                            <i className="fa-solid fa-play" />
                            <span>Start</span>
                        </Button>
                    )}
                </div>
            </div>
            <div className="grid grid-cols-3 gap-4 text-sm">
                <div className="flex flex-col gap-1">
                    <span className="text-secondary-text-color">State</span>
                    <span
                        className={clsx("font-semibold", {
                            "text-success-color": status.running,
                            "text-secondary-text-color": !status.running,
                        })}
                    >
                        {status.running ? "Running" : "Stopped"}
                    </span>
                </div>
                <div className="flex flex-col gap-1">
                    <span className="text-secondary-text-color">Task Count</span>
                    <span className="font-semibold text-main-text-color">{status.taskCount}</span>
                </div>
                <div className="flex flex-col gap-1">
                    <span className="text-secondary-text-color">Last Updated</span>
                    <span className="font-semibold text-main-text-color">
                        {dayjs(lastRefresh).format("HH:mm:ss")}
                    </span>
                </div>
            </div>
        </div>
    );
}

function DashboardTaskQueueSection({ model }: DashboardViewProps) {
    const tasks = jotai.useAtomValue(model.tasksAtom);
    const tasksLoading = jotai.useAtomValue(model.tasksLoadingAtom);
    const tasksError = jotai.useAtomValue(model.tasksErrorAtom);

    if (tasksLoading) {
        return (
            <div className="dashboard-section p-4 border-b border-border-color">
                <div className="flex items-center gap-2 text-sm text-secondary-text-color">
                    <span>Loading tasks...</span>
                </div>
            </div>
        );
    }

    if (tasksError) {
        return (
            <div className="dashboard-section p-4 border-b border-border-color">
                <div className="flex items-center gap-2 text-sm text-error-color">
                    <i className="fa-solid fa-circle-exclamation" />
                    <span>Error loading tasks: {tasksError}</span>
                </div>
            </div>
        );
    }

    if (tasks.length === 0) {
        return (
            <div className="dashboard-section p-4 border-b border-border-color">
                <h2 className="text-base font-semibold text-main-text-color mb-4">Task Queue</h2>
                <div className="text-sm text-secondary-text-color">No tasks in queue</div>
            </div>
        );
    }

    // Count tasks by status
    const statusCounts = tasks.reduce(
        (acc, task) => {
            acc[task.status] = (acc[task.status] || 0) + 1;
            return acc;
        },
        {} as Record<string, number>
    );

    // Get unique statuses
    const statuses = Object.keys(statusCounts).sort();

    return (
        <div className="dashboard-section p-4 border-b border-border-color">
            <div className="flex items-center justify-between mb-4">
                <h2 className="text-base font-semibold text-main-text-color">Task Queue</h2>
                <span className="text-sm text-secondary-text-color">{tasks.length} total</span>
            </div>
            <div className="flex items-center gap-4 mb-4 text-sm">
                {statuses.map((status) => (
                    <div key={status} className="flex items-center gap-2">
                        <span className="text-secondary-text-color capitalize">{status}:</span>
                        <span className="font-semibold text-main-text-color">{statusCounts[status]}</span>
                    </div>
                ))}
            </div>
            <div className="flex flex-col gap-2">
                {tasks.map((task) => (
                    <div
                        key={task.taskId}
                        className="flex items-center justify-between p-3 bg-secondary-bg rounded hover:bg-secondary-bg-hover transition-colors"
                    >
                        <div className="flex flex-col gap-1 flex-grow min-w-0">
                            <div className="flex items-center gap-2">
                                <span
                                    className={clsx("text-xs font-semibold px-2 py-0.5 rounded capitalize", {
                                        "bg-success-color/20 text-success-color": task.status === "completed",
                                        "bg-warning-color/20 text-warning-color":
                                            task.status === "in_progress" || task.status === "pending",
                                        "bg-error-color/20 text-error-color": task.status === "failed",
                                    })}
                                >
                                    {task.status}
                                </span>
                                <span className="text-sm text-main-text-color truncate">
                                    {task.description}
                                </span>
                            </div>
                            <div className="flex items-center gap-4 text-xs text-secondary-text-color">
                                <span>ID: {task.taskId}</span>
                                <span>Created: {dayjs(task.createdAt).format("MMM D, HH:mm:ss")}</span>
                                {task.assignedAgentId && <span>Agent: {task.assignedAgentId}</span>}
                            </div>
                        </div>
                    </div>
                ))}
            </div>
        </div>
    );
}

function DashboardWorkerStatusSection({ model }: DashboardViewProps) {
    const workers = jotai.useAtomValue(model.workersAtom);
    const workersLoading = jotai.useAtomValue(model.workersLoadingAtom);

    if (workersLoading) {
        return (
            <div className="dashboard-section p-4 border-b border-border-color">
                <div className="flex items-center gap-2 text-sm text-secondary-text-color">
                    <span>Loading workers...</span>
                </div>
            </div>
        );
    }

    if (workers.length === 0) {
        return (
            <div className="dashboard-section p-4 border-b border-border-color">
                <h2 className="text-base font-semibold text-main-text-color mb-4">Worker Status</h2>
                <div className="text-sm text-secondary-text-color">No active workers</div>
            </div>
        );
    }

    return (
        <div className="dashboard-section p-4 border-b border-border-color">
            <div className="flex items-center justify-between mb-4">
                <h2 className="text-base font-semibold text-main-text-color">Worker Status</h2>
                <span className="text-sm text-secondary-text-color">{workers.length} active</span>
            </div>
            <div className="grid grid-cols-1 gap-2">
                {workers.map((worker, idx) => (
                    <div
                        key={`${worker.taskId}-${idx}`}
                        className="flex items-center justify-between p-3 bg-secondary-bg rounded"
                    >
                        <div className="flex flex-col gap-1 flex-grow min-w-0">
                            <div className="flex items-center gap-2">
                                <span className="text-sm font-semibold text-main-text-color">
                                    Worker {idx + 1}
                                </span>
                                <span
                                    className={clsx("text-xs font-semibold px-2 py-0.5 rounded capitalize", {
                                        "bg-warning-color/20 text-warning-color":
                                            worker.status === "in_progress",
                                    })}
                                >
                                    {worker.status}
                                </span>
                            </div>
                            <div className="flex items-center gap-4 text-xs text-secondary-text-color">
                                <span>Task: {worker.taskDescription}</span>
                                <span>Created: {dayjs(worker.createdAt).format("MMM D, HH:mm:ss")}</span>
                            </div>
                        </div>
                        {worker.blockId && (
                            <Button className="outline grey text-xs">
                                <span>Block</span>
                            </Button>
                        )}
                    </div>
                ))}
            </div>
        </div>
    );
}

function DashboardRefreshSection({ model }: DashboardViewProps) {
    const refresh = jotai.useSetAtom(model.refreshAtom);
    const autoRefreshEnabled = jotai.useAtomValue(model.autoRefreshEnabledAtom);
    const setAutoRefreshEnabled = jotai.useSetAtom(model.autoRefreshEnabledAtom);
    const lastRefresh = jotai.useAtomValue(model.lastRefreshAtom);
    const [isRefreshing, setIsRefreshing] = React.useState(false);

    const handleRefresh = React.useCallback(async () => {
        if (isRefreshing) return;
        setIsRefreshing(true);
        try {
            await refresh();
        } finally {
            setIsRefreshing(false);
        }
    }, [refresh, isRefreshing]);

    const handleToggleAutoRefresh = React.useCallback(() => {
        model.toggleAutoRefresh();
    }, [model]);

    return (
        <div className="dashboard-section p-4">
            <div className="flex items-center justify-between">
                <div className="flex items-center gap-4">
                    <span className="text-sm text-secondary-text-color">
                        Last refresh: {dayjs(lastRefresh).format("HH:mm:ss")}
                    </span>
                    <Button
                        className={clsx("solid", "grey", "text-sm", {
                            "opacity-50": isRefreshing,
                        })}
                        onClick={handleRefresh}
                        disabled={isRefreshing}
                    >
                        <i className={clsx("fa-solid", isRefreshing ? "fa-spinner fa-spin" : "fa-rotate")} />
                        <span>{isRefreshing ? "Refreshing..." : "Refresh"}</span>
                    </Button>
                </div>
                <div className="flex items-center gap-2">
                    <span className="text-sm text-secondary-text-color">Auto-refresh (5s)</span>
                    <div
                        className={clsx("w-10 h-5 rounded-full relative cursor-pointer transition-colors", {
                            "bg-accent-color": autoRefreshEnabled,
                            "bg-button-grey-bg": !autoRefreshEnabled,
                        })}
                        onClick={handleToggleAutoRefresh}
                    >
                        <div
                            className={clsx(
                                "w-4 h-4 rounded-full absolute top-0.5 transition-all transform",
                                {
                                    "left-auto right-0.5": autoRefreshEnabled,
                                    "left-0.5": !autoRefreshEnabled,
                                }
                            )}
                        />
                    </div>
                </div>
            </div>
        </div>
    );
}

function DashboardView({ model }: DashboardViewProps) {
    React.useEffect(() => {
        // Start auto-refresh when component mounts
        model.startAutoRefresh();

        // Cleanup on unmount
        return () => {
            model.dispose();
        };
    }, [model]);

    return (
        <div className="w-full h-full flex flex-col bg-main-bg overflow-hidden">
            <div className="flex flex-col h-full overflow-hidden">
                <div className="flex-grow overflow-y-auto">
                    <DashboardStatusSection model={model} />
                    <DashboardTaskQueueSection model={model} />
                    <DashboardWorkerStatusSection model={model} />
                </div>
                <DashboardRefreshSection model={model} />
            </div>
        </div>
    );
}

export { DashboardView };
