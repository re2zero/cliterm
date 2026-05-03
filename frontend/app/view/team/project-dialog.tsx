// Copyright 2026, Command Zone Inc.
// SPDX-License-Identifier: Apache-2.0

import * as React from "react";
import { cn } from "@/util/util";

interface ProjectDialogProps {
    project?: TeamProject;
    onSubmit: (data: { name: string; path: string; spec: string }) => Promise<void>;
    onCancel: () => void;
}

const SPECS = [
    { value: "", label: "None" },
    { value: "sdd", label: "SDD (Gentleman-AI)" },
    { value: "trellis", label: "Trellis" },
];

export function ProjectDialog({ project, onSubmit, onCancel }: ProjectDialogProps) {
    const isEditing = !!project;
    const [name, setName] = React.useState(project?.name ?? "");
    const [path, setPath] = React.useState(project?.path ?? "");
    const [spec, setSpec] = React.useState(project?.spec ?? "");
    const [submitting, setSubmitting] = React.useState(false);

    const handleSubmit = async () => {
        if (!name.trim() || !path.trim() || submitting) return;
        setSubmitting(true);
        try {
            await onSubmit({ name: name.trim(), path: path.trim(), spec });
        } finally {
            setSubmitting(false);
        }
    };

    const inputCls = "w-full bg-[#0a0a10] border border-white/10 rounded-md text-xs text-slate-200 focus:outline-none focus:ring-1 focus:ring-cyan-500/50 px-2.5 py-1.5 placeholder:text-slate-600";

    return (
        <div className="flex flex-col h-full w-full bg-[#0d0d14] border-r border-white/5" style={{ colorScheme: "dark" }}>
            <div className="flex items-center justify-between px-5 py-3 border-b border-white/5 shrink-0">
                <button className="flex items-center gap-1.5 text-slate-400 hover:text-slate-200 transition-colors cursor-pointer" onClick={onCancel}>
                    <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
                    </svg>
                    <span className="text-xs">{isEditing ? "Edit Project" : "New Project"}</span>
                </button>
            </div>

            <div className="flex-1 overflow-y-auto px-5 py-5 space-y-4">
                <div>
                    <label className="text-[10px] text-slate-500 uppercase tracking-wider font-medium mb-1 block">Name</label>
                    <input className={inputCls} placeholder="My Project" value={name} onChange={(e) => setName(e.target.value)} autoFocus />
                </div>
                <div>
                    <label className="text-[10px] text-slate-500 uppercase tracking-wider font-medium mb-1 block">Path</label>
                    <input className={cn(inputCls, "font-mono text-[10px]")} placeholder="/home/user/project" value={path} onChange={(e) => setPath(e.target.value)} />
                </div>
                <div>
                    <label className="text-[10px] text-slate-500 uppercase tracking-wider font-medium mb-1.5 block">Dev Spec</label>
                    <div className="flex gap-1.5">
                        {SPECS.map((s) => (
                            <button key={s.value}
                                className={cn(
                                    "px-2.5 py-1 text-[10px] rounded-md border cursor-pointer transition-colors font-medium",
                                    spec === s.value ? "bg-cyan-500/10 border-cyan-500/30 text-cyan-400" : "border-white/10 text-slate-500 hover:border-cyan-500/30",
                                )}
                                onClick={() => setSpec(s.value)}
                            >{s.label}</button>
                        ))}
                    </div>
                </div>
            </div>

            <div className="flex justify-end gap-2 px-5 py-3 border-t border-white/5 shrink-0">
                <button className="px-4 py-1.5 rounded-md text-xs text-slate-500 hover:text-slate-300 cursor-pointer transition-colors" onClick={onCancel}>
                    Cancel
                </button>
                <button className="px-4 py-1.5 rounded-md bg-cyan-500/10 text-cyan-400 hover:bg-cyan-500/20 text-xs font-medium cursor-pointer disabled:opacity-50 border border-cyan-500/20 transition-colors"
                    onClick={handleSubmit} disabled={!name.trim() || !path.trim() || submitting}>
                    {submitting ? "Saving..." : isEditing ? "Save" : "Create"}
                </button>
            </div>
        </div>
    );
}
