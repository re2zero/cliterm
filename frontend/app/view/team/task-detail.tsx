// Copyright 2026, Command Zone Inc.
// SPDX-License-Identifier: Apache-2.0

import * as React from "react";
import { cn } from "@/util/util";
import { PriorityBadge } from "./priority-badge";
import { AssigneePicker } from "./assignee-picker";
import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";

interface TaskDetailProps {
    task: TeamTask;
    workers: TeamWorker[];
    members: TeamMember[];
    allTasks: TeamTask[];
    activities: TeamActivity[];
    onClose: () => void;
    onUpdate: (taskId: string, updates: Record<string, any>) => void;
    onAssign: (taskId: string, memberId: string) => void;
    onPause: (taskId: string) => void;
    onResume: (taskId: string) => void;
    onDelete: (taskId: string) => void;
}

const STATUS_OPTIONS = ["pending", "assigned", "working", "paused", "done", "failed"] as const;
const PRIORITY_OPTIONS = ["low", "medium", "high", "urgent"] as const;

const STATUS_COLORS: Record<string, string> = {
    pending: "text-slate-500",
    assigned: "text-blue-400",
    working: "text-amber-400",
    paused: "text-yellow-400",
    done: "text-green-400",
    failed: "text-red-400",
};

const STATUS_BG: Record<string, string> = {
    pending: "bg-slate-500/10",
    assigned: "bg-blue-400/10",
    working: "bg-amber-400/10",
    paused: "bg-yellow-400/10",
    done: "bg-green-400/10",
    failed: "bg-red-400/10",
};

export function TaskDetail({
    task,
    workers,
    members,
    allTasks,
    activities,
    onClose,
    onUpdate,
    onAssign,
    onPause,
    onResume,
    onDelete,
}: TaskDetailProps) {
    const [confirmDelete, setConfirmDelete] = React.useState(false);
    const [showRunPicker, setShowRunPicker] = React.useState(false);
    const [outputHistory, setOutputHistory] = React.useState<TeamTaskOutput[]>([]);

    React.useEffect(() => {
        RpcApi.TeamGetTaskOutputHistoryCommand(TabRpcClient, task.taskid)
            .then((result) => setOutputHistory(result ?? []))
            .catch(() => {});
    }, [task.taskid]);

    const assignedMember = task.assignedworkerid
        ? workers.find((w) => w.workerid === task.assignedworkerid)
        : null;

    const taskActivities = (activities ?? []).filter((a) => a.taskid === task.taskid);
    const deps = task.dependson?.map((depId) => allTasks.find((t) => t.taskid === depId)).filter(Boolean) ?? [];

    const cycleStatus = () => {
        const currentIndex = STATUS_OPTIONS.indexOf(task.status as any);
        const nextIndex = (currentIndex + 1) % STATUS_OPTIONS.length;
        onUpdate(task.taskid, { status: STATUS_OPTIONS[nextIndex] });
    };

    const cyclePriority = () => {
        const currentIndex = PRIORITY_OPTIONS.indexOf(task.priority as any);
        const nextIndex = (currentIndex + 1) % PRIORITY_OPTIONS.length;
        onUpdate(task.taskid, { priority: PRIORITY_OPTIONS[nextIndex] });
    };

    return (
        <div className="flex flex-col h-full bg-[#0d0d14] border-l border-white/5 overflow-y-auto" style={{ width: 380, minWidth: 380 }}>
            <div className="flex items-center justify-between px-4 py-2.5 border-b border-white/5 shrink-0">
                <button
                    className="text-[10px] text-slate-500 hover:text-slate-200 transition-colors cursor-pointer"
                    onClick={onClose}
                >
                    ← Back
                </button>
                <span className="font-mono text-[10px] text-slate-500">{task.taskid.slice(0, 8).toUpperCase()}</span>
            </div>

            <div className="flex-1 overflow-y-auto px-4 py-3 space-y-4">
                <h3 className="text-base font-medium text-slate-200 leading-snug">{task.title}</h3>

                <div className="flex gap-2">
                    <button
                        className={cn(
                            "px-2 py-0.5 text-[10px] uppercase tracking-wider font-medium rounded-sm cursor-pointer transition-colors",
                            STATUS_BG[task.status] || STATUS_BG.pending,
                            STATUS_COLORS[task.status] || STATUS_COLORS.pending,
                        )}
                        onClick={cycleStatus}
                    >
                        [{task.status}]
                    </button>
                    <button
                        className="px-2 py-0.5 cursor-pointer transition-colors"
                        onClick={cyclePriority}
                    >
                        <PriorityBadge priority={task.priority} />
                    </button>
                </div>

                <div className="space-y-3">
                    <div className="flex items-center gap-3">
                        <span className="text-[10px] uppercase tracking-widest text-slate-500 font-medium w-16 shrink-0">Assigned</span>
                        {assignedMember ? (
                            <div className="flex items-center gap-1.5">
                                <span className={cn(
                                    "w-1.5 h-1.5 rounded-sm",
                                    assignedMember.status === "idle" ? "bg-green-400" :
                                    assignedMember.status === "working" ? "bg-amber-400 animate-pulse" :
                                    "bg-slate-500",
                                )} />
                                <span className="text-xs text-slate-300">{assignedMember.name}</span>
                                <span className={cn(
                                    "text-[9px] uppercase",
                                    assignedMember.status === "idle" ? "text-green-400" :
                                    assignedMember.status === "working" ? "text-amber-400" :
                                    "text-slate-500",
                                )}>
                                    {assignedMember.status}
                                </span>
                            </div>
                        ) : (
                            <span className="text-xs text-slate-500">Unassigned</span>
                        )}
                    </div>

                    <div className="flex items-center gap-3">
                        <span className="text-[10px] uppercase tracking-widest text-slate-500 font-medium w-16 shrink-0">Created</span>
                        <span className="text-xs text-slate-400 font-mono">
                            {task.createdat ? new Date(task.createdat * 1000).toLocaleString() : "—"}
                        </span>
                    </div>

                    {deps.length > 0 && (
                        <div className="flex items-start gap-3">
                            <span className="text-[10px] uppercase tracking-widest text-slate-500 font-medium w-16 shrink-0">Dependencies</span>
                            <div className="flex flex-wrap gap-1">
                                {deps.map((dep) => (
                                    <span
                                        key={dep.taskid}
                                        className={cn(
                                            "px-1.5 py-0.5 text-[9px] rounded-sm",
                                            dep.status === "done"
                                                ? "bg-green-400/10 text-green-400"
                                                : "bg-slate-500/10 text-slate-500",
                                        )}
                                    >
                                        {dep.title.slice(0, 20)}{dep.title.length > 20 ? "…" : ""} {dep.status === "done" && "✓"}
                                    </span>
                                ))}
                            </div>
                        </div>
                    )}
                </div>

                {task.description && (
                    <div className="pb-3 border-b border-white/5">
                        <div className="text-[10px] uppercase tracking-widest text-slate-500 font-medium mb-1">Description</div>
                        <p className="text-xs text-slate-400 leading-relaxed">{task.description}</p>
                    </div>
                )}

                <div className="pb-3 border-b border-white/5">
                    <div className="text-[10px] uppercase tracking-widest text-slate-500 font-medium mb-1.5">Actions</div>
                    <div className="flex flex-wrap gap-1.5">
                        {(task.status === "pending" || task.status === "failed" || task.status === "paused") && (
                            <button
                                className="px-2 py-1 text-[10px] rounded-sm bg-accent/20 text-accent hover:bg-accent/30 transition-colors cursor-pointer"
                                onClick={() => setShowRunPicker(!showRunPicker)}
                            >
                                {showRunPicker ? "✕ Run" : "▶ Run"}
                            </button>
                        )}
                        {task.status === "working" && (
                            <button
                                className="px-2 py-1 text-[10px] rounded-sm bg-amber-400/10 text-amber-400 hover:bg-amber-400/20 transition-colors cursor-pointer"
                                onClick={() => onPause(task.taskid)}
                            >
                                ⏸ Pause
                            </button>
                        )}
                        {task.status === "paused" && (
                            <button
                                className="px-2 py-1 text-[10px] rounded-sm bg-cyan-500/20 text-cyan-400 hover:bg-cyan-500/30 transition-colors cursor-pointer"
                                onClick={() => onResume(task.taskid)}
                            >
                                ▶ Resume
                            </button>
                        )}
                    </div>
                    {showRunPicker && members.length > 0 && (
                        <div className="mt-2 flex flex-col gap-1">
                            <div className="text-[10px] text-slate-500 mb-0.5">Assign to member:</div>
                            {members.map((m) => (
                                <button
                                    key={m.memberid}
                                    className="flex items-center gap-2 px-2 py-1 text-[11px] rounded-sm bg-white/[0.02] border border-white/[0.04] text-slate-300 hover:border-accent/30 hover:text-accent transition-colors cursor-pointer"
                                    onClick={() => {
                                        onAssign(task.taskid, m.memberid);
                                        setShowRunPicker(false);
                                    }}
                                >
                                    <span className="w-2 h-2 rounded-full bg-green-400" />
                                    {m.name}
                                    {m.tool && <span className="text-slate-500">({m.tool})</span>}
                                </button>
                            ))}
                        </div>
                    )}
                </div>

                {outputHistory.length > 0 && (
                    <div className="pb-3 border-b border-white/5">
                        <div className="text-[10px] uppercase tracking-widest text-slate-500 font-medium mb-1">Output</div>
                        <div className="max-h-[200px] overflow-auto rounded-sm border border-white/[0.04] bg-black/30 p-2 space-y-1">
                            {outputHistory.map((output, i) => (
                                <div key={i} className="text-[11px]">
                                    <span className="text-slate-500 tabular-nums font-mono">
                                        {output.timestamp ? new Date(output.timestamp).toLocaleTimeString() : ""}{" "}
                                    </span>
                                    <span className="text-slate-400 font-mono leading-relaxed">{output.content}</span>
                                </div>
                            ))}
                        </div>
                    </div>
                )}

                {taskActivities.length > 0 && (
                    <div className="pb-3 border-b border-white/5">
                        <div className="text-[10px] uppercase tracking-widest text-slate-500 font-medium mb-1">Activity</div>
                        <div className="space-y-0.5">
                            {taskActivities.map((a) => (
                                <div key={a.id} className="flex gap-2 text-[10px]">
                                    <span className="text-slate-500 shrink-0 tabular-nums font-mono">
                                        {new Date(a.createdat * 1000).toLocaleTimeString()}
                                    </span>
                                    <span className="text-slate-400">[{a.type}]</span>
                                    <span className="text-slate-300">{a.description}</span>
                                </div>
                            ))}
                        </div>
                    </div>
                )}

                {task.error && (
                    <div className="p-2 border-l-2 border-red-400 bg-red-500/5 rounded-sm">
                        <div className="text-[10px] uppercase tracking-widest text-red-400 font-medium mb-1">Error</div>
                        <p className="text-xs text-red-300 leading-relaxed">{task.error}</p>
                    </div>
                )}

                <div className="pt-2">
                    {!confirmDelete ? (
                        <button
                            className="text-[10px] text-slate-500 hover:text-red-400 transition-colors cursor-pointer"
                            onClick={() => setConfirmDelete(true)}
                        >
                            Delete task
                        </button>
                    ) : (
                        <div className="flex items-center gap-2">
                            <span className="text-[10px] text-red-400">Confirm delete?</span>
                            <button
                                className="text-[10px] text-red-400 hover:text-red-300 cursor-pointer font-medium"
                                onClick={() => onDelete(task.taskid)}
                            >
                                Yes
                            </button>
                            <button
                                className="text-[10px] text-slate-500 hover:text-slate-300 cursor-pointer"
                                onClick={() => setConfirmDelete(false)}
                            >
                                Cancel
                            </button>
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
}
