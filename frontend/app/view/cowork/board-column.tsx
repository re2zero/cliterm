// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import * as React from "react";
import { cn } from "@/util/util";
import { BoardCard } from "./board-card";

export interface ColumnConfig {
    status: string;
    title: string;
    dotColor: string;
    bgColor: string;
}

interface BoardColumnProps {
    column: ColumnConfig;
    tasks: CoworkTask[];
    workers: CoworkWorker[];
    allTasks: CoworkTask[];
    onTaskClick: (task: CoworkTask) => void;
    onRetryTask: (taskId: string) => void;
}

export function BoardColumn({ column, tasks, workers, allTasks, onTaskClick, onRetryTask }: BoardColumnProps) {
    return (
        <div className="flex flex-col min-w-[220px] flex-1 min-h-0">
            <div className={cn("flex items-center gap-2 px-3 py-2 rounded-t-lg", column.bgColor)}>
                <span className={cn("w-2 h-2 rounded-full", column.dotColor)} />
                <span className="text-xs font-semibold text-primary">{column.title}</span>
                <span className="text-xs text-muted-foreground tabular-nums">{tasks.length}</span>
            </div>

            <div className="flex-1 overflow-y-auto overflow-x-hidden rounded-b-lg border border-border/50 border-t-0 bg-card/30 p-2 space-y-2">
                {tasks.length === 0 ? (
                    <div className="flex items-center justify-center py-8 text-xs text-muted-foreground">
                        No tasks
                    </div>
                ) : (
                    tasks.map((task) => (
                        <BoardCard
                            key={task.taskid}
                            task={task}
                            workers={workers}
                            allTasks={allTasks}
                            onClick={() => onTaskClick(task)}
                            onRetry={() => onRetryTask(task.taskid)}
                        />
                    ))
                )}
            </div>
        </div>
    );
}
