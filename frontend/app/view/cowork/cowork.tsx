// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import * as jotai from "jotai";
import * as React from "react";
import { CoworkViewModel } from "./cowork-model";
import { RuntimeDetectionPanel } from "./runtime-detection-panel";
import { WorkerConfigDialog } from "./worker-config-dialog";

interface CoworkViewProps {
    blockId: string;
    blockRef: React.RefObject<HTMLDivElement>;
    contentRef: React.RefObject<HTMLDivElement>;
    model: CoworkViewModel;
}

export function CoworkView({ model }: CoworkViewProps) {
    const pendingTasks = jotai.useAtomValue(model.pendingTasksAtom) ?? [];
    const workingTasks = jotai.useAtomValue(model.workingTasksAtom) ?? [];
    const doneTasks = jotai.useAtomValue(model.doneTasksAtom) ?? [];
    const failedTasks = jotai.useAtomValue(model.failedTasksAtom) ?? [];
    const workers = jotai.useAtomValue(model.workersAtom) ?? [];
    const activities = jotai.useAtomValue(model.activityLogAtom) ?? [];
    const isSupervising = jotai.useAtomValue(model.isSupervisingAtom) ?? false;
    const isProcessing = jotai.useAtomValue(model.isProcessingAtom) ?? false;
    const lastLLMCall = jotai.useAtomValue(model.lastLLMCallAtom) ?? "";
    const error = jotai.useAtomValue(model.errorAtom) ?? null;
    const status = jotai.useAtomValue(model.statusAtom) ?? {
        pendingtasks: 0,
        workingtasks: 0,
        donetasks: 0,
        failedtasks: 0,
        activeworkers: 0,
        idleworkers: 0,
    };

    const [newTaskTitle, setNewTaskTitle] = React.useState("");
    const [newTaskDesc, setNewTaskDesc] = React.useState("");
    const [newTaskPriority, setNewTaskPriority] = React.useState("medium");
    const [showWorkerConfig, setShowWorkerConfig] = React.useState(false);

    const handleCreateTask = async () => {
        if (!newTaskTitle.trim()) {
            return;
        }
        await model.createTask(newTaskTitle, newTaskDesc, newTaskPriority);
        setNewTaskTitle("");
        setNewTaskDesc("");
        setNewTaskPriority("medium");
    };

    const handleDeleteTask = async (taskId: string) => {
        await model.deleteTask(taskId);
    };

    const handleDeleteWorker = async (workerId: string) => {
        await model.deleteWorker(workerId);
    };

    const handleAssignTask = async (taskId: string, workerId: string) => {
        await model.assignTask(taskId, workerId);
    };

    const handlePauseTask = async (taskId: string) => {
        await model.pauseTask(taskId);
    };

    const handleResumeTask = async (taskId: string) => {
        await model.resumeTask(taskId);
    };

    const handleRefresh = async () => {
        await model.refreshAllData();
    };

    const toggleSupervision = () => {
        if (isSupervising) {
            model.stopSupervision();
        } else {
            model.startSupervision();
        }
    };

    const statusColors: Record<string, string> = {
        working: "bg-green-500",
        idle: "bg-gray-400",
        offline: "bg-gray-300",
        error: "bg-red-500",
    };

    const priorityColors: Record<string, string> = {
        low: "text-gray-400",
        medium: "text-blue-400",
        high: "text-orange-400",
        urgent: "text-red-500",
    };

    const formatTime = (ts: number) => {
        if (!ts) {
            return "";
        }
        return new Date(ts * 1000).toLocaleTimeString();
    };

    return (
        <div className="flex flex-col h-full p-3 gap-3 overflow-auto">
            <div className="flex items-center gap-2">
                <button
                    className={`px-3 py-1 rounded text-sm transition-colors cursor-pointer ${
                        isSupervising
                            ? "bg-green-600 text-white hover:bg-green-700"
                            : "bg-accent/80 text-primary hover:bg-accent"
                    }`}
                    onClick={toggleSupervision}
                    disabled={isProcessing}
                >
                    {isSupervising ? "👑 Supervising" : "👑 Start Supervise"}
                </button>
                <button
                    className="px-3 py-1 rounded bg-base/50 text-primary text-sm hover:bg-base/70 transition-colors cursor-pointer"
                    onClick={handleRefresh}
                >
                    Refresh
                </button>
                {isProcessing && <span className="text-sm text-gray-400">Processing...</span>}
                {lastLLMCall && (
                    <span className="text-xs text-gray-500">
                        Last LLM: {new Date(lastLLMCall).toLocaleTimeString()}
                    </span>
                )}
                {error && <span className="text-xs text-red-400">Error: {error}</span>}
            </div>

            <div className="flex gap-4 text-sm">
                <span>Pending: {status.pendingtasks}</span>
                <span>Working: {status.workingtasks}</span>
                <span>Done: {status.donetasks}</span>
                <span>Failed: {status.failedtasks}</span>
                <span>Active Workers: {status.activeworkers}</span>
                <span>Idle Workers: {status.idleworkers}</span>
            </div>

            <div className="rounded border border-border/50 bg-base p-3">
                <h3 className="text-sm font-semibold mb-2">New Task</h3>
                <div className="flex gap-2">
                    <input
                        className="flex-1 bg-base/50 border border-border/50 rounded px-2 py-1 text-sm"
                        placeholder="Task title"
                        value={newTaskTitle}
                        onChange={(e) => setNewTaskTitle(e.target.value)}
                    />
                    <select
                        className="bg-base/50 border border-border/50 rounded px-2 py-1 text-sm"
                        value={newTaskPriority}
                        onChange={(e) => setNewTaskPriority(e.target.value)}
                    >
                        <option value="low">Low</option>
                        <option value="medium">Medium</option>
                        <option value="high">High</option>
                        <option value="urgent">Urgent</option>
                    </select>
                    <button
                        className="px-3 py-1 rounded bg-accent/80 text-primary hover:bg-accent transition-colors cursor-pointer text-sm"
                        onClick={handleCreateTask}
                    >
                        Create
                    </button>
                </div>
                <input
                    className="w-full mt-2 bg-base/50 border border-border/50 rounded px-2 py-1 text-sm"
                    placeholder="Description (optional)"
                    value={newTaskDesc}
                    onChange={(e) => setNewTaskDesc(e.target.value)}
                />
            </div>

            <div>
                <h3 className="text-sm font-semibold mb-2">Task Board</h3>
                <div className="grid grid-cols-4 gap-2">
                    <TaskColumn
                        title="Pending"
                        tasks={pendingTasks}
                        onDelete={handleDeleteTask}
                        priorityColors={priorityColors}
                        workers={workers}
                        onAssignTask={handleAssignTask}
                        onPauseTask={handlePauseTask}
                        onResumeTask={handleResumeTask}
                    />
                    <TaskColumn
                        title="Working"
                        tasks={workingTasks}
                        onDelete={handleDeleteTask}
                        priorityColors={priorityColors}
                        workers={workers}
                        onAssignTask={handleAssignTask}
                        onPauseTask={handlePauseTask}
                        onResumeTask={handleResumeTask}
                    />
                    <TaskColumn
                        title="Done"
                        tasks={doneTasks}
                        onDelete={handleDeleteTask}
                        priorityColors={priorityColors}
                        workers={workers}
                        onAssignTask={handleAssignTask}
                        onPauseTask={handlePauseTask}
                        onResumeTask={handleResumeTask}
                    />
                    <TaskColumn
                        title="Failed"
                        tasks={failedTasks}
                        onDelete={handleDeleteTask}
                        priorityColors={priorityColors}
                        workers={workers}
                        onAssignTask={handleAssignTask}
                        onPauseTask={handlePauseTask}
                        onResumeTask={handleResumeTask}
                    />
                </div>
            </div>

            <RuntimeDetectionPanel className="rounded border border-border/50 bg-base" />

            <div>
                <div className="flex items-center justify-between mb-2">
                    <h3 className="text-sm font-semibold">Workers</h3>
                    <button
                        className="text-xs text-accent hover:text-accent/80 cursor-pointer"
                        onClick={() => setShowWorkerConfig(true)}
                    >
                        Config
                    </button>
                </div>
                <div className="rounded border border-border/50 bg-base p-3">
                    {workers.length === 0 ? (
                        <span className="text-sm text-gray-500">No workers registered</span>
                    ) : (
                        <div className="flex flex-col gap-2">
                            {workers.map((w) => (
                                <div key={w.workerid} className="flex items-center gap-2 text-sm">
                                    <span
                                        className={`inline-block w-2 h-2 rounded-full ${statusColors[w.status] ?? "bg-gray-300"}`}
                                    />
                                    <span className="flex-1">{w.name}</span>
                                    <span className="text-gray-400 text-xs">{w.tool}</span>
                                    <span className="text-gray-400 text-xs">{w.status}</span>
                                    {w.assignedtask && (
                                        <span className="text-xs text-blue-400">→ {w.assignedtask}</span>
                                    )}
                                    <button
                                        className="text-xs text-red-400 hover:text-red-300 cursor-pointer"
                                        onClick={() => handleDeleteWorker(w.workerid)}
                                    >
                                        Remove
                                    </button>
                                </div>
                            ))}
                        </div>
                    )}
                </div>
            </div>

            <div>
                <h3 className="text-sm font-semibold mb-2">Activity Log</h3>
                <div className="rounded border border-border/50 bg-base p-3 max-h-48 overflow-auto">
                    {activities.length === 0 ? (
                        <span className="text-sm text-gray-500">No activity yet</span>
                    ) : (
                        <div className="flex flex-col gap-1">
                            {activities.map((a) => (
                                <div key={a.id} className="text-xs">
                                    <span className="text-gray-500">{formatTime(a.createdat)}</span>{" "}
                                    <span className="text-gray-400">[{a.type}]</span> <span>{a.description}</span>
                                </div>
                            ))}
                        </div>
                    )}
                </div>
            </div>

            {showWorkerConfig && (
                <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
                    <WorkerConfigDialog
                        className="bg-base rounded-lg shadow-xl max-w-md w-full"
                        onWorkerCreated={(workerId) => {
                            setShowWorkerConfig(false);
                        }}
                        onCancel={() => setShowWorkerConfig(false)}
                    />
                </div>
            )}
        </div>
    );
}

function TaskColumn({
    title,
    tasks,
    onDelete,
    priorityColors,
    workers,
    onAssignTask,
    onPauseTask,
    onResumeTask,
}: {
    title: string;
    tasks: CoworkTask[];
    onDelete: (id: string) => void;
    priorityColors: Record<string, string>;
    workers: CoworkWorker[];
    onAssignTask: (taskId: string, workerId: string) => void;
    onPauseTask: (taskId: string) => void;
    onResumeTask: (taskId: string) => void;
}) {
    return (
        <div className="rounded border border-border/50 bg-base p-2">
            <h4 className="text-xs font-semibold mb-2">
                {title} ({tasks.length})
            </h4>
            <div className="flex flex-col gap-1">
                {tasks.length === 0 ? (
                    <span className="text-xs text-gray-500">None</span>
                ) : (
                    tasks.map((t) => (
                        <div key={t.taskid} className="text-xs p-1 rounded bg-base/30">
                            <div className="flex items-center gap-1">
                                <span className={priorityColors[t.priority] ?? ""}>{t.title}</span>
                                <span className="text-gray-400">[{t.status}]</span>
                                {(t.status === "working" || t.status === "pending") && (
                                    <button
                                        className="text-xs text-blue-400 hover:text-blue-300 cursor-pointer"
                                        onClick={() => onPauseTask(t.taskid)}
                                    >
                                        ⏸
                                    </button>
                                )}
                                {t.status === "paused" && (
                                    <button
                                        className="text-xs text-green-400 hover:text-green-300 cursor-pointer"
                                        onClick={() => onResumeTask(t.taskid)}
                                    >
                                        ▶
                                    </button>
                                )}
                                {workers.length > 0 && (
                                    <select
                                        value={t.assignedworker || ""}
                                        onChange={(e) => e.target.value && onAssignTask(t.taskid, e.target.value)}
                                        className="ml-auto text-xs bg-white border border-gray-300 rounded px-1 py-0.5"
                                    >
                                        <option value="">Unassigned</option>
                                        {workers.map((w) => (
                                            <option key={w.workerid} value={w.workerid}>
                                               	{w.name}
                                            </option>
                                        ))}
                                    </select>
                                )}
                                <button
                                    className="text-gray-500 hover:text-red-400 cursor-pointer"
                                    onClick={() => onDelete(t.taskid)}
                                >
                                    ×
                                </button>
                            </div>
                            {t.assignedworker && <div className="text-gray-500">→ {t.assignedworker}</div>}
                            {t.progress && <div className="text-gray-400">{t.progress}</div>}
                        </div>
                    ))
                )}
            </div>
        </div>
    );
}
