// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { cn } from "@/util/util";

export function StatusStrip({ status }: { status: TeamStatusData }) {
    const items = [
        { label: "Workers", value: `${status.activeworkers}/${status.totalmembers}`, color: "text-secondary", dot: "bg-green-500" },
        { label: "Pending", value: status.pendingtasks, color: "text-muted-foreground", dot: "bg-muted-foreground" },
        { label: "Working", value: status.workingtasks, color: "text-yellow-400", dot: "bg-yellow-400" },
        { label: "Done", value: status.donetasks, color: "text-blue-400", dot: "bg-blue-400" },
        { label: "Failed", value: status.failedtasks, color: "text-red-400", dot: "bg-red-400" },
    ];
    return (
        <div className="flex items-center gap-4 px-3 py-1.5 border-b border-border/50 text-xs">
            {items.map((item) => (
                <span key={item.label} className="flex items-center gap-1.5">
                    <span className={cn("w-1.5 h-1.5 rounded-full", item.dot)} />
                    <span className="text-muted-foreground">{item.label}</span>
                    <span className={cn("font-medium tabular-nums", item.color)}>{item.value}</span>
                </span>
            ))}
        </div>
    );
}
