// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import * as React from "react";
import { cn } from "@/util/util";
import { PriorityBadge } from "./priority-badge";

interface BoardCardProps {
    task: TeamTask;
    members: TeamWorker[];
    allTasks: TeamTask[];
    onClick: () => void;
    onRetry?: () => void;
}

const STATUS_DOT: Record<string, string> = {
    idle: "bg-muted-foreground",
    working: "bg-green-500",
    offline: "bg-muted-foreground/40",
    error: "bg-red-500",
};

export function BoardCard({ task, members, allTasks, onClick, onRetry }: BoardCardProps) {
    const assignedMember = task.assignedworkerid
        ? members.find((w) => w.workerid === task.assignedworkerid)
        : null;

    const isFailed = task.status === "failed";
    const isWorking = task.status === "working";
    const isDone = task.status === "done";

    return (
        <div
            className={cn(
                "group/card rounded-lg border-[0.5px] bg-card py-3 px-2.5 cursor-pointer transition-colors",
                "shadow-[0_3px_6px_-2px_rgba(0,0,0,0.02),0_1px_1px_0_rgba(0,0,0,0.04)]",
                "hover:border-accent hover:bg-accent/50",
                isFailed && "border-destructive/40",
            )}
            onClick={onClick}
        >
            {/* Row 1: Identifier */}
            <p className="text-[11px] text-muted-foreground">
                {task.taskid.slice(0, 8).toUpperCase()}
            </p>

            {/* Row 2: Title */}
            <p className={cn(
                "mt-1 text-sm font-medium leading-snug line-clamp-2",
                isDone && "text-muted-foreground",
            )}>
                {task.title}
            </p>

            {/* Row 3: Priority + Member + Progress */}
            <div className="mt-2 flex items-center gap-2 flex-wrap">
                <PriorityBadge priority={task.priority} />

                {assignedMember && (
                    <span className="flex items-center gap-1 text-[11px] text-secondary">
                        <span className={cn("w-1.5 h-1.5 rounded-full", STATUS_DOT[assignedMember.status] ?? "bg-muted-foreground/40")} />
                        <span className="text-secondary">{assignedMember.name}</span>
                    </span>
                )}

                {task.progress && (
                    <span className="text-[11px] text-muted-foreground tabular-nums">{task.progress}</span>
                )}

                {isWorking && !task.progress && (
                    <span className="text-[11px] text-blue-400">working...</span>
                )}

                {isDone && (
                    <span className="text-[11px] text-blue-400">✓ done</span>
                )}

                {isFailed && task.error && (
                    <span className="text-[11px] text-red-400 truncate max-w-[140px]">✗ {task.error}</span>
                )}
            </div>

            {/* Retry button for failed tasks */}
            {isFailed && task.retrycount != null && task.maxretries != null && task.retrycount < task.maxretries && (
                <button
                    className="mt-1.5 text-[11px] text-orange-400 hover:text-orange-300 cursor-pointer"
                    onClick={(e) => { e.stopPropagation(); onRetry?.(); }}
                >
                    ↻ Retry ({task.retrycount}/{task.maxretries})
                </button>
            )}

            {/* Row 4: Dependencies */}
            {task.dependson && task.dependson.length > 0 && (
                <div className="mt-1.5 text-[11px] text-muted-foreground">
                    deps: {task.dependson.map((depId) => {
                        const dep = allTasks.find((t) => t.taskid === depId);
                        return dep ? dep.title.slice(0, 20) : depId.slice(0, 8);
                    }).join(", ")}
                </div>
            )}
        </div>
    );
}
