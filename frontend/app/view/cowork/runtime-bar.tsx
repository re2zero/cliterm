// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import * as React from "react";
import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";
import { cn } from "@/util/util";

export function RuntimeBar({ className }: { className?: string }) {
    const [runtimes, setRuntimes] = React.useState<AIRuntime[]>([]);
    const [loading, setLoading] = React.useState(true);

    React.useEffect(() => {
        RpcApi.CoworkDetectRuntimesCommand(TabRpcClient)
            .then((r) => setRuntimes(r.runtimes))
            .catch(() => {})
            .finally(() => setLoading(false));
    }, []);

    if (loading) return null;

    return (
        <div className={cn("flex items-center gap-2 px-4 py-2 border-b border-border/50", className)}>
            <span className="text-[11px] text-muted-foreground uppercase tracking-wider shrink-0">Runtime</span>
            <div className="flex items-center gap-2 flex-wrap">
                {runtimes.map((r) => {
                    const isOnline = r.status === "online";
                    return (
                        <div
                            key={r.name}
                            className={cn(
                                "flex items-center gap-1.5 px-2 py-0.5 rounded-md border text-xs",
                                isOnline
                                    ? "border-green-500/20 bg-green-500/5"
                                    : "border-border/50 bg-muted/30 opacity-50",
                            )}
                        >
                            <span className={cn("w-1.5 h-1.5 rounded-full shrink-0", isOnline ? "bg-green-500" : "bg-muted-foreground/40")} />
                            <span className={isOnline ? "text-secondary" : "text-muted-foreground"}>
                                {r.display_name ?? r.name}
                            </span>
                            {isOnline && r.version && (
                                <span className="text-muted-foreground text-[10px]">{r.version}</span>
                            )}
                            {!isOnline && (
                                <span className="text-muted-foreground text-[10px]">offline</span>
                            )}
                        </div>
                    );
                })}
                {runtimes.length === 0 && (
                    <span className="text-xs text-muted-foreground">none detected</span>
                )}
            </div>
        </div>
    );
}
