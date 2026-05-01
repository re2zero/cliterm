// Copyright 2026, Command Zone Inc.
// SPDX-License-Identifier: Apache-2.0

import * as React from "react";
import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";
import { cn } from "@/util/util";

export interface MemberFormData {
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

interface MemberListProps {
    members: TeamWorker[];
    projects: TeamProject[];
    selectedMemberId: string | null;
    onSelectMember: (memberId: string | null) => void;
    onEditMember: (memberId: string) => void;
    onDeleteMember: (memberId: string) => void;
    onNewMember: () => void;
    onNewProject: () => void;
    onEditProject: (projectId: string) => void;
    onDeleteProject: (projectId: string) => void;
}

export function MemberList({ members, projects, selectedMemberId, onSelectMember, onEditMember, onDeleteMember, onNewMember, onNewProject, onEditProject, onDeleteProject }: MemberListProps) {
    const [deleteTarget, setDeleteTarget] = React.useState<TeamWorker | null>(null);
    const [deleteProjectTarget, setDeleteProjectTarget] = React.useState<TeamProject | null>(null);

    const handleDelete = () => {
        if (!deleteTarget) return;
        onDeleteMember(deleteTarget.workerid);
        setDeleteTarget(null);
    };

    const handleDeleteProject = () => {
        if (!deleteProjectTarget) return;
        onDeleteProject(deleteProjectTarget.projectid);
        setDeleteProjectTarget(null);
    };

    const sortedGroups = React.useMemo(() => {
        const memberMap = new Map<string, TeamWorker[]>();
        const unassigned: TeamWorker[] = [];
        for (const m of members) {
            if (m.projectid) {
                if (!memberMap.has(m.projectid)) memberMap.set(m.projectid, []);
                memberMap.get(m.projectid)!.push(m);
            } else {
                unassigned.push(m);
            }
        }
        const result: { project: TeamProject | null; members: TeamWorker[] }[] = [];
        for (const p of projects) {
            result.push({ project: p, members: memberMap.get(p.projectid) ?? [] });
        }
        for (const [pid, grp] of memberMap) {
            if (!projects.some((p) => p.projectid === pid)) {
                result.push({ project: { projectid: pid, name: pid, path: "", spec: "", createdat: 0, updatedat: 0 }, members: grp });
            }
        }
        if (unassigned.length > 0 || result.length === 0) {
            result.push({ project: null, members: unassigned });
        }
        return result;
    }, [members, projects]);

    return (
        <div className="w-[180px] min-w-[160px] border-r border-border/50 flex flex-col bg-card/50">
            <div className="flex items-center justify-between px-2.5 py-2 border-b border-border/50">
                <span className="text-[11px] font-semibold text-primary uppercase tracking-wider">Members</span>
                <span className="text-[10px] text-muted-foreground tabular-nums">{members.length}</span>
            </div>
            <div className="flex-1 overflow-y-auto py-0.5">
                {sortedGroups.map((group) => {
                    const project = group.project;
                    const projectId = project?.projectid ?? null;
                    return (
                        <ProjectGroup
                            key={projectId ?? "default"}
                            project={project}
                            members={group.members}
                            selectedMemberId={selectedMemberId}
                            onSelectMember={onSelectMember}
                            onEditMember={onEditMember}
                            onDeleteMember={(id) => setDeleteTarget(group.members.find((m) => m.workerid === id) ?? null)}
                            onNewMember={onNewMember}
                            onEditProject={projectId ? () => onEditProject(projectId) : undefined}
                            onDeleteProject={projectId ? () => setDeleteProjectTarget(project!) : undefined}
                            onNewProject={onNewProject}
                        />
                    );
                })}
                {members.length === 0 && sortedGroups.length === 0 && (
                    <div className="px-2 py-6 text-[10px] text-muted-foreground text-center leading-relaxed">
                        No members<br />Create one ↓
                    </div>
                )}
            </div>
            <div className="border-t border-border/50 p-1.5 flex gap-1">
                <button
                    className="flex-1 px-2 py-1.5 rounded text-[11px] bg-accent/80 text-primary hover:bg-accent cursor-pointer font-medium transition-colors"
                    onClick={onNewMember}
                >
                    + Member
                </button>
                <button
                    className="flex-1 px-2 py-1.5 rounded text-[11px] bg-blue-500/80 text-primary hover:bg-blue-500 cursor-pointer font-medium transition-colors"
                    onClick={onNewProject}
                >
                    + Project
                </button>
            </div>

            {deleteTarget && (
                <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50"
                    onClick={(e) => e.target === e.currentTarget && setDeleteTarget(null)}>
                    <div className="bg-card border border-border/50 rounded-lg shadow-2xl p-4 w-72" style={{ colorScheme: "dark" }}>
                        <h3 className="text-sm font-semibold text-primary mb-1.5">Delete Member</h3>
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

            {deleteProjectTarget && (
                <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50"
                    onClick={(e) => e.target === e.currentTarget && setDeleteProjectTarget(null)}>
                    <div className="bg-card border border-border/50 rounded-lg shadow-2xl p-4 w-72" style={{ colorScheme: "dark" }}>
                        <h3 className="text-sm font-semibold text-primary mb-1.5">Delete Project</h3>
                        <p className="text-xs text-secondary mb-1">
                            Delete <span className="text-primary font-medium">{deleteProjectTarget.name}</span>?
                        </p>
                        <p className="text-[11px] text-red-400 mb-3">This will unassign all members from this project.</p>
                        <div className="flex justify-end gap-2">
                            <button className="px-2.5 py-1 rounded text-xs text-muted-foreground hover:text-primary cursor-pointer" onClick={() => setDeleteProjectTarget(null)}>Cancel</button>
                            <button className="px-2.5 py-1 rounded bg-red-500/80 text-white hover:bg-red-500 text-xs font-medium cursor-pointer" onClick={handleDeleteProject}>Delete</button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}

function MemberItem({ member, isSelected, onSelect, onEdit, onDelete }: {
    member: TeamWorker;
    isSelected: boolean;
    onSelect: () => void;
    onEdit: () => void;
    onDelete: () => void;
}) {
    const [menuOpen, setMenuOpen] = React.useState(false);
    const menuRef = React.useRef<HTMLDivElement>(null);
    const status = STATUS_ICON[member.status] ?? STATUS_ICON["offline"];

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
            className={cn(
                "flex items-center gap-1.5 px-2 py-1.5 cursor-pointer transition-colors group",
                isSelected ? "bg-accent/10" : "hover:bg-accent/5",
            )}
            onClick={onSelect}
        >
            <span className={cn("w-1.5 h-1.5 rounded-full shrink-0", status.dot)} title={status.label} />
            <div className="flex-1 min-w-0">
                <div className="text-[11px] text-primary truncate leading-tight">{member.name}</div>
                <div className="text-[9px] text-muted-foreground">{member.status}</div>
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

function ProjectGroup({ project, members, selectedMemberId, onSelectMember, onEditMember, onDeleteMember, onNewMember, onEditProject, onDeleteProject, onNewProject }: {
    project: TeamProject | null;
    members: TeamWorker[];
    selectedMemberId: string | null;
    onSelectMember: (memberId: string | null) => void;
    onEditMember: (memberId: string) => void;
    onDeleteMember: (memberId: string) => void;
    onNewMember: () => void;
    onEditProject?: () => void;
    onDeleteProject?: () => void;
    onNewProject: () => void;
}) {
    const [menuOpen, setMenuOpen] = React.useState(false);
    const menuRef = React.useRef<HTMLDivElement>(null);
    const isDefault = !project;

    React.useEffect(() => {
        if (!menuOpen) return;
        const handler = (e: MouseEvent) => {
            if (menuRef.current && !menuRef.current.contains(e.target as Node)) setMenuOpen(false);
        };
        document.addEventListener("mousedown", handler);
        return () => document.removeEventListener("mousedown", handler);
    }, [menuOpen]);

    return (
        <div className="mb-1">
            <div className={cn(
                "flex items-center gap-1.5 px-2 py-1 border-b border-border/20 group",
                isDefault ? "bg-muted/20" : "bg-blue-500/5",
            )}>
                <div className="flex-1 min-w-0">
                    <div className="text-[11px] font-medium text-primary truncate">
                        {isDefault ? "Default" : project.name}
                    </div>
                    {!isDefault && project.path && (
                        <div className="text-[9px] text-muted-foreground truncate font-mono">
                            {project.path}
                        </div>
                    )}
                </div>
                <span className="text-[10px] text-muted-foreground tabular-nums">{members.length}</span>
                {!isDefault && (
                    <div className="relative" ref={menuRef}>
                        <button
                            className="p-0.5 rounded text-muted-foreground hover:text-primary cursor-pointer opacity-0 group-hover:opacity-100 transition-opacity"
                            onClick={(e) => { e.stopPropagation(); setMenuOpen(!menuOpen); }}
                        >⋮</button>
                        {menuOpen && (
                            <div className="absolute right-0 top-full mt-0.5 bg-card border border-border/50 rounded shadow-lg z-20 py-0.5 min-w-[72px]">
                                <button className="w-full text-left px-2 py-1 text-[11px] text-primary hover:bg-accent/10 cursor-pointer"
                                    onClick={(e) => { e.stopPropagation(); setMenuOpen(false); onEditProject?.(); }}>Edit</button>
                                <button className="w-full text-left px-2 py-1 text-[11px] text-red-400 hover:bg-red-500/10 cursor-pointer"
                                    onClick={(e) => { e.stopPropagation(); setMenuOpen(false); onDeleteProject?.(); }}>Delete</button>
                            </div>
                        )}
                    </div>
                )}
                <button
                    className="px-1.5 py-0.5 rounded text-[10px] bg-accent/80 text-primary hover:bg-accent cursor-pointer opacity-0 group-hover:opacity-100 transition-opacity font-medium"
                    onClick={onNewMember}
                >+</button>
            </div>
            <div>
                {members.map((m) => (
                    <MemberItem
                        key={m.workerid}
                        member={m}
                        isSelected={selectedMemberId === m.workerid}
                        onSelect={() => onSelectMember(m.workerid)}
                        onEdit={() => onEditMember(m.workerid)}
                        onDelete={() => onDeleteMember(m.workerid)}
                    />
                ))}
                {members.length === 0 && (
                    <div className="px-2 py-3 text-[10px] text-muted-foreground text-center italic">
                        No members
                    </div>
                )}
            </div>
        </div>
    );
}

export interface MemberDetailPanelProps {
    member: TeamWorker;
    allTasks: TeamTask[];
    onClose: () => void;
    onEdit: () => void;
    onTaskClick: (task: TeamTask) => void;
}

const TASK_STATUS_LABEL: Record<string, string> = {
    pending: "Pending",
    assigned: "Assigned",
    working: "Working",
    done: "Done",
    failed: "Failed",
    paused: "Paused",
};

const TASK_STATUS_COLOR: Record<string, string> = {
    pending: "bg-muted-foreground/30",
    assigned: "bg-blue-400",
    working: "bg-yellow-400",
    done: "bg-green-500",
    failed: "bg-red-500",
    paused: "bg-orange-400",
};

export function MemberDetailPanel({ member, allTasks, onClose, onEdit, onTaskClick }: MemberDetailPanelProps) {
    const status = STATUS_ICON[member.status] ?? STATUS_ICON["offline"];
    const memberTasks = allTasks.filter((t) => t.assignedworkerid === member.workerid);

    const activeTasks = memberTasks.filter((t) => t.status === "working" || t.status === "assigned");
    const historyTasks = memberTasks.filter((t) => t.status === "done" || t.status === "failed");
    const pendingTasks = memberTasks.filter((t) => t.status === "pending" || t.status === "paused");

    return (
        <div className="w-[320px] min-w-[280px] border-l border-border/50 flex flex-col bg-card overflow-hidden" style={{ colorScheme: "dark" }}>
            <div className="flex items-center justify-between px-4 py-3 border-b border-border/50">
                <div className="flex items-center gap-2">
                    <span className={cn("w-2 h-2 rounded-full", status.dot)} />
                    <h3 className="text-sm font-semibold text-primary">{member.name}</h3>
                </div>
                <div className="flex items-center gap-2">
                    <button className="text-[11px] text-muted-foreground hover:text-primary cursor-pointer" onClick={onEdit}>Edit</button>
                    <button className="text-muted-foreground hover:text-primary cursor-pointer text-sm" onClick={onClose}>✕</button>
                </div>
            </div>

                <div className="flex-1 overflow-y-auto">
                    <div className="px-4 py-3 border-b border-border/30 space-y-2">
                        <div className="flex items-center gap-3 text-xs">
                            <span className={cn("px-1.5 py-0.5 rounded text-[10px] font-medium", member.status === "idle" ? "bg-green-500/10 text-green-400" : member.status === "working" ? "bg-yellow-500/10 text-yellow-400" : "bg-muted text-muted-foreground")}>
                                {status.label}
                            </span>
                            <span className="text-muted-foreground">Member</span>
                        </div>
                    </div>

                {activeTasks.length > 0 && (
                    <div className="px-4 py-3 border-b border-border/30">
                        <h4 className="text-[10px] text-muted-foreground uppercase tracking-wider mb-2">Active ({activeTasks.length})</h4>
                        <div className="space-y-1">
                            {activeTasks.map((t) => (
                                <button key={t.taskid} className="w-full text-left px-2 py-1.5 rounded hover:bg-accent/10 cursor-pointer transition-colors" onClick={() => onTaskClick(t)}>
                                    <div className="flex items-center gap-1.5">
                                        <span className={cn("w-1.5 h-1.5 rounded-full shrink-0", TASK_STATUS_COLOR[t.status] ?? "bg-muted-foreground/30")} />
                                        <span className="text-xs text-primary truncate">{t.title}</span>
                                    </div>
                                </button>
                            ))}
                        </div>
                    </div>
                )}

                {pendingTasks.length > 0 && (
                    <div className="px-4 py-3 border-b border-border/30">
                        <h4 className="text-[10px] text-muted-foreground uppercase tracking-wider mb-2">Queued ({pendingTasks.length})</h4>
                        <div className="space-y-1">
                            {pendingTasks.map((t) => (
                                <button key={t.taskid} className="w-full text-left px-2 py-1.5 rounded hover:bg-accent/10 cursor-pointer transition-colors" onClick={() => onTaskClick(t)}>
                                    <div className="flex items-center gap-1.5">
                                        <span className={cn("w-1.5 h-1.5 rounded-full shrink-0", TASK_STATUS_COLOR[t.status] ?? "bg-muted-foreground/30")} />
                                        <span className="text-xs text-primary truncate">{t.title}</span>
                                    </div>
                                </button>
                            ))}
                        </div>
                    </div>
                )}

                <div className="px-4 py-3">
                    <h4 className="text-[10px] text-muted-foreground uppercase tracking-wider mb-2">History ({historyTasks.length})</h4>
                    {historyTasks.length === 0 ? (
                        <p className="text-[11px] text-muted-foreground italic">No completed tasks yet</p>
                    ) : (
                        <div className="space-y-1">
                            {historyTasks.map((t) => (
                                <button key={t.taskid} className="w-full text-left px-2 py-1.5 rounded hover:bg-accent/10 cursor-pointer transition-colors" onClick={() => onTaskClick(t)}>
                                    <div className="flex items-center gap-1.5">
                                        <span className={cn("w-1.5 h-1.5 rounded-full shrink-0", TASK_STATUS_COLOR[t.status] ?? "bg-muted-foreground/30")} />
                                        <span className="text-xs text-primary truncate">{t.title}</span>
                                    </div>
                                </button>
                            ))}
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
}

interface MemberEditorProps {
    member?: TeamWorker;
    members: TeamWorker[];
    onClose: () => void;
    onSubmit: (tool: string, config: MemberFormData) => Promise<string | void>;
}

export function MemberEditor({ member, onClose, onSubmit }: MemberEditorProps) {
    const isCreating = !member;
    const [submitting, setSubmitting] = React.useState(false);
    const [runtimes, setRuntimes] = React.useState<AIRuntime[]>([]);
    const [nameDirty, setNameDirty] = React.useState(false);

    const [tool, setTool] = React.useState("claude");
    const [name, setName] = React.useState(member?.name ?? "");
    const [description, setDescription] = React.useState("");
    const [persona, setPersona] = React.useState("");
    const [customCmd, setCustomCmd] = React.useState("");
    const [model, setModel] = React.useState("");
    const [maxRetries, setMaxRetries] = React.useState(3);
    const [capabilities, setCapabilities] = React.useState<string[]>([]);
    const [skills, setSkills] = React.useState<string[]>([]);
    const [mcpServers, setMcpServers] = React.useState<string[]>([]);

    React.useEffect(() => {
        RpcApi.TeamDetectRuntimesCommand(TabRpcClient)
            .then((r) => setRuntimes(r.runtimes))
            .catch(() => {});
    }, []);

    React.useEffect(() => {
        if (isCreating && !nameDirty) {
            setName(tool === "opencode" ? "OpenCode Member" : `${tool.charAt(0).toUpperCase() + tool.slice(1)} Member`);
        }
    }, [tool, isCreating, nameDirty]);

    const handleNameChange = (value: string) => {
        setNameDirty(true);
        setName(value);
    };

    const handleSubmit = async () => {
        if (submitting) return;
        setSubmitting(true);
        try {
            await onSubmit(tool, {
                name: name || undefined,
                description: description || undefined,
                persona: persona || undefined,
                customcmd: customCmd || undefined,
                maxretries: maxRetries > 0 ? maxRetries : undefined,
                capabilities: capabilities.length > 0 ? capabilities : undefined,
                skills: skills.length > 0 ? skills : undefined,
                mcpservers: mcpServers.length > 0 ? mcpServers.map((s) => ({ name: s } as TeamMCPConfig)) : undefined,
            });
            onClose();
        } catch (e) {
            console.error("Failed to save member:", e);
        } finally {
            setSubmitting(false);
        }
    };

    const inputCls = "w-full bg-base border border-border/50 rounded text-sm text-primary focus:outline-none focus:ring-1 focus:ring-accent px-2.5 py-1.5";

    return (
        <div className="flex flex-col h-full w-full bg-card" style={{ colorScheme: "dark" }}>
            <div className="flex items-center justify-between px-5 py-3 border-b border-border/50 shrink-0">
                <h3 className="text-sm font-semibold text-primary">
                    {isCreating ? "New Member" : member?.name ?? "Member"}
                </h3>
                <button className="text-muted-foreground hover:text-primary cursor-pointer text-sm" onClick={onClose}>✕</button>
            </div>

            <div className="flex-1 overflow-y-auto px-5 py-4 space-y-5">
                <Section title="Identity">
                    <FieldRow label="Name">
                        <input className={inputCls} value={name} onChange={(e) => handleNameChange(e.target.value)} disabled={!isCreating} />
                    </FieldRow>
                    <FieldRow label="Description">
                        <input className={inputCls} placeholder="What this member does..." value={description} onChange={(e) => setDescription(e.target.value)} />
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
                                    </button>
                                );
                            })}
                        </div>
                        <FieldRow label="Custom Command" className="mt-2">
                            <input className={cn(inputCls, "font-mono text-xs")} placeholder="Override default launch command..." value={customCmd} onChange={(e) => setCustomCmd(e.target.value)} />
                        </FieldRow>
                    </Section>
                )}

                {!isCreating && member && (
                    <Section title="Runtime">
                        <FieldRow label="Status">
                            <span className={cn(
                                "px-1.5 py-0.5 rounded text-[10px] font-medium",
                                member.status === "idle" ? "bg-green-500/10 text-green-400" :
                                member.status === "working" ? "bg-yellow-500/10 text-yellow-400" :
                                "bg-muted text-muted-foreground",
                            )}>{STATUS_ICON[member.status]?.label ?? member.status}</span>
                        </FieldRow>
                    </Section>
                )}

                <Section title="Persona">
                    <textarea
                        className={cn(inputCls, "font-mono resize-y min-h-[120px]")}
                        rows={6} value={persona} onChange={(e) => setPersona(e.target.value)}
                        placeholder="System prompt for this member. Define its role, expertise, and behavior..."
                    />
                </Section>

                <Section title="Capabilities & Skills">
                    <div className="flex flex-wrap gap-1.5 mb-3">
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
                    <TagPicker label="Skills" options={PRESET_SKILLS} selected={skills} onChange={setSkills} />
                    <div className="mt-3">
                        <TagPicker label="MCP Servers" options={PRESET_MCPS} selected={mcpServers} onChange={setMcpServers} />
                    </div>
                </Section>

                <Section title="Advanced">
                    <div className="flex gap-4">
                        <FieldRow label="Retries">
                            <input type="number" min={0} max={10} className="w-16 bg-base border border-border/50 rounded text-sm text-primary text-center px-1.5 py-1 focus:outline-none focus:ring-1 focus:ring-accent" value={maxRetries} onChange={(e) => setMaxRetries(Number(e.target.value))} />
                        </FieldRow>
                        <FieldRow label="Model Override">
                            <input className={cn(inputCls, "font-mono text-xs")} placeholder="default" value={model} onChange={(e) => setModel(e.target.value)} />
                        </FieldRow>
                    </div>
                </Section>
            </div>

            <div className="flex justify-end gap-2 px-5 py-3 border-t border-border/50 shrink-0">
                <button className="px-4 py-1.5 rounded text-sm text-muted-foreground hover:text-primary cursor-pointer" onClick={onClose}>
                    Cancel
                </button>
                <button className="px-4 py-1.5 rounded bg-accent/80 text-primary hover:bg-accent text-sm font-medium cursor-pointer disabled:opacity-50"
                    onClick={handleSubmit} disabled={submitting}>
                    {submitting ? (isCreating ? "Creating..." : "Saving...") : (isCreating ? "Create" : "Save")}
                </button>
            </div>
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
