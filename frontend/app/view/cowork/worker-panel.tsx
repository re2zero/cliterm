// Copyright 2026, Command Zone Inc.
// SPDX-License-Identifier: Apache-2.0

import * as React from "react";
import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";
import { Markdown } from "@/app/element/markdown";
import { cn } from "@/util/util";

export interface WorkerFormData {
    name?: string;
    concurrency?: number;
    timeout?: number;
    maxRetries?: number;
    capabilities?: string[];
    role?: string;
    desc?: string;
    soul?: string;
    skills?: string[];
    mcpservers?: string[];
    customcmd?: string;
}

const STATUS_ICON: Record<string, { dot: string; label: string }> = {
    idle: { dot: "bg-green-500", label: "Idle" },
    working: { dot: "bg-yellow-400 animate-pulse", label: "Working" },
    offline: { dot: "bg-muted-foreground/40", label: "Offline" },
    error: { dot: "bg-red-500", label: "Error" },
};

const CAPABILITIES = ["frontend", "backend", "debugging", "testing", "review", "refactor"];
const RUNTIMES = ["claude", "opencode", "cursor", "aider"];
const PRESET_SKILLS = ["code-review", "git-master", "frontend-design", "qt-cpp-bug-fixer", "webapp-testing", "docx", "pdf", "xlsx"];
const PRESET_MCPS = ["filesystem", "github", "browser", "database", "fetch"];

function Section({ title, children }: { title: string; children: React.ReactNode }) {
    return (
        <div>
            <h4 className="text-[11px] font-semibold text-muted-foreground uppercase tracking-wider mb-2 pb-1 border-b border-border/30">{title}</h4>
            <div className="space-y-2.5">{children}</div>
        </div>
    );
}

function FieldRow({ label, children, className }: { label: string; children: React.ReactNode; className?: string }) {
    return (
        <div className={cn("flex flex-col gap-1", className)}>
            <label className="text-[11px] text-muted-foreground">{label}</label>
            {children}
        </div>
    );
}

interface WorkerListProps {
    workers: CoworkWorker[];
    onEditWorker: (workerId: string) => void;
    onDeleteWorker: (workerId: string) => void;
    onNewWorker: () => void;
}

export function WorkerList({ workers, onEditWorker, onDeleteWorker, onNewWorker }: WorkerListProps) {
    const [deleteTarget, setDeleteTarget] = React.useState<CoworkWorker | null>(null);

    const handleDelete = () => {
        if (!deleteTarget) return;
        onDeleteWorker(deleteTarget.workerid);
        setDeleteTarget(null);
    };

    return (
        <div className="w-[140px] min-w-[120px] border-r border-border/50 flex flex-col bg-card/50">
            <div className="flex items-center justify-between px-2.5 py-2 border-b border-border/50">
                <span className="text-[11px] font-semibold text-primary uppercase tracking-wider">Workers</span>
                <span className="text-[10px] text-muted-foreground tabular-nums">{workers.length}</span>
            </div>
            <div className="flex-1 overflow-y-auto py-0.5">
                {workers.map((w) => (
                    <WorkerItem
                        key={w.workerid}
                        worker={w}
                        onEdit={() => onEditWorker(w.workerid)}
                        onDelete={() => setDeleteTarget(w)}
                    />
                ))}
                {workers.length === 0 && (
                    <div className="px-2 py-6 text-[10px] text-muted-foreground text-center leading-relaxed">
                        No workers<br />Create one ↓
                    </div>
                )}
            </div>
            <div className="border-t border-border/50 p-1.5">
                <button
                    className="w-full px-2 py-1.5 rounded text-[11px] bg-accent/80 text-primary hover:bg-accent cursor-pointer font-medium transition-colors"
                    onClick={onNewWorker}
                >
                    + New
                </button>
            </div>

            {deleteTarget && (
                <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50"
                    onClick={(e) => e.target === e.currentTarget && setDeleteTarget(null)}>
                    <div className="bg-card border border-border/50 rounded-lg shadow-2xl p-4 w-72" style={{ colorScheme: "dark" }}>
                        <h3 className="text-sm font-semibold text-primary mb-1.5">Delete Worker</h3>
                        <p className="text-xs text-secondary mb-1">
                            Delete <span className="text-primary font-medium">{deleteTarget.name}</span>?
                        </p>
                        <p className="text-[11px] text-red-400 mb-3">This action cannot be undone.</p>
                        <div className="flex justify-end gap-2">
                            <button className="px-2.5 py-1 rounded text-xs text-muted-foreground hover:text-primary cursor-pointer" onClick={() => setDeleteTarget(null)}>Cancel</button>
                            <button className="px-2.5 py-1 rounded bg-red-500/80 text-white hover:bg-red-500 text-xs font-medium cursor-pointer" onClick={handleDelete}>Delete</button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}

function WorkerItem({ worker, onEdit, onDelete }: {
    worker: CoworkWorker;
    onEdit: () => void;
    onDelete: () => void;
}) {
    const [menuOpen, setMenuOpen] = React.useState(false);
    const menuRef = React.useRef<HTMLDivElement>(null);
    const status = STATUS_ICON[worker.status] ?? STATUS_ICON["offline"];

    React.useEffect(() => {
        if (!menuOpen) return;
        const handler = (e: MouseEvent) => {
            if (menuRef.current && !menuRef.current.contains(e.target as Node)) setMenuOpen(false);
        };
        document.addEventListener("mousedown", handler);
        return () => document.removeEventListener("mousedown", handler);
    }, [menuOpen]);

    return (
        <div
            className="flex items-center gap-1.5 px-2 py-1.5 cursor-pointer hover:bg-accent/5 transition-colors group"
            onClick={onEdit}
        >
            <span className={cn("w-1.5 h-1.5 rounded-full shrink-0", status.dot)} title={status.label} />
            <div className="flex-1 min-w-0">
                <div className="text-[11px] text-primary truncate leading-tight">{worker.name}</div>
                <div className="text-[9px] text-muted-foreground">{worker.tool}</div>
            </div>
            <div className="relative" ref={menuRef}>
                <button
                    className="p-0.5 rounded text-muted-foreground hover:text-primary cursor-pointer opacity-0 group-hover:opacity-100 transition-opacity"
                    onClick={(e) => { e.stopPropagation(); setMenuOpen(!menuOpen); }}
                >⋮</button>
                {menuOpen && (
                    <div className="absolute right-0 top-full mt-0.5 bg-card border border-border/50 rounded shadow-lg z-20 py-0.5 min-w-[72px]">
                        <button className="w-full text-left px-2 py-1 text-[11px] text-primary hover:bg-accent/10 cursor-pointer"
                            onClick={(e) => { e.stopPropagation(); setMenuOpen(false); onEdit(); }}>Edit</button>
                        <button className="w-full text-left px-2 py-1 text-[11px] text-red-400 hover:bg-red-500/10 cursor-pointer"
                            onClick={(e) => { e.stopPropagation(); setMenuOpen(false); onDelete(); }}>Delete</button>
                    </div>
                )}
            </div>
        </div>
    );
}

interface WorkerEditorProps {
    worker?: CoworkWorker;
    workers: CoworkWorker[];
    onClose: () => void;
    onSubmit: (tool: string, config: WorkerFormData) => Promise<string | void>;
}

export function WorkerEditor({ worker, onClose, onSubmit }: WorkerEditorProps) {
    const isCreating = !worker;
    const [submitting, setSubmitting] = React.useState(false);
    const [runtimes, setRuntimes] = React.useState<AIRuntime[]>([]);

    const [tool, setTool] = React.useState(worker?.tool ?? "claude");
    const [name, setName] = React.useState(worker?.name ?? "");
    const [concurrency, setConcurrency] = React.useState((worker as any)?.concurrency ?? 3);
    const [timeout, setTimeout_] = React.useState((worker as any)?.timeout ?? 300);
    const [maxRetries, setMaxRetries] = React.useState((worker as any)?.maxretries ?? 3);
    const [capabilities, setCapabilities] = React.useState<string[]>(worker?.capabilities ?? []);
    const [role, setRole] = React.useState(worker?.role ?? "");
    const [desc, setDesc] = React.useState(worker?.desc ?? "");
    const [customCmd, setCustomCmd] = React.useState(worker?.customcmd ?? "");
    const [soul, setSoul] = React.useState(worker?.soul ?? "");
    const [skills, setSkills] = React.useState<string[]>(worker?.skills ? worker.skills.split(",").map((s) => s.trim()).filter(Boolean) : []);
    const [mcpServers, setMcpServers] = React.useState<string[]>(worker?.mcpservers ? worker.mcpservers.split(",").map((s) => s.trim()).filter(Boolean) : []);

    React.useEffect(() => {
        RpcApi.CoworkDetectRuntimesCommand(TabRpcClient)
            .then((r) => setRuntimes(r.runtimes))
            .catch(() => {});
    }, []);

    React.useEffect(() => {
        if (isCreating) {
            setName(tool === "opencode" ? "OpenCode Worker" : `${tool.charAt(0).toUpperCase() + tool.slice(1)} Worker`);
        }
    }, [tool, isCreating]);

    const handleSubmit = async () => {
        if (submitting) return;
        setSubmitting(true);
        await onSubmit(tool, {
            name: name || undefined,
            concurrency, timeout, maxRetries,
            capabilities: capabilities.length > 0 ? capabilities : undefined,
            role: role || undefined,
            desc: desc || undefined,
            soul: soul || undefined,
            skills: skills.length > 0 ? skills : undefined,
            mcpservers: mcpServers.length > 0 ? mcpServers : undefined,
            customcmd: customCmd || undefined,
        });
        setSubmitting(false);
        onClose();
    };

    const inputCls = "w-full bg-base border border-border/50 rounded text-sm text-primary focus:outline-none focus:ring-1 focus:ring-accent px-2.5 py-1.5";
    const inputSmCls = "w-16 bg-base border border-border/50 rounded text-sm text-primary text-center px-1.5 py-1 focus:outline-none focus:ring-1 focus:ring-accent";

    const presetBtnCls = (preset: string) => {
        const isActive = (preset === "standard" && concurrency === 3) || (preset === "quick" && concurrency === 1) || (preset === "power" && concurrency === 5);
        return cn(
            "px-2 py-1 text-xs rounded border cursor-pointer transition-colors",
            isActive ? "bg-accent/20 border-accent text-accent" : "border-border/50 text-muted-foreground hover:border-accent/50",
        );
    };

    return (
        <div className="flex flex-col h-full w-full bg-card" style={{ colorScheme: "dark" }}>
            <div className="flex items-center justify-between px-5 py-3 border-b border-border/50 shrink-0">
                <div className="flex items-center gap-2">
                    <h3 className="text-sm font-semibold text-primary">
                        {isCreating ? "New Worker" : worker?.name ?? "Worker"}
                    </h3>
                    {!isCreating && worker && (
                        <span className={cn(
                            "px-1.5 py-0.5 rounded text-[10px]",
                            worker.status === "idle" ? "bg-green-500/10 text-green-400" :
                            worker.status === "working" ? "bg-yellow-500/10 text-yellow-400" :
                            "bg-muted text-muted-foreground",
                        )}>
                            {STATUS_ICON[worker.status]?.label ?? worker.status}
                        </span>
                    )}
                </div>
                <button className="text-muted-foreground hover:text-primary cursor-pointer text-sm" onClick={onClose}>✕</button>
            </div>

            <div className="flex-1 overflow-y-auto px-5 py-4 space-y-5">
                <Section title="Identity">
                    <FieldRow label="Name">
                        <input className={inputCls} value={name} onChange={(e) => setName(e.target.value)} disabled={!isCreating} />
                    </FieldRow>
                    <FieldRow label="Role">
                        <input className={inputCls} placeholder="e.g. Frontend specialist" value={role} onChange={(e) => setRole(e.target.value)} />
                    </FieldRow>
                    <FieldRow label="Description">
                        <textarea className={cn(inputCls, "resize-y")} rows={2} value={desc} onChange={(e) => setDesc(e.target.value)}
                            placeholder="Brief description for users..." />
                    </FieldRow>
                </Section>

                {isCreating && (
                    <Section title="Runtime">
                        <div className="grid grid-cols-2 gap-2">
                            {RUNTIMES.map((rt) => {
                                const runtime = runtimes.find((r) => r.name === rt);
                                const isOnline = runtime?.status === "online";
                                return (
                                    <button key={rt}
                                        className={cn(
                                            "flex items-center gap-2 px-3 py-2 rounded border cursor-pointer transition-colors",
                                            tool === rt ? "border-accent bg-accent/10" : "border-border/50 hover:border-accent/50",
                                            !isOnline && "opacity-50",
                                        )}
                                        onClick={() => setTool(rt)}
                                    >
                                        <span className={cn("w-2 h-2 rounded-full shrink-0", isOnline ? "bg-green-500" : "bg-muted-foreground/40")} />
                                        <span className="text-xs text-primary font-medium">{rt === "opencode" ? "OpenCode" : rt.charAt(0).toUpperCase() + rt.slice(1)}</span>
                                        {isOnline && runtime?.version && <span className="text-[10px] text-muted-foreground">{runtime.version}</span>}
                                    </button>
                                );
                            })}
                        </div>
                        <FieldRow label="Custom Command" className="mt-3">
                            <input className={inputCls} placeholder="Override default launch command..." value={customCmd} onChange={(e) => setCustomCmd(e.target.value)} />
                        </FieldRow>
                    </Section>
                )}

                {!isCreating && worker && (
                    <Section title="Runtime">
                        <FieldRow label="Tool">
                            <span className="text-sm text-secondary">{worker.tool}</span>
                        </FieldRow>
                    </Section>
                )}

                <Section title="Performance">
                    {isCreating && (
                        <div className="flex gap-2 mb-3">
                            <button className={presetBtnCls("standard")} onClick={() => { setConcurrency(3); setTimeout_(300); setMaxRetries(3); }}>Standard</button>
                            <button className={presetBtnCls("quick")} onClick={() => { setConcurrency(1); setTimeout_(120); setMaxRetries(1); }}>Quick</button>
                            <button className={presetBtnCls("power")} onClick={() => { setConcurrency(5); setTimeout_(600); setMaxRetries(5); }}>Power</button>
                        </div>
                    )}
                    <div className="flex gap-4">
                        <FieldRow label="Concurrency">
                            <input type="number" className={inputSmCls} value={concurrency} onChange={(e) => setConcurrency(Number(e.target.value))} />
                        </FieldRow>
                        <FieldRow label="Timeout">
                            <div className="flex items-center gap-1">
                                <input type="number" className={inputSmCls} value={timeout} onChange={(e) => setTimeout_(Number(e.target.value))} />
                                <span className="text-xs text-muted-foreground">s</span>
                            </div>
                        </FieldRow>
                        <FieldRow label="Retries">
                            <input type="number" className={inputSmCls} value={maxRetries} onChange={(e) => setMaxRetries(Number(e.target.value))} />
                        </FieldRow>
                    </div>
                </Section>

                <Section title="Capabilities">
                    <div className="flex flex-wrap gap-1.5">
                        {CAPABILITIES.map((cap) => (
                            <button key={cap}
                                className={cn(
                                    "px-2.5 py-1 text-[11px] rounded-full border cursor-pointer transition-colors",
                                    capabilities.includes(cap) ? "bg-accent/20 border-accent text-accent" : "border-border/50 text-muted-foreground hover:border-accent/50",
                                )}
                                onClick={() => setCapabilities((prev) => prev.includes(cap) ? prev.filter((c) => c !== cap) : [...prev, cap])}
                            >{cap}</button>
                        ))}
                    </div>
                </Section>

                <Section title="Agent Configuration">
                    <SoulEditor value={soul} onChange={setSoul} />
                    <div className="mt-3">
                        <TagPicker label="Skills" options={PRESET_SKILLS} selected={skills} onChange={setSkills} />
                    </div>
                    <div className="mt-3">
                        <TagPicker label="MCP Servers" options={PRESET_MCPS} selected={mcpServers} onChange={setMcpServers} />
                    </div>
                </Section>
            </div>

            <div className="flex justify-end gap-2 px-5 py-3 border-t border-border/50 shrink-0">
                <button className="px-4 py-1.5 rounded text-sm text-muted-foreground hover:text-primary cursor-pointer" onClick={onClose}>
                    {isCreating ? "Cancel" : "Close"}
                </button>
                {isCreating && (
                    <button className="px-4 py-1.5 rounded bg-accent/80 text-primary hover:bg-accent text-sm font-medium cursor-pointer disabled:opacity-50"
                        onClick={handleSubmit} disabled={submitting}>
                        {submitting ? "Creating..." : "Create Worker"}
                    </button>
                )}
            </div>
        </div>
    );
}

function SoulEditor({ value, onChange }: { value: string; onChange: (v: string) => void }) {
    const [mode, setMode] = React.useState<"source" | "preview">("source");

    return (
        <div>
            <div className="flex items-center justify-between mb-1">
                <div className="flex items-center gap-1.5">
                    <span className="text-[11px] text-muted-foreground">Soul (system prompt)</span>
                    <span className="text-[9px] text-muted-foreground/60 bg-muted/30 px-1 rounded">SOUL.md</span>
                </div>
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
                    className="w-full bg-base border border-border/50 rounded text-sm text-primary focus:outline-none focus:ring-1 focus:ring-accent px-2.5 py-1.5 font-mono resize-y"
                    rows={8} value={value} onChange={(e) => onChange(e.target.value)}
                    placeholder="# System Prompt\n\nYou are a senior engineer..."
                />
            ) : (
                <div className="border border-border/50 rounded p-3 max-h-[260px] overflow-auto bg-base">
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

function TagPicker({ label, options, selected, onChange }: {
    label: string;
    options: string[];
    selected: string[];
    onChange: (v: string[]) => void;
}) {
    const [custom, setCustom] = React.useState("");

    const toggle = (item: string) => {
        onChange(selected.includes(item) ? selected.filter((s) => s !== item) : [...selected, item]);
    };

    const addCustom = () => {
        const trimmed = custom.trim();
        if (trimmed && !selected.includes(trimmed)) {
            onChange([...selected, trimmed]);
        }
        setCustom("");
    };

    return (
        <div>
            <label className="text-[11px] text-muted-foreground mb-1 block">{label}</label>
            <div className="flex flex-wrap gap-1">
                {options.map((opt) => (
                    <button key={opt}
                        className={cn(
                            "px-2 py-0.5 text-[11px] rounded-full border cursor-pointer transition-colors",
                            selected.includes(opt) ? "bg-accent/20 border-accent text-accent" : "border-border/50 text-muted-foreground hover:border-accent/50",
                        )}
                        onClick={() => toggle(opt)}
                    >{opt}</button>
                ))}
                {selected.filter((s) => !options.includes(s)).map((s) => (
                    <span key={s} className="inline-flex items-center gap-0.5 px-2 py-0.5 text-[11px] rounded-full border border-accent/30 bg-accent/10 text-accent">
                        {s}
                        <button className="cursor-pointer hover:text-red-400" onClick={() => toggle(s)}>×</button>
                    </span>
                ))}
            </div>
            <div className="flex gap-1 mt-1">
                <input className="flex-1 bg-base border border-border/50 rounded text-[11px] text-primary focus:outline-none focus:ring-1 focus:ring-accent px-2 py-1"
                    placeholder="Add custom..." value={custom} onChange={(e) => setCustom(e.target.value)}
                    onKeyDown={(e) => e.key === "Enter" && (e.preventDefault(), addCustom())} />
                <button className="px-2 py-1 text-[11px] rounded border border-border/50 text-muted-foreground hover:text-primary hover:border-accent/50 cursor-pointer"
                    onClick={addCustom}>+</button>
            </div>
        </div>
    );
}
