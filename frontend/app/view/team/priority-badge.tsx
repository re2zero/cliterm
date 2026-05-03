// Copyright 2026, Command Zone Inc.
// SPDX-License-Identifier: Apache-2.0

import { cn } from "@/util/util";

const PRIORITY_CONFIG: Record<string, { barColor: string; textColor: string; icon?: string }> = {
    urgent: { barColor: "bg-red-400", textColor: "text-red-400", icon: "!" },
    high: { barColor: "bg-orange-400", textColor: "text-orange-400" },
    medium: { barColor: "bg-cyan-500/60", textColor: "text-cyan-400" },
    low: { barColor: "bg-slate-600", textColor: "text-slate-500" },
};

export function PriorityBar({ priority, className }: { priority: string; className?: string }) {
    const config = PRIORITY_CONFIG[priority] ?? PRIORITY_CONFIG["low"];
    return (
        <div className={cn("w-[3px] h-full rounded-r-sm shrink-0", config.barColor, className)} />
    );
}

export function PriorityBadge({ priority, className }: { priority: string; className?: string }) {
    const config = PRIORITY_CONFIG[priority] ?? PRIORITY_CONFIG["low"];
    return (
        <span className={cn("text-[10px] uppercase tracking-wider font-medium", config.textColor, className)}>
            [{priority}]
        </span>
    );
}

export function PriorityIcon({ priority, className }: { priority: string; className?: string }) {
    const config = PRIORITY_CONFIG[priority] ?? PRIORITY_CONFIG["low"];
    return (
        <div className={cn("flex items-center gap-1", className)}>
            <div className={cn("w-[3px] h-3 rounded-sm", config.barColor)} />
            {config.icon && (
                <span className={cn("text-[9px] font-bold", config.textColor)}>{config.icon}</span>
            )}
        </div>
    );
}
