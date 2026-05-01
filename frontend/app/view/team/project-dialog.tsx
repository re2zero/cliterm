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

    const inputCls = "w-full bg-base border border-border/50 rounded text-sm text-primary focus:outline-none focus:ring-1 focus:ring-accent px-2.5 py-1.5";

    return (
        <div className="bg-card border border-border/50 shadow-2xl rounded-lg w-full max-w-sm p-5" style={{ colorScheme: "dark" }}>
            <div className="flex items-center justify-between mb-4">
                <h3 className="text-sm font-semibold text-primary">{isEditing ? "Edit Project" : "New Project"}</h3>
                <button className="text-muted-foreground hover:text-primary cursor-pointer text-sm" onClick={onCancel}>✕</button>
            </div>
            <div className="space-y-3">
                <div>
                    <label className="text-[11px] text-muted-foreground mb-0.5 block">Name</label>
                    <input className={inputCls} placeholder="My Project" value={name} onChange={(e) => setName(e.target.value)} autoFocus />
                </div>
                <div>
                    <label className="text-[11px] text-muted-foreground mb-0.5 block">Path</label>
                    <input className={cn(inputCls, "font-mono text-xs")} placeholder="/home/user/project" value={path} onChange={(e) => setPath(e.target.value)} />
                </div>
                <div>
                    <label className="text-[11px] text-muted-foreground mb-1 block">Dev Spec</label>
                    <div className="flex gap-1.5">
                        {SPECS.map((s) => (
                            <button key={s.value}
                                className={cn(
                                    "px-2.5 py-1 text-[11px] rounded-full border cursor-pointer transition-colors",
                                    spec === s.value ? "bg-accent/20 border-accent text-accent" : "border-border/50 text-muted-foreground hover:border-accent/50",
                                )}
                                onClick={() => setSpec(s.value)}
                            >{s.label}</button>
                        ))}
                    </div>
                </div>
            </div>
            <div className="flex justify-end gap-2 mt-4">
                <button className="px-3 py-1.5 rounded text-sm text-muted-foreground hover:text-primary cursor-pointer" onClick={onCancel}>Cancel</button>
                <button className="px-3 py-1.5 rounded bg-accent/80 text-primary hover:bg-accent text-sm font-medium cursor-pointer disabled:opacity-50"
                    onClick={handleSubmit} disabled={!name.trim() || !path.trim() || submitting}>
                    {submitting ? "Saving..." : isEditing ? "Save" : "Create"}
                </button>
            </div>
        </div>
    );
}
