// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import * as React from "react";
import { BoardColumn, type ColumnConfig } from "./board-column";

const COLUMNS: ColumnConfig[] = [
    { status: "pending", title: "Pending", dotColor: "bg-muted-foreground", bgColor: "bg-muted/30" },
    { status: "working", title: "Working", dotColor: "bg-yellow-400", bgColor: "bg-yellow-400/5" },
    { status: "done", title: "Done", dotColor: "bg-blue-400", bgColor: "bg-blue-400/5" },
    { status: "failed", title: "Failed", dotColor: "bg-red-400", bgColor: "bg-red-400/5" },
];

interface BoardViewProps {
    pendingTasks: CoworkTask[];
    workingTasks: CoworkTask[];
    doneTasks: CoworkTask[];
    failedTasks: CoworkTask[];
    allTasks: CoworkTask[];
    workers: CoworkWorker[];
    onTaskClick: (task: CoworkTask) => void;
    onRetryTask: (taskId: string) => void;
}

export function BoardView({
    pendingTasks,
    workingTasks,
    doneTasks,
    failedTasks,
    allTasks,
    workers,
    onTaskClick,
    onRetryTask,
}: BoardViewProps) {
    const columnData = [
        { column: COLUMNS[0], tasks: pendingTasks },
        { column: COLUMNS[1], tasks: workingTasks },
        { column: COLUMNS[2], tasks: doneTasks },
        { column: COLUMNS[3], tasks: failedTasks },
    ];

    return (
        <div className="flex-1 flex gap-3 min-h-0 overflow-x-auto p-3">
            {columnData.map(({ column, tasks }) => (
                <BoardColumn
                    key={column.status}
                    column={column}
                    tasks={tasks}
                    workers={workers}
                    allTasks={allTasks}
                    onTaskClick={onTaskClick}
                    onRetryTask={onRetryTask}
                />
            ))}
        </div>
    );
}
