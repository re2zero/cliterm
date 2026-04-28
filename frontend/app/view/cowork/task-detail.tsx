// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import * as React from "react";
import { cn } from "@/util/util";
import { PriorityBadge } from "./priority-badge";
import { AssigneePicker } from "./assignee-picker";

interface TaskDetailProps {
    task: CoworkTask;
    workers: CoworkWorker[];
    allTasks: CoworkTask[];
    outputHistory: string[];
    activities: CoworkActivity[];
    onClose: () => void;
    onUpdate: (taskId: string, updates: Record<string, any>) => void;
    onExecute: (taskId: string, command: string) => void;
    onPause: (taskId: string) => void;
    onResume: (taskId: string) => void;
    onRetry: (taskId: string) => void;
    onDelete: (taskId: string) => void;
}

const STATUS_OPTIONS = ["pending", "assigned", "working", "paused", "done", "failed"];
const PRIORITY_OPTIONS = ["low", "medium", "high", "urgent"];

export function TaskDetail({
    task,
    workers,
    allTasks,
    outputHistory,
    activities,
    onClose,
    onUpdate,
    onExecute,
    onPause,
    onResume,
    onRetry,
    onDelete,
}: TaskDetailProps) {
    const [confirmDelete, setConfirmDelete] = React.useState(false);
    const [executeCmd, setExecuteCmd] = React.useState("");
    const [showExecute, setShowExecute] = React.useState(false);

    const assignedWorker = task.assignedworker
        ? workers.find((w) => w.workerid === task.assignedworker)
        : null;

    const taskActivities = activities.filter((a) => a.taskid === task.taskid);
    const deps = task.dependson?.map((depId) => allTasks.find((t) => t.taskid === depId)).filter(Boolean) ?? [];

    const selectCls = "bg-base border border-border/50 rounded text-xs text-primary px-2 py-1 cursor-pointer focus:outline-none focus:ring-1 focus:ring-accent";

    return (
        <div className="flex flex-col h-full border-l border-border/50 bg-card overflow-y-auto" style={{ width: 380, minWidth: 380 }}>
            {/* Header */}
            <div className="flex items-center justify-between px-4 py-2.5 border-b border-border/50 shrink-0">
                <button className="text-xs text-muted-foreground hover:text-primary cursor-pointer" onClick={onClose}>← Back</button>
                <span className="text-xs text-muted-foreground">{task.taskid.slice(0, 8).toUpperCase()}</span>
            </div>

            <div className="flex-1 overflow-y-auto p-4 space-y-4">
                {/* Title */}
                <h3 className="text-sm font-semibold text-primary leading-snug">{task.title}</h3>

                {/* Properties */}
                <div className="space-y-2">
                    <div className="flex items-center gap-2">
                        <span className="text-[11px] text-muted-foreground w-16 shrink-0">Status</span>
                        <select className={selectCls} value={task.status} onChange={(e) => onUpdate(task.taskid, { status: e.target.value })}>
                            {STATUS_OPTIONS.map((s) => <option key={s} value={s}>{s}</option>)}
                        </select>
                    </div>
                    <div className="flex items-center gap-2">
                        <span className="text-[11px] text-muted-foreground w-16 shrink-0">Priority</span>
                        <select className={selectCls} value={task.priority} onChange={(e) => onUpdate(task.taskid, { priority: e.target.value })}>
                            {PRIORITY_OPTIONS.map((p) => <option key={p} value={p}>{p}</option>)}
                        </select>
                    </div>
                    <div className="flex items-center gap-2">
                        <span className="text-[11px] text-muted-foreground w-16 shrink-0">Worker</span>
                        <AssigneePicker
                            workers={workers}
                            selectedWorkerId={task.assignedworker}
                            onSelect={(workerId) => onUpdate(task.taskid, { assignedworker: workerId })}
                        />
                    </div>
                    <div className="flex items-center gap-2">
                        <span className="text-[11px] text-muted-foreground w-16 shrink-0">Created</span>
                        <span className="text-xs text-secondary">
                            {task.createdat ? new Date(task.createdat * 1000).toLocaleString() : "—"}
                        </span>
                    </div>
                    {deps.length > 0 && (
                        <div className="flex items-start gap-2">
                            <span className="text-[11px] text-muted-foreground w-16 shrink-0">Deps</span>
                            <div className="flex flex-wrap gap-1">
                                {deps.map((dep) => (
                                    <span key={dep.taskid} className={cn(
                                        "px-1.5 py-0.5 text-[10px] rounded",
                                        dep.status === "done" ? "bg-blue-500/10 text-blue-400" : "bg-muted text-muted-foreground",
                                    )}>
                                        {dep.title.slice(0, 20)}
                                    </span>
                                ))}
                            </div>
                        </div>
                    )}
                </div>

                {/* Description */}
                {task.description && (
                    <div>
                        <div className="text-[10px] text-muted-foreground uppercase tracking-wider mb-1">Description</div>
                        <p className="text-xs text-secondary leading-relaxed">{task.description}</p>
                    </div>
                )}

                {/* Actions */}
                <div>
                    <div className="text-[10px] text-muted-foreground uppercase tracking-wider mb-1.5">Actions</div>
                    <div className="flex flex-wrap gap-1.5">
                        {task.status !== "working" && (
                            <button
                                className="px-2 py-1 text-[11px] rounded bg-accent/80 text-primary hover:bg-accent cursor-pointer"
                                onClick={() => setShowExecute(!showExecute)}
                            >
                                ▶ Execute
                            </button>
                        )}
                        {task.status === "working" && (
                            <button
                                className="px-2 py-1 text-[11px] rounded bg-yellow-500/10 text-yellow-400 hover:bg-yellow-500/20 cursor-pointer"
                                onClick={() => onPause(task.taskid)}
                            >
                                ⏸ Pause
                            </button>
                        )}
                        {task.status === "paused" && (
                            <button
                                className="px-2 py-1 text-[11px] rounded bg-accent/80 text-primary hover:bg-accent cursor-pointer"
                                onClick={() => onResume(task.taskid)}
                            >
                                ▶ Resume
                            </button>
                        )}
                        {task.status === "failed" && (
                            <button
                                className="px-2 py-1 text-[11px] rounded bg-orange-500/10 text-orange-400 hover:bg-orange-500/20 cursor-pointer"
                                onClick={() => onRetry(task.taskid)}
                            >
                                ↻ Retry
                            </button>
                        )}
                    </div>
                    {showExecute && (
                        <div className="mt-2 flex gap-1.5">
                            <input
                                className="flex-1 bg-base border border-border/50 rounded text-xs text-primary px-2 py-1 focus:outline-none focus:ring-1 focus:ring-accent"
                                placeholder="Command to execute..."
                                value={executeCmd}
                                onChange={(e) => setExecuteCmd(e.target.value)}
                                onKeyDown={(e) => {
                                    if (e.key === "Enter" && executeCmd.trim()) {
                                        onExecute(task.taskid, executeCmd);
                                        setExecuteCmd("");
                                        setShowExecute(false);
                                    }
                                }}
                                autoFocus
                            />
                            <button
                                className="px-2 py-1 text-[11px] rounded bg-accent/80 text-primary cursor-pointer"
                                onClick={() => {
                                    if (executeCmd.trim()) {
                                        onExecute(task.taskid, executeCmd);
                                        setExecuteCmd("");
                                        setShowExecute(false);
                                    }
                                }}
                            >
                                Run
                            </button>
                        </div>
                    )}
                </div>

                {/* Output */}
                {outputHistory.length > 0 && (
                    <div>
                        <div className="text-[10px] text-muted-foreground uppercase tracking-wider mb-1">Output</div>
                        <div className="max-h-[200px] overflow-auto rounded border border-border/50 bg-muted/30 p-2 space-y-0.5">
                            {outputHistory.map((line, i) => (
                                <p key={i} className="text-[11px] text-secondary font-mono leading-relaxed">{line}</p>
                            ))}
                        </div>
                    </div>
                )}

                {/* Activity */}
                {taskActivities.length > 0 && (
                    <div>
                        <div className="text-[10px] text-muted-foreground uppercase tracking-wider mb-1">Activity</div>
                        <div className="space-y-0.5">
                            {taskActivities.map((a) => (
                                <div key={a.id} className="flex gap-2 text-[11px]">
                                    <span className="text-muted-foreground shrink-0 tabular-nums">
                                        {new Date(a.createdat * 1000).toLocaleTimeString()}
                                    </span>
                                    <span className="text-secondary">[{a.type}]</span>
                                    <span className="text-primary">{a.description}</span>
                                </div>
                            ))}
                        </div>
                    </div>
                )}

                {/* Error */}
                {task.error && (
                    <div>
                        <div className="text-[10px] text-muted-foreground uppercase tracking-wider mb-1">Error</div>
                        <p className="text-xs text-red-400 bg-red-500/5 rounded p-2">{task.error}</p>
                    </div>
                )}

                {/* Danger zone */}
                <div className="pt-4 border-t border-border/30">
                    {!confirmDelete ? (
                        <button
                            className="text-[11px] text-muted-foreground hover:text-red-400 cursor-pointer"
                            onClick={() => setConfirmDelete(true)}
                        >
                            Delete task
                        </button>
                    ) : (
                        <div className="flex items-center gap-2">
                            <span className="text-[11px] text-red-400">Confirm delete?</span>
                            <button className="text-[11px] text-red-400 hover:text-red-300 cursor-pointer font-medium" onClick={() => onDelete(task.taskid)}>Yes, delete</button>
                            <button className="text-[11px] text-muted-foreground hover:text-primary cursor-pointer" onClick={() => setConfirmDelete(false)}>Cancel</button>
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
}
