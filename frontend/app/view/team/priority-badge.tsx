// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { cn } from "@/util/util";

// Priority config: number of filled bars and colors
const PRIORITY_CONFIG: Record<string, { bars: number; activeColor: string; bgColor: string; textColor: string }> = {
    urgent: { bars: 4, activeColor: "text-red-400", bgColor: "bg-red-500/10", textColor: "text-red-400" },
    high: { bars: 3, activeColor: "text-orange-400", bgColor: "bg-orange-500/10", textColor: "text-orange-400" },
    medium: { bars: 2, activeColor: "text-yellow-400", bgColor: "bg-yellow-500/10", textColor: "text-yellow-400" },
    low: { bars: 1, activeColor: "text-muted-foreground", bgColor: "bg-muted", textColor: "text-muted-foreground" },
};

export function PriorityBadge({ priority, className }: { priority: string; className?: string }) {
    const config = PRIORITY_CONFIG[priority] ?? PRIORITY_CONFIG["low"];
    return (
        <span className={cn("inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[11px] font-medium", config.bgColor, config.textColor, className)}>
            <span className="flex items-end gap-[1px] h-[10px]">
                {[1, 2, 3, 4].map((i) => (
                    <span
                        key={i}
                        className={cn(
                            "w-[3px] rounded-[0.5px]",
                            i <= config.bars ? config.activeColor : "text-muted-foreground/30",
                        )}
                        style={{ height: `${i * 2 + 2}px` }}
                    />
                ))}
            </span>
            <span>{priority}</span>
        </span>
    );
}

export function PriorityIcon({ priority, className }: { priority: string; className?: string }) {
    const config = PRIORITY_CONFIG[priority] ?? PRIORITY_CONFIG["low"];
    return (
        <span className={cn("flex items-end gap-[1px] h-[12px]", className)}>
            {[1, 2, 3, 4].map((i) => (
                <span
                    key={i}
                    className={cn(
                        "w-[3px] rounded-[0.5px]",
                        i <= config.bars ? config.activeColor : "text-muted-foreground/30",
                    )}
                    style={{ height: `${i * 2 + 3}px` }}
                />
            ))}
        </span>
    );
}
