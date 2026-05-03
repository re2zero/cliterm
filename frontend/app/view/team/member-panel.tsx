// Copyright 2026, Command Zone Inc.
// SPDX-License-Identifier: Apache-2.0

import * as React from "react";
import { Markdown } from "@/app/element/markdown";
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
    projectid?: string;
}

const STATUS_ICON: Record<string, { dot: string; label: string }> = {
    idle: { dot: "bg-green-400", label: "idle" },
    working: { dot: "bg-amber-400 animate-pulse", label: "working" },
    offline: { dot: "bg-slate-600", label: "offline" },
    error: { dot: "bg-red-400", label: "error" },
};

const TASK_STATUS_ICON: Record<string, { icon: string; color: string }> = {
    pending: { icon: "○", color: "text-slate-500" },
    assigned: { icon: "●", color: "text-cyan-400" },
    working: { icon: "●", color: "text-amber-400 animate-pulse" },
    done: { icon: "✓", color: "text-green-400" },
    failed: { icon: "✗", color: "text-red-400" },
    paused: { icon: "◼", color: "text-orange-400" },
};

const CAPABILITIES = ["frontend", "backend", "debugging", "testing", "review", "refactor"];
const RUNTIMES = ["claude", "opencode", "cursor", "aider"];
const PRESET_SKILLS = ["code-review", "git-master", "frontend-design", "qt-cpp-bug-fixer", "webapp-testing", "docx", "pdf", "xlsx"];
const PRESET_MCPS = ["filesystem", "github", "browser", "database", "fetch"];

function Section({ title, children }: { title: string; children: React.ReactNode }) {
    return (
        <div>
            <h4 className="text-[10px] uppercase tracking-widest text-slate-500 font-medium mb-2 pb-1.5 border-b border-white/5">{title}</h4>
            <div className="space-y-2">{children}</div>
        </div>
    );
}

function FieldRow({ label, children, className }: { label: string; children: React.ReactNode; className?: string }) {
    return (
        <div className={cn("flex flex-col gap-1", className)}>
            <label className="text-[10px] text-slate-400 font-medium">{label}</label>
            {children}
        </div>
    );
}

interface MemberListProps {
    members: TeamWorker[];
    projects: TeamProject[];
    templates: TeamMember[];
    selectedMemberId: string | null;
    onSelectMember: (memberId: string | null) => void;
    onEditMember: (memberId: string) => void;
    onDeleteMember: (memberId: string) => void;
    onNewMember: (projectId?: string | null) => void;
    onNewProject: () => void;
    onEditProject: (projectId: string) => void;
    onDeleteProject: (projectId: string) => void;
    onDropMember: (memberId: string, projectId: string | null) => void;
}

export function MemberList({ members, projects, selectedMemberId, onSelectMember, onEditMember, onDeleteMember, onNewMember, onNewProject, onEditProject, onDeleteProject, onDropMember }: MemberListProps) {
    const [deleteConfirm, setDeleteConfirm] = React.useState<string | null>(null);
    const [deleteProjectConfirm, setDeleteProjectConfirm] = React.useState<string | null>(null);

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
        <div className="w-[200px] min-w-[180px] border-r border-white/5 flex flex-col bg-[#0d0d14]">
            <div className="px-3 py-2 border-b border-white/5">
                <span className="text-[10px] uppercase tracking-widest text-slate-500 font-medium">Projects</span>
            </div>
            <div className="flex-1 overflow-y-auto py-1">
                {sortedGroups.map((group) => {
                    const project = group.project;
                    const projectId = project?.projectid ?? null;
                    return (
                        <ProjectGroup
                            key={projectId ?? "unassigned"}
                            project={project}
                            members={group.members}
                            selectedMemberId={selectedMemberId}
                            onSelectMember={onSelectMember}
                            onEditMember={onEditMember}
                            onDeleteMember={(id) => setDeleteConfirm(id)}
                            onNewMember={onNewMember}
                            onEditProject={projectId ? () => onEditProject(projectId) : undefined}
                            onDeleteProject={projectId ? () => setDeleteProjectConfirm(projectId) : undefined}
                            onNewProject={onNewProject}
                            onDropMember={onDropMember}
                            deleteConfirm={deleteConfirm}
                            setDeleteConfirm={setDeleteConfirm}
                            deleteProjectConfirm={deleteProjectConfirm}
                            setDeleteProjectConfirm={setDeleteProjectConfirm}
                            onDeleteMemberConfirm={onDeleteMember}
                            onDeleteProjectConfirm={onDeleteProject}
                        />
                    );
                })}
                {members.length === 0 && sortedGroups.length === 0 && (
                    <div className="px-3 py-8 text-[10px] text-slate-600 text-center leading-relaxed">
                        No members<br />Create one to begin
                    </div>
                )}
            </div>
            <div className="border-t border-white/5 p-2 flex gap-1.5">
                <button
                    className="flex-1 px-2.5 py-1.5 rounded-md text-[10px] bg-cyan-500/10 text-cyan-400 hover:bg-cyan-500/20 cursor-pointer font-medium transition-colors border border-cyan-500/20"
                    onClick={() => onNewMember()}
                >
                    + Member
                </button>
                <button
                    className="flex-1 px-2.5 py-1.5 rounded-md text-[10px] bg-cyan-500/10 text-cyan-400 hover:bg-cyan-500/20 cursor-pointer font-medium transition-colors border border-cyan-500/20"
                    onClick={onNewProject}
                >
                    + Project
                </button>
            </div>
        </div>
    );
}

function ProjectGroup({ project, members, selectedMemberId, onSelectMember, onEditMember, onDeleteMember, onNewMember, onEditProject, onDeleteProject, onNewProject, onDropMember, deleteConfirm, setDeleteConfirm, deleteProjectConfirm, setDeleteProjectConfirm, onDeleteMemberConfirm, onDeleteProjectConfirm }: {
    project: TeamProject | null;
    members: TeamWorker[];
    selectedMemberId: string | null;
    onSelectMember: (memberId: string | null) => void;
    onEditMember: (memberId: string) => void;
    onDeleteMember: (memberId: string) => void;
    onNewMember: (projectId?: string | null) => void;
    onEditProject?: () => void;
    onDeleteProject?: () => void;
    onNewProject: () => void;
    onDropMember?: (memberId: string, projectId: string | null) => void;
    deleteConfirm: string | null;
    setDeleteConfirm: (id: string | null) => void;
    deleteProjectConfirm: string | null;
    setDeleteProjectConfirm: (id: string | null) => void;
    onDeleteMemberConfirm: (memberId: string) => void;
    onDeleteProjectConfirm: (projectId: string) => void;
}) {
    const [dragOver, setDragOver] = React.useState(false);
    const [collapsed, setCollapsed] = React.useState(false);
    const isDefault = !project;

    const handleDragOver = (e: React.DragEvent) => {
        e.preventDefault();
        e.dataTransfer.dropEffect = "move";
        setDragOver(true);
    };

    const handleDragLeave = () => setDragOver(false);

    const handleDrop = (e: React.DragEvent) => {
        e.preventDefault();
        setDragOver(false);
        const memberId = e.dataTransfer.getData("text/plain");
        if (memberId && onDropMember) {
            onDropMember(memberId, project?.projectid ?? null);
        }
    };

    const projectId = project?.projectid ?? null;

    return (
        <div className={cn("mb-0.5", dragOver && "ring-1 ring-cyan-500/30 rounded-md")}
            onDragOver={handleDragOver}
            onDragLeave={handleDragLeave}
            onDrop={handleDrop}
        >
            <div className={cn(
                "flex items-center gap-1.5 px-3 py-1.5 hover:bg-white/[0.03] transition-colors",
                isDefault ? "bg-white/[0.02]" : "",
            )}>
                <button
                    className="text-slate-500 hover:text-slate-300 cursor-pointer transition-colors"
                    onClick={() => setCollapsed(!collapsed)}
                >
                    <svg className={cn("w-3 h-3 transition-transform", collapsed ? "-rotate-90" : "")} fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                    </svg>
                </button>
                <div className="flex-1 min-w-0">
                    <div className={cn("text-xs truncate", isDefault ? "text-slate-400" : "text-slate-200")}>
                        {isDefault ? "Unassigned" : project.name}
                    </div>
                </div>
                <span className="text-[10px] text-slate-500 font-mono">({members.length})</span>
                {!isDefault && (
                    <button
                        className="p-1 rounded text-slate-600 hover:text-cyan-400 hover:bg-white/[0.03] cursor-pointer transition-colors"
                        onClick={(e) => {
                            e.stopPropagation();
                            if (deleteProjectConfirm === projectId) {
                                onDeleteProjectConfirm(projectId);
                                setDeleteProjectConfirm(null);
                            } else {
                                setDeleteProjectConfirm(projectId);
                            }
                        }}
                    >
                        {deleteProjectConfirm === projectId ? (
                            <span className="text-[9px] text-red-400">Yes?</span>
                        ) : (
                            <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 12h.01M12 12h.01M19 12h.01M6 12a1 1 0 11-2 0 1 1 0 012 0zm7 0a1 1 0 11-2 0 1 1 0 012 0zm7 0a1 1 0 11-2 0 1 1 0 012 0z" />
                            </svg>
                        )}
                    </button>
                )}
                <button
                    className="px-1.5 py-0.5 rounded text-[9px] text-slate-500 hover:text-cyan-400 hover:bg-white/[0.03] cursor-pointer transition-colors font-mono"
                    onClick={() => onNewMember(project?.projectid ?? null)}
                >+</button>
            </div>
            {!collapsed && (
                <div className="py-0.5">
                    {members.map((m) => (
                        <MemberItem
                            key={m.workerid}
                            member={m}
                            isSelected={selectedMemberId === m.workerid}
                            onSelect={() => onSelectMember(m.workerid)}
                            onEdit={() => onEditMember(m.workerid)}
                            onDelete={() => onDeleteMember(m.workerid)}
                            deleteConfirm={deleteConfirm}
                            setDeleteConfirm={setDeleteConfirm}
                            onDeleteMemberConfirm={onDeleteMemberConfirm}
                        />
                    ))}
                    {members.length === 0 && (
                        <div className="px-3 py-4 text-[10px] text-slate-600 text-center italic">
                            Drop member here
                        </div>
                    )}
                </div>
            )}
        </div>
    );
}

function MemberItem({ member, isSelected, onSelect, onEdit, onDelete, deleteConfirm, setDeleteConfirm, onDeleteMemberConfirm }: {
    member: TeamWorker;
    isSelected: boolean;
    onSelect: () => void;
    onEdit: () => void;
    onDelete: (memberId: string) => void;
    deleteConfirm: string | null;
    setDeleteConfirm: (id: string | null) => void;
    onDeleteMemberConfirm: (memberId: string) => void;
}) {
    const status = STATUS_ICON[member.status] ?? STATUS_ICON["offline"];

    const handleDragStart = (e: React.DragEvent) => {
        e.dataTransfer.setData("text/plain", member.workerid);
        e.dataTransfer.effectAllowed = "move";
    };

    return (
        <div
            draggable
            onDragStart={handleDragStart}
            className={cn(
                "flex items-center gap-2 px-3 py-1.5 cursor-pointer transition-colors group relative",
                isSelected ? "bg-cyan-500/10 border-l-2 border-cyan-400 ml-2 rounded-r-md" : "hover:bg-white/[0.03] ml-2 rounded-md",
            )}
            onClick={onSelect}
        >
            <span className={cn("w-1.5 h-1.5 rounded-md shrink-0", status.dot)} />
            <div className="flex-1 min-w-0">
                <div className="text-xs text-slate-200 truncate leading-tight">{member.name}</div>
                <div className={cn("text-[10px] font-mono truncate", status.label === "working" ? "text-amber-400" : "text-slate-500")}>{status.label}</div>
            </div>
            <div className="flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
                <button
                    className="p-1 rounded text-slate-500 hover:text-cyan-400 cursor-pointer transition-colors"
                    onClick={(e) => { e.stopPropagation(); onEdit(); }}
                >
                    <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" />
                    </svg>
                </button>
                <button
                    className="p-1 rounded text-slate-500 hover:text-red-400 cursor-pointer transition-colors"
                    onClick={(e) => {
                        e.stopPropagation();
                        if (deleteConfirm === member.workerid) {
                            onDeleteMemberConfirm(member.workerid);
                            setDeleteConfirm(null);
                        } else {
                            setDeleteConfirm(member.workerid);
                        }
                    }}
                >
                    {deleteConfirm === member.workerid ? (
                        <span className="text-[9px] text-red-400">Yes?</span>
                    ) : (
                        <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                        </svg>
                    )}
                </button>
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

export function MemberDetailPanel({ member, allTasks, onClose, onEdit, onTaskClick }: MemberDetailPanelProps) {
    const status = STATUS_ICON[member.status] ?? STATUS_ICON["offline"];
    const memberTasks = allTasks.filter((t) => t.assignedworkerid === member.workerid);

    const activeTasks = memberTasks.filter((t) => t.status === "working" || t.status === "assigned");
    const historyTasks = memberTasks.filter((t) => t.status === "done" || t.status === "failed");
    const pendingTasks = memberTasks.filter((t) => t.status === "pending" || t.status === "paused");

    return (
        <div className="w-[320px] min-w-[280px] border-l border-white/5 flex flex-col bg-[#0d0d14] overflow-hidden">
            <div className="flex items-center justify-between px-4 py-3 border-b border-white/5">
                <div className="flex items-center gap-2">
                    <span className={cn("w-2 h-2 rounded-md", status.dot)} />
                    <h3 className="text-sm font-medium text-slate-200">{member.name}</h3>
                </div>
                <div className="flex items-center gap-3">
                    <button className="text-[10px] text-cyan-400 hover:text-cyan-300 cursor-pointer transition-colors" onClick={onEdit}>Edit</button>
                    <button className="text-slate-500 hover:text-slate-300 cursor-pointer transition-colors text-sm" onClick={onClose}>✕</button>
                </div>
            </div>

            <div className="px-4 py-2.5 border-b border-white/5">
                <div className="flex items-center gap-2 text-xs">
                    <span className={cn("text-[10px] font-mono", status.label === "working" ? "text-amber-400" : "text-slate-500")}>{status.label}</span>
                    <span className="text-slate-600">·</span>
                    <span className="text-slate-500">Member</span>
                </div>
            </div>

            <div className="flex-1 overflow-y-auto">
                {activeTasks.length > 0 && (
                    <div className="px-4 py-3 border-b border-white/5">
                        <h4 className="text-[10px] uppercase tracking-widest text-slate-500 font-medium mb-2">Active ({activeTasks.length})</h4>
                        <div className="space-y-0.5">
                            {activeTasks.map((t) => (
                                <button key={t.taskid} className="w-full text-left px-2 py-1.5 rounded-md hover:bg-white/[0.03] cursor-pointer transition-colors" onClick={() => onTaskClick(t)}>
                                    <div className="flex items-center gap-1.5">
                                        <span className={cn("text-xs", TASK_STATUS_ICON[t.status]?.color ?? "text-slate-500")}>{TASK_STATUS_ICON[t.status]?.icon}</span>
                                        <span className="text-xs text-slate-200 truncate">{t.title}</span>
                                    </div>
                                </button>
                            ))}
                        </div>
                    </div>
                )}

                {pendingTasks.length > 0 && (
                    <div className="px-4 py-3 border-b border-white/5">
                        <h4 className="text-[10px] uppercase tracking-widest text-slate-500 font-medium mb-2">Queued ({pendingTasks.length})</h4>
                        <div className="space-y-0.5">
                            {pendingTasks.map((t) => (
                                <button key={t.taskid} className="w-full text-left px-2 py-1.5 rounded-md hover:bg-white/[0.03] cursor-pointer transition-colors" onClick={() => onTaskClick(t)}>
                                    <div className="flex items-center gap-1.5">
                                        <span className={cn("text-xs", TASK_STATUS_ICON[t.status]?.color ?? "text-slate-500")}>{TASK_STATUS_ICON[t.status]?.icon}</span>
                                        <span className="text-xs text-slate-200 truncate">{t.title}</span>
                                    </div>
                                </button>
                            ))}
                        </div>
                    </div>
                )}

                <div className="px-4 py-3">
                    <h4 className="text-[10px] uppercase tracking-widest text-slate-500 font-medium mb-2">History ({historyTasks.length})</h4>
                    {historyTasks.length === 0 ? (
                        <p className="text-[10px] text-slate-600 italic">No completed tasks yet</p>
                    ) : (
                        <div className="space-y-0.5">
                            {historyTasks.map((t) => (
                                <button key={t.taskid} className="w-full text-left px-2 py-1.5 rounded-md hover:bg-white/[0.03] cursor-pointer transition-colors" onClick={() => onTaskClick(t)}>
                                    <div className="flex items-center gap-1.5">
                                        <span className={cn("text-xs", TASK_STATUS_ICON[t.status]?.color ?? "text-slate-500")}>{TASK_STATUS_ICON[t.status]?.icon}</span>
                                        <span className="text-xs text-slate-200 truncate">{t.title}</span>
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
    templateMember?: TeamMember;
    members: TeamWorker[];
    templates: TeamMember[];
    defaultProjectId?: string | null;
    onClose: () => void;
    onSubmit: (tool: string, config: MemberFormData) => Promise<string | void>;
    onSaveTemplate?: (name: string, config: MemberFormData) => Promise<void>;
}

export function MemberEditor({ member, templateMember, members, templates, defaultProjectId, onClose, onSubmit, onSaveTemplate }: MemberEditorProps) {
    const isCreating = !member;
    const [submitting, setSubmitting] = React.useState(false);
    const [runtimes, setRuntimes] = React.useState<AIRuntime[]>([]);
    const [nameDirty, setNameDirty] = React.useState(false);
    const [selectedTemplate, setSelectedTemplate] = React.useState<string | null>(null);

    const [tool, setTool] = React.useState(templateMember?.tool || member?.name?.includes("opencode") ? "opencode" : "claude");
    const [name, setName] = React.useState(member?.name ?? "");
    const [description, setDescription] = React.useState(templateMember?.description ?? "");
    const [persona, setPersona] = React.useState(templateMember?.persona ?? "");
    const [customCmd, setCustomCmd] = React.useState(templateMember?.customcmd ?? "");
    const [model, setModel] = React.useState(templateMember?.model ?? "");
    const [maxRetries, setMaxRetries] = React.useState(templateMember?.maxretries ?? 3);
    const [capabilities, setCapabilities] = React.useState<string[]>(templateMember?.capabilities ?? []);
    const [skills, setSkills] = React.useState<string[]>(templateMember?.skills ?? []);
    const [mcpJson, setMcpJson] = React.useState(templateMember?.mcpservers?.length ? JSON.stringify(templateMember.mcpservers, null, 2) : "[]");

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

    const applyTemplate = (t: TeamMember) => {
        setSelectedTemplate(t.name);
        setTool(t.tool || "claude");
        setDescription(t.description ?? "");
        setPersona(t.persona ?? "");
        setSkills(t.skills ?? []);
        setCapabilities(t.capabilities ?? []);
        setCustomCmd(t.customcmd ?? "");
        setMaxRetries(t.maxretries ?? 3);
        setMcpJson(t.mcpservers?.length ? JSON.stringify(t.mcpservers, null, 2) : "[]");

        const baseName = t.name ?? "member";
        const existingNames = new Set(members.map((m) => m.name));
        if (!existingNames.has(baseName)) {
            setNameDirty(true);
            setName(baseName);
        } else {
            let nextNum = 1;
            while (existingNames.has(`${baseName}-${nextNum}`)) { nextNum++; }
            setNameDirty(true);
            setName(`${baseName}-${nextNum}`);
        }
    };

    const handleSubmit = async () => {
        if (submitting) return;
        setSubmitting(true);
        try {
            let mcpServers = undefined;
            try {
                const parsed = JSON.parse(mcpJson);
                if (Array.isArray(parsed) && parsed.length > 0) {
                    mcpServers = parsed;
                }
            } catch {}

            await onSubmit(tool, {
                name: name || undefined,
                description: description || undefined,
                persona: persona || undefined,
                customcmd: customCmd || undefined,
                maxretries: maxRetries > 0 ? maxRetries : undefined,
                capabilities: capabilities.length > 0 ? capabilities : undefined,
                skills: skills.length > 0 ? skills : undefined,
                mcpservers: mcpServers,
                projectid: isCreating ? defaultProjectId ?? undefined : undefined,
            });
            onClose();
        } catch (e) {
            console.error("Failed to save member:", e);
        } finally {
            setSubmitting(false);
        }
    };

    const inputCls = "w-full bg-[#0a0a10] border border-white/10 rounded-md text-xs text-slate-200 focus:outline-none focus:ring-1 focus:ring-cyan-500/50 px-2.5 py-1.5 placeholder:text-slate-600";

    return (
        <div className="flex flex-col h-full w-full bg-[#0d0d14] relative z-20">
            <div className="flex items-center justify-between px-5 py-3 border-b border-white/5 shrink-0">
                <button className="flex items-center gap-1.5 text-slate-400 hover:text-slate-200 transition-colors cursor-pointer" onClick={onClose}>
                    <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
                    </svg>
                    <span className="text-xs">{isCreating ? "New Member" : member?.name ?? "Member"}</span>
                </button>
                {onSaveTemplate && (
                    <button
                        className="px-2.5 py-1 rounded-md text-[10px] font-medium bg-amber-500/10 text-amber-400 border border-amber-500/20 hover:bg-amber-500/20 cursor-pointer transition-colors"
                        onClick={() => {
                            const config: MemberFormData = {
                                name: name || undefined,
                                description: description || undefined,
                                persona: persona || undefined,
                                skills: skills.length > 0 ? skills : undefined,
                                capabilities: capabilities.length > 0 ? capabilities : undefined,
                                customcmd: customCmd || undefined,
                                maxretries: maxRetries > 0 ? maxRetries : undefined,
                            };
                            const mcpServers = (() => { try { const p = JSON.parse(mcpJson); return Array.isArray(p) && p.length > 0 ? p : undefined; } catch { return undefined; } })();
                            onSaveTemplate(name || "Untitled Template", { ...config, mcpservers: mcpServers });
                        }}
                    >★ Save as Template</button>
                )}
            </div>

            <div className="flex-1 overflow-y-auto px-5 py-4 space-y-5">
                {isCreating && templates.length > 0 && (
                    <Section title="Template">
                        <div className="grid grid-cols-2 gap-1.5">
                            <button
                                className={cn(
                                    "px-2 py-1.5 rounded-md border cursor-pointer transition-colors text-left",
                                    selectedTemplate === null ? "border-cyan-500/50 bg-cyan-500/10" : "border-white/10 hover:border-cyan-500/30",
                                )}
                                onClick={() => { setSelectedTemplate(null); setNameDirty(true); setName(""); setDescription(""); setPersona(""); setSkills([]); setMcpJson("[]"); setCapabilities([]); setCustomCmd(""); }}
                            >
                                <span className="text-[11px] text-slate-400">Empty</span>
                            </button>
                            {templates.map((t) => (
                                <button key={t.name}
                                    className={cn(
                                        "px-2 py-1.5 rounded-md border cursor-pointer transition-colors text-left",
                                        selectedTemplate === t.name ? "border-cyan-500/50 bg-cyan-500/10" : "border-white/10 hover:border-cyan-500/30",
                                    )}
                                    onClick={() => applyTemplate(t)}
                                >
                                    <span className="text-[11px] text-slate-200 font-medium block truncate">{t.name}</span>
                                    {t.description && <span className="text-[9px] text-slate-500 block leading-tight line-clamp-2">{t.description}</span>}
                                </button>
                            ))}
                        </div>
                    </Section>
                )}

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
                                            "flex items-center gap-2 px-3 py-2 rounded-md border cursor-pointer transition-colors",
                                            tool === rt ? "border-cyan-500/50 bg-cyan-500/10" : "border-white/10 hover:border-cyan-500/30",
                                            !isOnline && "opacity-50",
                                        )}
                                        onClick={() => setTool(rt)}
                                    >
                                        <span className={cn("w-1.5 h-1.5 rounded-md shrink-0", isOnline ? "bg-green-400" : "bg-slate-600")} />
                                        <span className="text-xs text-slate-200 font-medium">{rt === "opencode" ? "OpenCode" : rt.charAt(0).toUpperCase() + rt.slice(1)}</span>
                                    </button>
                                );
                            })}
                        </div>
                        <FieldRow label="Custom Command" className="mt-2">
                            <input className={cn(inputCls, "font-mono text-[10px]")} placeholder="Override default launch command..." value={customCmd} onChange={(e) => setCustomCmd(e.target.value)} />
                        </FieldRow>
                    </Section>
                )}

                {!isCreating && member && (
                    <Section title="Runtime">
                        <FieldRow label="Status">
                            <span className={cn(
                                "px-2 py-0.5 rounded-md text-[10px] font-mono font-medium",
                                member.status === "idle" ? "bg-green-500/10 text-green-400" :
                                member.status === "working" ? "bg-amber-500/10 text-amber-400" :
                                "bg-slate-500/10 text-slate-500",
                            )}>{STATUS_ICON[member.status]?.label ?? member.status}</span>
                        </FieldRow>
                    </Section>
                )}

                <Section title="Persona">
                    <PersonaEditor value={persona} onChange={setPersona} />
                </Section>

                <Section title="Capabilities & Skills">
                    <div className="flex flex-wrap gap-1.5 mb-3">
                        {CAPABILITIES.map((cap) => (
                            <button key={cap}
                                className={cn(
                                    "px-2.5 py-1 text-[10px] rounded-md border cursor-pointer transition-colors",
                                    capabilities.includes(cap) ? "bg-cyan-500/10 border-cyan-500/30 text-cyan-400" : "border-white/10 text-slate-500 hover:border-cyan-500/30",
                                )}
                                onClick={() => setCapabilities((prev) => prev.includes(cap) ? prev.filter((c) => c !== cap) : [...prev, cap])}
                            >{cap}</button>
                        ))}
                    </div>
                    <TagPicker label="Skills" options={PRESET_SKILLS} selected={skills} onChange={setSkills} />
                    <div className="mt-3">
                        <MCPJsonEditor value={mcpJson} onChange={setMcpJson} />
                    </div>
                </Section>

                <Section title="Advanced">
                    <div className="flex gap-4">
                        <FieldRow label="Retries">
                            <input type="number" min={0} max={10} className="w-16 bg-[#0a0a10] border border-white/10 rounded-md text-xs text-slate-200 text-center px-1.5 py-1 focus:outline-none focus:ring-1 focus:ring-cyan-500/50" value={maxRetries} onChange={(e) => setMaxRetries(Number(e.target.value))} />
                        </FieldRow>
                        <FieldRow label="Model Override">
                            <input className={cn(inputCls, "font-mono text-[10px]")} placeholder="default" value={model} onChange={(e) => setModel(e.target.value)} />
                        </FieldRow>
                    </div>
                </Section>
            </div>

            <div className="flex justify-end gap-2 px-5 py-3 border-t border-white/5 shrink-0">
                <button className="px-4 py-1.5 rounded-md text-xs text-slate-500 hover:text-slate-300 cursor-pointer transition-colors" onClick={onClose}>
                    Cancel
                </button>
                <button className="px-4 py-1.5 rounded-md bg-cyan-500/10 text-cyan-400 hover:bg-cyan-500/20 text-xs font-medium cursor-pointer disabled:opacity-50 border border-cyan-500/20 transition-colors"
                    onClick={handleSubmit} disabled={submitting}>
                    {submitting ? (isCreating ? "Creating..." : "Saving...") : (isCreating ? "Create" : "Save")}
                </button>
            </div>
        </div>
    );
}

function PersonaEditor({ value, onChange }: { value: string; onChange: (v: string) => void }) {
    const [mode, setMode] = React.useState<"source" | "preview">("source");

    return (
        <div>
            <div className="flex items-center justify-between mb-1">
                <span className="text-[10px] text-slate-400 font-medium">System Prompt</span>
                <div className="flex rounded-md border border-white/10 overflow-hidden">
                    <button
                        className={cn("px-2 py-0.5 text-[10px] cursor-pointer transition-colors", mode === "source" ? "bg-cyan-500/10 text-cyan-400" : "text-slate-500 hover:text-slate-300")}
                        onClick={() => setMode("source")}
                    >Source</button>
                    <button
                        className={cn("px-2 py-0.5 text-[10px] cursor-pointer border-l border-white/10 transition-colors", mode === "preview" ? "bg-cyan-500/10 text-cyan-400" : "text-slate-500 hover:text-slate-300")}
                        onClick={() => setMode("preview")}
                    >Preview</button>
                </div>
            </div>
            {mode === "source" ? (
                <textarea
                    className="w-full bg-[#0a0a10] border border-white/10 rounded-md text-xs text-slate-200 focus:outline-none focus:ring-1 focus:ring-cyan-500/50 px-2.5 py-1.5 font-mono resize-y placeholder:text-slate-600 min-h-[80px]"
                    rows={Math.max(3, value.split("\n").length)}
                    value={value}
                    onChange={(e) => onChange(e.target.value)}
                    placeholder="System prompt for this member. Define its role, expertise, and behavior..."
                />
            ) : (
                <div className="border border-white/10 rounded-md p-2.5 bg-[#0a0a10]">
                    {value.trim() ? (
                        <Markdown text={value} className="text-xs" scrollable={false} />
                    ) : (
                        <span className="text-xs text-slate-600 italic">No persona defined</span>
                    )}
                </div>
            )}
        </div>
    );
}

function MCPJsonEditor({ value, onChange }: { value: string; onChange: (v: string) => void }) {
    const [mode, setMode] = React.useState<"source" | "preview">("source");
    const [error, setError] = React.useState<string | null>(null);

    const validateJson = (text: string) => {
        if (!text.trim()) { setError(null); return; }
        try {
            const parsed = JSON.parse(text);
            if (!Array.isArray(parsed)) { setError("Must be a JSON array"); return; }
            setError(null);
        } catch (e) { setError("Invalid JSON"); }
    };

    React.useEffect(() => { validateJson(value); }, [value]);

    return (
        <div>
            <div className="flex items-center justify-between mb-1">
                <span className="text-[10px] text-slate-400 font-medium">MCP Servers</span>
                <div className="flex rounded-md border border-white/10 overflow-hidden">
                    <button
                        className={cn("px-2 py-0.5 text-[10px] cursor-pointer transition-colors", mode === "source" ? "bg-cyan-500/10 text-cyan-400" : "text-slate-500 hover:text-slate-300")}
                        onClick={() => setMode("source")}
                    >JSON</button>
                    <button
                        className={cn("px-2 py-0.5 text-[10px] cursor-pointer border-l border-white/10 transition-colors", mode === "preview" ? "bg-cyan-500/10 text-cyan-400" : "text-slate-500 hover:text-slate-300")}
                        onClick={() => setMode("preview")}
                    >Preview</button>
                </div>
            </div>
            {mode === "source" ? (
                <div className="min-h-[100px]">
                    <textarea
                        className="w-full bg-[#0a0a10] border border-white/10 rounded-md text-xs text-slate-200 focus:outline-none focus:ring-1 focus:ring-cyan-500/50 px-2.5 py-1.5 font-mono resize-y placeholder:text-slate-600"
                        value={value}
                        onChange={(e) => { onChange(e.target.value); validateJson(e.target.value); }}
                        placeholder='[{"name": "filesystem"}, {"name": "github", "type": "stdio", "command": "npx", "args": ["-y", "@modelcontextprotocol/server-github"]}]'
                    />
                    {error && <span className="text-[10px] text-red-400 mt-0.5 block">{error}</span>}
                </div>
            ) : (
                <div className="border border-white/10 rounded-md p-2 min-h-[60px] bg-[#0a0a10]">
                    {(() => {
                        if (!value.trim()) return <span className="text-xs text-slate-600 italic">No MCP servers configured</span>;
                        try {
                            const parsed = JSON.parse(value);
                            if (!Array.isArray(parsed)) return <span className="text-xs text-red-400">Invalid: not an array</span>;
                            if (parsed.length === 0) return <span className="text-xs text-slate-600 italic">Empty array</span>;
                            return (
                                <div className="space-y-1">
                                    {parsed.map((s: any, i: number) => (
                                        <div key={i} className="flex items-center gap-1.5">
                                            <span className="w-1 h-1 rounded-md bg-cyan-400 shrink-0" />
                                            <span className="text-xs text-slate-200">{s.name || `server-${i}`}</span>
                                            {s.type && <span className="text-[10px] text-slate-500">{s.type}</span>}
                                        </div>
                                    ))}
                                </div>
                            );
                        } catch { return <span className="text-xs text-red-400">Invalid JSON</span>; }
                    })()}
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
            <label className="text-[10px] text-slate-400 font-medium mb-1 block">{label}</label>
            <div className="flex flex-wrap gap-1">
                {options.map((opt) => (
                    <button key={opt}
                        className={cn(
                            "px-2 py-0.5 text-[10px] rounded-md border cursor-pointer transition-colors",
                            selected.includes(opt) ? "bg-cyan-500/10 border-cyan-500/30 text-cyan-400" : "border-white/10 text-slate-500 hover:border-cyan-500/30",
                        )}
                        onClick={() => toggle(opt)}
                    >{opt}</button>
                ))}
                {selected.filter((s) => !options.includes(s)).map((s) => (
                    <span key={s} className="inline-flex items-center gap-0.5 px-2 py-0.5 text-[10px] rounded-md border border-cyan-500/30 bg-cyan-500/10 text-cyan-400">
                        {s}
                        <button className="cursor-pointer hover:text-red-400" onClick={() => toggle(s)}>×</button>
                    </span>
                ))}
            </div>
            <div className="flex gap-1 mt-1">
                <input className="flex-1 bg-[#0a0a10] border border-white/10 rounded-md text-[10px] text-slate-200 focus:outline-none focus:ring-1 focus:ring-cyan-500/50 px-2 py-1 placeholder:text-slate-600"
                    placeholder="Add custom..." value={custom} onChange={(e) => setCustom(e.target.value)}
                    onKeyDown={(e) => e.key === "Enter" && (e.preventDefault(), addCustom())} />
                <button className="px-2 py-1 text-[10px] rounded-md border border-white/10 text-slate-500 hover:text-slate-300 hover:border-cyan-500/30 cursor-pointer transition-colors"
                    onClick={addCustom}>+</button>
            </div>
        </div>
    );
}
