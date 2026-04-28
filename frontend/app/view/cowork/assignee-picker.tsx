// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import * as React from "react";
import { cn } from "@/util/util";

interface AssigneePickerProps {
    workers: CoworkWorker[];
    selectedWorkerId: string | null;
    onSelect: (workerId: string | null) => void;
}

export function AssigneePicker({ workers, selectedWorkerId, onSelect }: AssigneePickerProps) {
    const [open, setOpen] = React.useState(false);
    const selected = workers.find((w) => w.workerid === selectedWorkerId);

    const idleWorkers = workers.filter((w) => w.status === "idle");
    const busyWorkers = workers.filter((w) => w.status !== "idle");

    return (
        <div className="relative">
            <button
                className="flex items-center gap-1.5 px-2 py-1 rounded text-xs hover:bg-accent/30 transition-colors cursor-pointer"
                onClick={() => setOpen(!open)}
            >
                {selected ? (
                    <>
                        <span className={cn("w-1.5 h-1.5 rounded-full", selected.status === "idle" ? "bg-green-500" : "bg-yellow-400")} />
                        <span className="text-primary">{selected.name}</span>
                    </>
                ) : (
                    <span className="text-muted-foreground">Unassigned</span>
                )}
                <span className="text-muted-foreground">▾</span>
            </button>

            {open && (
                <>
                    <div className="fixed inset-0 z-40" onClick={() => setOpen(false)} />
                    <div className="absolute left-0 top-full mt-1 z-50 min-w-[180px] bg-card border border-border/50 rounded-lg shadow-xl py-1">
                        <button
                            className="w-full text-left px-3 py-1.5 text-xs text-muted-foreground hover:bg-accent/30 cursor-pointer"
                            onClick={() => { onSelect(null); setOpen(false); }}
                        >
                            Unassigned
                        </button>
                        {idleWorkers.length > 0 && (
                            <>
                                <div className="px-3 py-1 text-[10px] text-muted-foreground uppercase tracking-wider">Idle</div>
                                {idleWorkers.map((w) => (
                                    <button
                                        key={w.workerid}
                                        className={cn(
                                            "w-full text-left px-3 py-1.5 text-xs hover:bg-accent/30 cursor-pointer flex items-center gap-1.5",
                                            selectedWorkerId === w.workerid && "bg-accent/20",
                                        )}
                                        onClick={() => { onSelect(w.workerid); setOpen(false); }}
                                    >
                                        <span className="w-1.5 h-1.5 rounded-full bg-green-500" />
                                        <span className="text-primary">{w.name}</span>
                                        <span className="text-muted-foreground text-[10px]">{w.tool}</span>
                                    </button>
                                ))}
                            </>
                        )}
                        {busyWorkers.length > 0 && (
                            <>
                                <div className="px-3 py-1 text-[10px] text-muted-foreground uppercase tracking-wider">Busy</div>
                                {busyWorkers.map((w) => (
                                    <button
                                        key={w.workerid}
                                        className={cn(
                                            "w-full text-left px-3 py-1.5 text-xs hover:bg-accent/30 cursor-pointer flex items-center gap-1.5",
                                            selectedWorkerId === w.workerid && "bg-accent/20",
                                        )}
                                        onClick={() => { onSelect(w.workerid); setOpen(false); }}
                                    >
                                        <span className={cn("w-1.5 h-1.5 rounded-full", w.status === "working" ? "bg-yellow-400" : "bg-red-400")} />
                                        <span className="text-primary">{w.name}</span>
                                        <span className="text-muted-foreground text-[10px]">{w.status}</span>
                                    </button>
                                ))}
                            </>
                        )}
                    </div>
                </>
            )}
        </div>
    );
}
