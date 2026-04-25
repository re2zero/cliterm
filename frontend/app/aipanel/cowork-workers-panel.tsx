// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";
import { useEffect, useState } from "react";

export function CoworkWorkersPanel() {
    const [isExpanded, setIsExpanded] = useState(false);
    const [workers, setWorkers] = useState<CoworkWorker[]>([]);
    const [tasks, setTasks] = useState<CoworkTask[]>([]);
    const [isLoading, setIsLoading] = useState(false);

    const loadData = async () => {
        setIsLoading(true);
        try {
            const [workersData, tasksData] = await Promise.all([
                RpcApi.CoworkListWorkersCommand(TabRpcClient),
                RpcApi.CoworkListTasksCommand(TabRpcClient, {}),
            ]);
            setWorkers(workersData);
            setTasks(tasksData);
        } catch (e) {
            console.error("Failed to load cowork data:", e);
        } finally {
            setIsLoading(false);
        }
    };

    useEffect(() => {
        if (isExpanded) {
            loadData();
        }
    }, [isExpanded]);

    const safeWorkers = workers ?? [];
    const safeTasks = tasks ?? [];
    const pendingTasks = safeTasks.filter((t) => t.status === "pending" || t.status === "assigned");
    const workingTasks = safeTasks.filter((t) => t.status === "working");
    const activeWorkers = safeWorkers.filter((w) => w.status === "working");
    const idleWorkers = safeWorkers.filter((w) => w.status === "idle");

    const statusColors: Record<string, string> = {
        working: "bg-green-500",
        idle: "bg-gray-400",
        offline: "bg-gray-300",
        error: "bg-red-500",
    };

    return (
        <div className="border-t border-gray-700">
            <button
                onClick={() => setIsExpanded(!isExpanded)}
                className="w-full px-3 py-2 flex items-center justify-between text-sm text-gray-300 hover:bg-gray-800 transition-colors"
            >
                <div className="flex items-center gap-2">
                    <span>👥</span>
                    <span className="font-medium">Cowork Workers</span>
                    <span className="text-xs text-gray-500">
                        ({activeWorkers.length} active, {idleWorkers.length} idle, {pendingTasks.length} pending)
                    </span>
                </div>
                <span className="text-xs">{isExpanded ? "▼" : "▶"}</span>
            </button>

            {isExpanded && (
                <div className="px-3 pb-3">
                    {isLoading ? (
                        <div className="text-xs text-gray-500 py-2">Loading...</div>
                    ) : safeWorkers.length === 0 && safeTasks.length === 0 ? (
                        <div className="text-xs text-gray-500 py-2">
                            No workers or tasks. Use AI to create and manage workers.
                        </div>
                    ) : (
                        <div className="space-y-3">
                            {safeWorkers.length > 0 && (
                                <div>
                                    <div className="text-xs text-gray-500 mb-1">Workers</div>
                                    <div className="flex flex-wrap gap-2">
                                        {safeWorkers.map((w) => (
                                            <div
                                                key={w.workerid}
                                                className="flex flex-col gap-1 px-2 py-1.5 bg-gray-800 rounded text-xs"
                                            >
                                                <div className="flex items-center gap-1.5">
                                                    <span
                                                        className={`w-1.5 h-1.5 rounded-full ${statusColors[w.status] ?? "bg-gray-400"}`}
                                                    />
                                                    <span className="text-gray-300 font-medium">{w.name}</span>
                                                    <span className="text-gray-500">({w.tool})</span>
                                                    {w.role && <span className="text-purple-400">[{w.role}]</span>}
                                                </div>
                                                {w.desc && (
                                                    <div className="text-gray-500 text-[10px] pl-3">{w.desc}</div>
                                                )}
                                                {w.assignedtask && (
                                                    <div className="text-blue-400 pl-3">
                                                        → {w.assignedtask.substring(0, 15)}...
                                                    </div>
                                                )}
                                            </div>
                                        ))}
                                    </div>
                                </div>
                            )}

                            {safeTasks.length > 0 && (
                                <div>
                                    <div className="text-xs text-gray-500 mb-1">Tasks</div>
                                    <div className="flex flex-wrap gap-2">
                                        {safeTasks.map((t) => (
                                            <div
                                                key={t.taskid}
                                                className={`px-2 py-1 rounded text-xs ${
                                                    t.status === "working"
                                                        ? "bg-green-900/50 text-green-300"
                                                        : t.status === "done"
                                                          ? "bg-blue-900/50 text-blue-300"
                                                          : t.status === "failed"
                                                            ? "bg-red-900/50 text-red-300"
                                                            : "bg-gray-800 text-gray-300"
                                                }`}
                                            >
                                                {(t.title ?? "").substring(0, 20)}...
                                                <span className="ml-1 text-gray-500">[{t.status}]</span>
                                            </div>
                                        ))}
                                    </div>
                                </div>
                            )}

                            <button
                                onClick={loadData}
                                className="text-xs text-gray-500 hover:text-gray-300 transition-colors"
                            >
                                ↻ Refresh
                            </button>
                        </div>
                    )}
                </div>
            )}
        </div>
    );
}
