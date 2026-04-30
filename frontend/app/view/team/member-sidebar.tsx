// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import * as React from "react";
import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";
import { Markdown } from "@/app/element/markdown";
import { cn } from "@/util/util";

interface WorkerSidebarProps {
    workers: TeamWorker[];
    onClose: () => void;
    onCreateWorker: (tool: string, config: WorkerFormData) => Promise<string | void>;
    onDeleteWorker: (workerId: string) => void;
}

interface WorkerFormData {
    name?: string;
    description?: string;
    persona?: string;
    skills?: string[];
    mcpservers?: TeamMCPConfig[];
    capabilities?: string[];
    customcmd?: string;
    maxretries?: number;
    maxconcurrency?: number;
}

const STATUS_DOT: Record<string, string> = {
    idle: "bg-green-500",
    working: "bg-yellow-400",
    offline: "bg-muted-foreground/40",
    error: "bg-red-500",
};

const CAPABILITIES = ["frontend", "backend", "debugging", "testing", "review", "refactor"];
const RUNTIMES = ["claude", "opencode", "cursor", "aider"];

export function EditorWithPreview({ label, value, onChange, placeholder, rows = 4 }: {
    label: string;
    value: string;
    onChange: (v: string) => void;
    placeholder?: string;
    rows?: number;
}) {
    const [mode, setMode] = React.useState<"source" | "preview">("source");
    const inputCls = "w-full bg-base border border-border/50 rounded text-sm text-primary focus:outline-none focus:ring-1 focus:ring-accent px-2.5 py-1.5 font-mono";

    return (
        <div>
            <div className="flex items-center justify-between mb-1">
                <span className="text-[11px] text-muted-foreground">{label}</span>
                <div className="flex rounded border border-border/50 overflow-hidden">
                    <button
                        className={cn("px-1.5 py-0.5 text-[10px] cursor-pointer", mode === "source" ? "bg-accent/20 text-accent" : "text-muted-foreground hover:text-primary")}
                        onClick={() => setMode("source")}
                    >Source</button>
                    <button
                        className={cn("px-1.5 py-0.5 text-[10px] cursor-pointer border-l border-border/50", mode === "preview" ? "bg-accent/20 text-accent" : "text-muted-foreground hover:text-primary")}
                        onClick={() => setMode("preview")}
                    >Preview</button>
                </div>
            </div>
            {mode === "source" ? (
                <textarea
                    className={cn(inputCls, "resize-y")}
                    rows={rows}
                    value={value}
                    onChange={(e) => onChange(e.target.value)}
                    placeholder={placeholder}
                />
            ) : (
                <div className="border border-border/50 rounded p-2 max-h-[200px] overflow-auto bg-base">
                    {value ? (
                        <Markdown text={value} className="text-xs" scrollable={false} />
                    ) : (
                        <span className="text-xs text-muted-foreground italic">No content to preview</span>
                    )}
                </div>
            )}
        </div>
    );
}

export function WorkerSidebar({ workers, onClose, onCreateWorker, onDeleteWorker }: WorkerSidebarProps) {
    const [selectedWorkerId, setSelectedWorkerId] = React.useState<string | null>(null);
    const [isNew, setIsNew] = React.useState(false);
    const [confirmDelete, setConfirmDelete] = React.useState<string | null>(null);
    const [submitting, setSubmitting] = React.useState(false);
    const [runtimes, setRuntimes] = React.useState<AIRuntime[]>([]);

    // Form state
    const [tool, setTool] = React.useState("claude");
    const [name, setName] = React.useState("");
    const [maxRetries, setMaxRetries] = React.useState(3);
    const [capabilities, setCapabilities] = React.useState<string[]>([]);
    const [description, setDescription] = React.useState("");
    const [customCmd, setCustomCmd] = React.useState("");
    const [persona, setPersona] = React.useState("");
    const [skills, setSkills] = React.useState("");
    const [mcpServers, setMcpServers] = React.useState("");

    React.useEffect(() => {
        RpcApi.TeamDetectRuntimesCommand(TabRpcClient)
            .then((r) => setRuntimes(r.runtimes))
            .catch(() => {});
    }, []);

    React.useEffect(() => {
        if (isNew) {
            setName(tool === "opencode" ? "OpenCode Worker" : `${tool.charAt(0).toUpperCase() + tool.slice(1)} Worker`);
        }
    }, [tool, isNew]);

    const handleNew = () => {
        setIsNew(true);
        setSelectedWorkerId(null);
        setTool("claude");
        setName("");
        setMaxRetries(3);
        setCapabilities([]);
        setDescription("");
        setCustomCmd("");
        setPersona("");
        setSkills("");
        setMcpServers("");
    };

    const handleSelectWorker = (workerId: string) => {
        const w = workers.find((w) => w.workerid === workerId);
        if (!w) return;
        setSelectedWorkerId(workerId);
        setIsNew(false);
        setTool("claude");
        setName(w.name ?? "");
        setMaxRetries(3);
        setCapabilities([]);
        setDescription("");
        setCustomCmd("");
        setPersona("");
        setSkills("");
        setMcpServers("");
    };

    const handleSubmit = async () => {
        if (submitting) return;
        setSubmitting(true);
        try {
            await onCreateWorker(tool, {
                name: name || undefined,
                maxretries: maxRetries > 0 ? maxRetries : undefined,
                capabilities: capabilities.length > 0 ? capabilities : undefined,
                description: description || undefined,
                persona: persona || undefined,
                skills: skills ? skills.split(",").map((s) => s.trim()).filter(Boolean) : undefined,
                mcpservers: mcpServers ? mcpServers.split(",").map((s) => ({ name: s.trim() } as TeamMCPConfig)).filter((c) => c.name) : undefined,
                customcmd: customCmd || undefined,
            });
            setIsNew(false);
        } catch (e) {
            console.error("Failed to create worker:", e);
        } finally {
            setSubmitting(false);
        }
    };

    const selectedWorker = workers.find((w) => w.workerid === selectedWorkerId);

    const inputCls = "w-full bg-base border border-border/50 rounded text-sm text-primary focus:outline-none focus:ring-1 focus:ring-accent px-2.5 py-1.5";

    const showDetail = isNew || selectedWorker;

    return (
        <div className="flex h-full border-l border-border/50 bg-card" style={{ width: 480, minWidth: 480, colorScheme: "dark" }}>
            {/* Left: Worker list */}
            <div className="w-[160px] min-w-[160px] border-r border-border/50 flex flex-col">
                <div className="flex items-center justify-between px-3 py-2 border-b border-border/50">
                    <span className="text-xs font-semibold text-primary">Workers</span>
                    <button className="text-muted-foreground hover:text-primary cursor-pointer text-sm" onClick={onClose}>✕</button>
                </div>
                <div className="flex-1 overflow-y-auto py-1">
                    {workers.map((w) => (
                        <button
                            key={w.workerid}
                            className={cn(
                                "w-full text-left px-3 py-2 hover:bg-accent/30 cursor-pointer transition-colors border-l-2",
                                selectedWorkerId === w.workerid ? "border-l-accent bg-accent/10" : "border-l-transparent",
                            )}
                            onClick={() => handleSelectWorker(w.workerid)}
                        >
                            <div className="flex items-center gap-1.5">
                                <span className={cn("w-1.5 h-1.5 rounded-full shrink-0", STATUS_DOT[w.status] ?? "bg-muted-foreground/40")} />
                                <span className="text-xs text-primary truncate">{w.name}</span>
                            </div>
                            <div className="text-[10px] text-muted-foreground mt-0.5 pl-3">{w.status}</div>
                        </button>
                    ))}
                    {workers.length === 0 && !isNew && (
                        <div className="px-3 py-4 text-[11px] text-muted-foreground text-center">
                            No workers yet
                        </div>
                    )}
                </div>
                <div className="border-t border-border/50 p-2">
                    <button
                        className="w-full px-2 py-1.5 rounded text-xs bg-accent/80 text-primary hover:bg-accent cursor-pointer font-medium transition-colors"
                        onClick={handleNew}
                    >
                        + New Worker
                    </button>
                </div>
            </div>

            {/* Right: Detail / Create form */}
            <div className="flex-1 flex flex-col min-w-0 overflow-y-auto">
                {!showDetail ? (
                    <div className="flex-1 flex items-center justify-center text-xs text-muted-foreground">
                        Select a worker or create new
                    </div>
                ) : (
                    <div className="p-4 space-y-4">
                        {/* Title */}
                        <div className="flex items-center justify-between">
                            <h3 className="text-sm font-semibold text-primary">
                                {isNew ? "New Worker" : selectedWorker?.name ?? "Worker"}
                            </h3>
                            {selectedWorker && (
                                <span className={cn("px-1.5 py-0.5 rounded text-[10px]", selectedWorker.status === "idle" ? "bg-green-500/10 text-green-400" : selectedWorker.status === "working" ? "bg-yellow-500/10 text-yellow-400" : "bg-muted text-muted-foreground")}>
                                    {selectedWorker.status}
                                </span>
                            )}
                        </div>

                        {/* Name */}
                        <div>
                            <label className="text-[11px] text-muted-foreground mb-0.5 block">Name</label>
                            <input className={inputCls} value={name} onChange={(e) => setName(e.target.value)} disabled={!isNew} />
                        </div>

                        {/* Runtime selector (only for new) */}
                        {isNew && (
                            <div>
                                <label className="text-[11px] text-muted-foreground mb-1 block">Runtime</label>
                                <div className="grid grid-cols-2 gap-1.5">
                                    {RUNTIMES.map((rt) => {
                                        const runtime = runtimes.find((r) => r.name === rt);
                                        const isOnline = runtime?.status === "online";
                                        return (
                                            <button
                                                key={rt}
                                                className={cn(
                                                    "flex items-center gap-1.5 px-2 py-1.5 rounded border cursor-pointer transition-colors",
                                                    tool === rt ? "border-accent bg-accent/10" : "border-border/50 hover:border-accent/50",
                                                    !isOnline && "opacity-50",
                                                )}
                                                onClick={() => setTool(rt)}
                                            >
                                                <span className={cn("w-1.5 h-1.5 rounded-full shrink-0", isOnline ? "bg-green-500" : "bg-muted-foreground/40")} />
                                                <span className="text-xs text-primary">{rt === "opencode" ? "OpenCode" : rt.charAt(0).toUpperCase() + rt.slice(1)}</span>
                                                {isOnline && runtime?.version && <span className="text-[9px] text-muted-foreground">{runtime.version}</span>}
                                            </button>
                                        );
                                    })}
                                </div>
                            </div>
                        )}

                        {/* Tool info (view only for existing) */}
                        {!isNew && selectedWorker && (
                            <div>
                                <label className="text-[11px] text-muted-foreground mb-0.5 block">Status</label>
                                <span className="text-sm text-secondary">{selectedWorker.status}</span>
                            </div>
                        )}

                        {/* Custom Command */}
                        {isNew && (
                            <div>
                                <label className="text-[11px] text-muted-foreground mb-0.5 block">Custom Command</label>
                                <input className={inputCls} placeholder="Override default launch command..." value={customCmd} onChange={(e) => setCustomCmd(e.target.value)} />
                            </div>
                        )}

                        {/* Retries */}
                        <div className="flex gap-3">
                            <label className="flex items-center gap-1 text-xs text-muted-foreground">
                                Retries <input type="number" className="w-16 bg-base border border-border/50 rounded text-sm text-primary text-center px-1.5 py-1 focus:outline-none focus:ring-1 focus:ring-accent" value={maxRetries} onChange={(e) => setMaxRetries(Number(e.target.value))} />
                            </label>
                        </div>

                        {/* Capabilities */}
                        <div>
                            <label className="text-[11px] text-muted-foreground mb-1 block">Capabilities</label>
                            <div className="flex flex-wrap gap-1.5">
                                {CAPABILITIES.map((cap) => (
                                    <button
                                        key={cap}
                                        className={cn(
                                            "px-2 py-0.5 text-[11px] rounded-full border cursor-pointer transition-colors",
                                            capabilities.includes(cap) ? "bg-accent/20 border-accent text-accent" : "border-border/50 text-muted-foreground hover:border-accent/50",
                                        )}
                                        onClick={() => setCapabilities((prev) => prev.includes(cap) ? prev.filter((c) => c !== cap) : [...prev, cap])}
                                    >{cap}</button>
                                ))}
                            </div>
                        </div>

                        {/* Description */}
                        <EditorWithPreview
                            label="Description"
                            value={description}
                            onChange={setDescription}
                            placeholder="Describe this worker's purpose..."
                            rows={3}
                        />

                        {/* System Prompt */}
                        <EditorWithPreview
                            label="Persona (system prompt)"
                            value={persona}
                            onChange={setPersona}
                            placeholder="# System Prompt\n\nYou are a senior engineer..."
                            rows={6}
                        />

                        {/* Skills */}
                        <EditorWithPreview
                            label="Skills"
                            value={skills}
                            onChange={setSkills}
                            placeholder="List of skills, one per line..."
                            rows={3}
                        />

                        {/* MCP Servers */}
                        <EditorWithPreview
                            label="MCP Servers"
                            value={mcpServers}
                            onChange={setMcpServers}
                            placeholder='{"mcpServers": {...}}'
                            rows={4}
                        />

                        {/* Actions */}
                        {isNew ? (
                            <div className="flex justify-end gap-2 pt-2">
                                <button className="px-3 py-1.5 rounded text-sm text-muted-foreground hover:text-primary cursor-pointer" onClick={() => setIsNew(false)}>Cancel</button>
                                <button className="px-3 py-1.5 rounded bg-accent/80 text-primary hover:bg-accent text-sm font-medium cursor-pointer disabled:opacity-50" onClick={handleSubmit} disabled={submitting}>
                                    {submitting ? "Creating..." : "Create"}
                                </button>
                            </div>
                        ) : selectedWorker && (
                            <div className="pt-4 border-t border-border/30">
                                {!confirmDelete ? (
                                    <button className="text-[11px] text-muted-foreground hover:text-red-400 cursor-pointer" onClick={() => setConfirmDelete(selectedWorker.workerid)}>
                                        Delete worker
                                    </button>
                                ) : (
                                    <div className="flex items-center gap-2">
                                        <span className="text-[11px] text-red-400">Confirm delete?</span>
                                        <button className="text-[11px] text-red-400 hover:text-red-300 cursor-pointer font-medium" onClick={() => { onDeleteWorker(selectedWorker.workerid); setSelectedWorkerId(null); setConfirmDelete(null); }}>Yes</button>
                                        <button className="text-[11px] text-muted-foreground hover:text-primary cursor-pointer" onClick={() => setConfirmDelete(null)}>Cancel</button>
                                    </div>
                                )}
                            </div>
                        )}
                    </div>
                )}
            </div>
        </div>
    );
}
