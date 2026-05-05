// Copyright 2026, Command Zone Inc.
// SPDX-License-Identifier: Apache-2.0

import * as React from "react";
import { cn } from "@/util/util";
import { PriorityBar } from "./priority-badge";

interface BoardCardProps {
    task: TeamTask;
    members: TeamWorker[];
    allTasks: TeamTask[];
    onClick: () => void;
}

const STATUS_DOT: Record<string, string> = {
    idle: "bg-green-400",
    working: "bg-amber-400 animate-pulse",
    offline: "bg-slate-500",
    error: "bg-red-400",
};

export function BoardCard({ task, members, allTasks, onClick }: BoardCardProps) {
    const assignedMember = task.assignedworkerid
        ? members.find((w) => w.workerid === task.assignedworkerid)
        : null;

    const isFailed = task.status === "failed";
    const isWorking = task.status === "working";
    const isDone = task.status === "done";
    const isPaused = task.status === "paused";

    const deps = task.dependson?.map((depId) => allTasks.find((t) => t.taskid === depId)).filter(Boolean) ?? [];
    const timestamp = task.createdat ? new Date(task.createdat * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : "";

    return (
        <div
            className={cn(
                "group/row flex items-center gap-2 h-12 px-2 transition-all duration-150 cursor-pointer",
                "bg-white/[0.02] border border-white/[0.04] rounded-md",
                "hover:bg-white/[0.03]",
                isFailed && "border-l-2 border-l-red-400/60",
                isWorking && "border-l-2 border-l-cyan-500/40",
                isDone && "opacity-70",
            )}
            onClick={onClick}
        >
            <PriorityBar priority={task.priority} />

            <div className="flex-1 min-w-0">
                <p className={cn(
                    "text-sm text-slate-200 truncate font-medium",
                    isDone && "text-slate-400",
                )}>
                    {task.title}
                </p>
                {deps.length > 0 && (
                    <div className="flex gap-1 mt-0.5">
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
                                {dep.title.slice(0, 15)}{dep.title.length > 15 ? "…" : ""}
                            </span>
                        ))}
                    </div>
                )}
            </div>

            <div className="flex items-center gap-2 shrink-0">
                {assignedMember && (
                    <div className="flex items-center gap-1.5">
                        <span className={cn(
                            "w-1.5 h-1.5 rounded-sm",
                            STATUS_DOT[assignedMember.status] ?? "bg-slate-500",
                        )} />
                        <span className="text-xs text-slate-300">{assignedMember.name}</span>
                    </div>
                )}

                <span className="text-[10px] text-slate-500 font-mono tabular-nums">
                    {isWorking && (
                        <span className="text-cyan-400 animate-pulse">working</span>
                    )}
                    {isPaused && (
                        <span className="text-amber-400">paused</span>
                    )}
                    {isDone && (
                        <span className="text-green-400">done</span>
                    )}
                    {isFailed && (
                        <span className="text-red-400">failed</span>
                    )}
                    {!isWorking && !isPaused && !isDone && !isFailed && (
                        <span className="text-slate-500">{timestamp}</span>
                    )}
                </span>
            </div>
        </div>
    );
}
