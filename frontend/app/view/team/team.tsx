// Copyright 2026, Command Zone Inc.
// SPDX-License-Identifier: Apache-2.0

import * as jotai from "jotai";
import * as React from "react";
import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";
import { TeamViewModel } from "./team-model";
import { BoardView } from "./board-view";
import { TaskDetail } from "./task-detail";
import { MemberList, MemberEditor, MemberDetailPanel } from "./member-panel";
import { ProjectDialog } from "./project-dialog";
import type { MemberFormData } from "./member-panel";
import { cn } from "@/util/util";

interface TeamViewProps {
    blockId: string;
    blockRef: React.RefObject<HTMLDivElement>;
    contentRef: React.RefObject<HTMLDivElement>;
    model: TeamViewModel;
}

type MemberEditorTarget = { type: "new"; projectId?: string | null } | { type: "edit"; memberId: string };

export function TeamView({ model }: TeamViewProps) {
    React.useEffect(() => {
        model.init();
        return () => model.dispose();
    }, [model]);

    const pendingTasks = jotai.useAtomValue(model.pendingTasksAtom) ?? [];
    const workingTasks = jotai.useAtomValue(model.workingTasksAtom) ?? [];
    const doneTasks = jotai.useAtomValue(model.doneTasksAtom) ?? [];
    const failedTasks = jotai.useAtomValue(model.failedTasksAtom) ?? [];
    const allTasks = [...pendingTasks, ...workingTasks, ...doneTasks, ...failedTasks];
    const runtimeMembers = jotai.useAtomValue(model.runtimeMembersAtom) ?? [];
    const activities = jotai.useAtomValue(model.activityLogAtom) ?? [];
    const status = jotai.useAtomValue(model.statusAtom);
    const isSupervising = jotai.useAtomValue(model.isSupervisingAtom) ?? false;
    const isProcessing = jotai.useAtomValue(model.isProcessingAtom) ?? false;
    const error = jotai.useAtomValue(model.errorAtom) ?? null;
    const projects = jotai.useAtomValue(model.projectsAtom) ?? [];
    const templates = jotai.useAtomValue(model.templatesAtom) ?? [];
    const allMembers = jotai.useAtomValue(model.membersAtom) ?? [];

    const [selectedTask, setSelectedTask] = React.useState<TeamTask | null>(null);
    const [selectedMemberId, setSelectedMemberId] = React.useState<string | null>(null);
    const [showActivity, setShowActivity] = React.useState(false);
    const [editorTarget, setEditorTarget] = React.useState<MemberEditorTarget | null>(null);
    const [editorVisible, setEditorVisible] = React.useState(false);
    const [showProjectDialog, setShowProjectDialog] = React.useState(false);
    const [projectVisible, setProjectVisible] = React.useState(false);
    const [editingProject, setEditingProject] = React.useState<TeamProject | undefined>(undefined);

    const openProjectDialog = (project?: TeamProject) => {
        setEditingProject(project);
        setShowProjectDialog(true);
        requestAnimationFrame(() => setProjectVisible(true));
    };

    const closeProjectDialog = () => {
        setProjectVisible(false);
        setTimeout(() => { setShowProjectDialog(false); setEditingProject(undefined); }, 300);
    };
    const [newTaskTitle, setNewTaskTitle] = React.useState("");

    const editorMember = editorTarget?.type === "edit" ? runtimeMembers.find((w) => w.workerid === editorTarget.memberId) : undefined;
    const editorTemplate = editorMember ? allMembers.find((t) => t.memberid === editorMember.memberid) : undefined;

    const openEditor = (target: MemberEditorTarget) => {
        setEditorTarget(target);
        requestAnimationFrame(() => setEditorVisible(true));
    };

    const closeEditor = () => {
        setEditorVisible(false);
        setTimeout(() => setEditorTarget(null), 300);
    };

    const handleRefresh = () => { model.refreshAllData(); };
    const toggleSupervision = () => {
        if (isSupervising) model.stopSupervision();
        else model.startSupervision();
    };
    const handleTaskClick = (task: TeamTask) => { setSelectedTask(task); setSelectedMemberId(null); };
    const handleMemberClick = (memberId: string) => { setSelectedMemberId(memberId); setSelectedTask(null); };
    const handleRetryTask = (taskId: string) => { model.retryTask(taskId); };

    const handleProjectSubmit = async (data: { name: string; path: string; spec: string }) => {
        if (editingProject) {
            await model.updateProject(editingProject.projectid, data);
        } else {
            await model.createProject(data);
        }
        closeProjectDialog();
    };

    const handleDropMember = async (memberId: string, projectId: string | null) => {
        if (projectId) {
            await model.assignMemberToProject(memberId, projectId);
        } else {
            await model.assignMemberToProject(memberId, "");
        }
    };

    const handleCreateTask = async () => {
        const title = newTaskTitle.trim();
        if (!title) return;
        await model.createTask(title, "", "medium");
        setNewTaskTitle("");
    };

    const selectedMember = selectedMemberId ? runtimeMembers.find((w) => w.workerid === selectedMemberId) : null;

    return (
        <div className="flex flex-col h-full overflow-hidden bg-[#0a0a0f]" style={{ colorScheme: "dark" }}>
            <div className="flex items-center justify-between px-3 py-2 border-b border-[#1e1e2e] bg-[#12121a] h-12 shrink-0">
                <div className="flex items-center gap-3">
                    <h2 className="text-sm font-semibold text-slate-200">Team</h2>
                    <div className="flex items-center gap-2 text-[11px]">
                        <span className="flex items-center gap-1.5 px-2 py-0.5 rounded bg-white/[0.02] border border-white/[0.04] text-slate-400">
                            <span className="w-1.5 h-1.5 rounded-full bg-green-400"></span>
                            {runtimeMembers.filter((m) => m.status === "idle").length}/{status?.totalmembers ?? 0} members
                        </span>
                        <span className="flex items-center gap-1.5 px-2 py-0.5 rounded bg-white/[0.02] border border-white/[0.04] text-slate-400">
                            <span className="w-1.5 h-1.5 rounded-full bg-amber-400"></span>
                            {workingTasks.length} working
                        </span>
                        <span className="flex items-center gap-1.5 px-2 py-0.5 rounded bg-white/[0.02] border border-white/[0.04] text-slate-400">
                            <span className="w-1.5 h-1.5 rounded-full bg-cyan-400"></span>
                            {doneTasks.length} done
                        </span>
                    </div>
                </div>

                <div className="flex items-center gap-2">
                    <button
                        className={cn(
                            "px-3 py-1.5 rounded-md text-xs font-medium transition-all duration-150 cursor-pointer",
                            isSupervising
                                ? "bg-cyan-500/20 border border-cyan-500/30 text-cyan-400 shadow-[0_0_12px_rgba(34,211,238,0.3)]"
                                : "bg-white/[0.02] border border-white/[0.04] text-slate-400 hover:bg-cyan-500/10 hover:border-cyan-500/20 hover:text-cyan-400",
                        )}
                        onClick={toggleSupervision}
                        disabled={isProcessing}
                    >
                        {isSupervising ? "👑 Supervising" : "👑 Auto"}
                    </button>
                    <button
                        className="px-2 py-1.5 rounded-md text-slate-500 hover:text-slate-300 transition-colors duration-150 cursor-pointer text-xs"
                        onClick={handleRefresh}
                    >⟳</button>
                    {isProcessing && <span className="text-[11px] text-cyan-400 animate-pulse">Processing...</span>}
                    {error && <span className="text-[11px] text-red-400 truncate max-w-[200px]">{error}</span>}
                </div>
            </div>

            <div className="flex-1 flex min-h-0 overflow-hidden">
                <div className="w-[200px] shrink-0 border-r border-[#1e1e2e] bg-[#0a0a0f]">
                    <MemberList
                        members={runtimeMembers}
                        projects={projects}
                        templates={templates}
                        selectedMemberId={selectedMemberId}
                        onSelectMember={handleMemberClick}
                        onEditMember={(memberId) => { setSelectedMemberId(null); openEditor({ type: "edit", memberId }); }}
                        onDeleteMember={(memberId) => { model.deleteRuntimeMember(memberId); if (selectedMemberId === memberId) setSelectedMemberId(null); }}
                        onNewMember={(projectId) => { setSelectedMemberId(null); openEditor({ type: "new", projectId }); }}
                        onNewProject={() => openProjectDialog()}
                        onEditProject={(id) => openProjectDialog(projects.find((p) => p.projectid === id))}
                        onDeleteProject={(id) => model.deleteProject(id)}
                        onDropMember={handleDropMember}
                    />
                </div>

                <div className="flex-1 min-w-0 flex flex-col bg-[#0a0a0f]">
                    <div className="flex-1 min-h-0 overflow-y-auto px-4 py-3">
                        <BoardView
                            pendingTasks={pendingTasks}
                            workingTasks={workingTasks}
                            doneTasks={doneTasks}
                            failedTasks={failedTasks}
                            allTasks={allTasks}
                            members={runtimeMembers}
                            onTaskClick={handleTaskClick}
                            onRetryTask={handleRetryTask}
                        />
                    </div>

                    <div className="px-4 pb-3 pt-0">
                        <div className="relative">
                            <input
                                type="text"
                                className="w-full px-3 py-2 pr-10 text-sm bg-[#12121a] border border-[#1e1e2e] rounded-md text-slate-200 placeholder:text-slate-600 focus:outline-none focus:border-cyan-500/50 focus:ring-1 focus:ring-cyan-500/20 transition-all duration-150"
                                placeholder="Describe a task..."
                                value={newTaskTitle}
                                onChange={(e) => setNewTaskTitle(e.target.value)}
                                onKeyDown={(e) => e.key === "Enter" && handleCreateTask()}
                            />
                            <button
                                className={cn(
                                    "absolute right-2 top-1/2 -translate-y-1/2 p-1 rounded transition-all duration-150",
                                    newTaskTitle.trim()
                                        ? "bg-cyan-500/20 text-cyan-400 hover:bg-cyan-500/30 cursor-pointer"
                                        : "bg-white/[0.02] text-slate-600 cursor-not-allowed",
                                )}
                                onClick={handleCreateTask}
                                disabled={!newTaskTitle.trim()}
                            >
                                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
                                </svg>
                            </button>
                        </div>
                    </div>
                </div>

                <div className="w-[380px] shrink-0 border-l border-[#1e1e2e] bg-[#12121a] overflow-y-auto">
                    {selectedTask ? (
                        <TaskDetail
                            task={selectedTask}
                            workers={runtimeMembers}
                            allTasks={allTasks}
                            activities={activities}
                            onClose={() => setSelectedTask(null)}
                            onUpdate={(taskId, updates) => { model.updateTask(taskId, updates); }}
                            onExecute={(taskId, command) => { model.executeTask(taskId, command); }}
                            onPause={(taskId) => { model.pauseTask(taskId); }}
                            onResume={(taskId) => { model.resumeTask(taskId); }}
                            onRetry={(taskId) => { model.retryTask(taskId); }}
                            onDelete={async (taskId) => { await model.deleteTask(taskId); setSelectedTask(null); }}
                        />
                    ) : selectedMember ? (
                        <div className="h-full">
                            <MemberDetailPanel
                                member={selectedMember}
                                allTasks={allTasks}
                                onClose={() => setSelectedMemberId(null)}
                                onEdit={() => { setSelectedMemberId(null); openEditor({ type: "edit", memberId: selectedMember.workerid }); }}
                                onTaskClick={(task) => { setSelectedMemberId(null); setSelectedTask(task); }}
                            />
                        </div>
                    ) : (
                        <div className="h-full flex items-center justify-center text-slate-600 text-sm">
                            Select a task or member to view details
                        </div>
                    )}
                </div>

                {editorTarget && (
                    <div className={cn(
                        "absolute top-0 bottom-0 left-[200px] right-[380px] flex z-10 transition-transform duration-300 ease-in-out bg-[#0a0a0f]",
                        editorVisible ? "translate-x-0" : "-translate-x-full",
                    )}>
                        <MemberEditor
                            member={editorMember}
                            templateMember={editorTemplate}
                            members={runtimeMembers}
                            templates={templates}
                            defaultProjectId={editorTarget.type === "new" ? editorTarget.projectId ?? null : null}
                            onClose={closeEditor}
                            onSubmit={editorTarget.type === "edit"
                                ? async (_tool, config) => {
                                    if (editorMember) {
                                        await model.updateMember(editorMember.memberid, config as any);
                                        await model.updateRuntimeMember(editorTarget.memberId, { name: (config as any).name, projectid: (config as any).projectid });
                                    }
                                }
                                : async (tool, config) => { await model.createRuntimeMember(tool, config as any); }
                            }
                            onSaveTemplate={async (templateName, config) => {
                                await model.saveTemplate({
                                    name: templateName,
                                    tool: "claude",
                                    description: config.description,
                                    persona: config.persona,
                                    skills: config.skills,
                                    capabilities: config.capabilities,
                                    customcmd: config.customcmd,
                                    maxretries: config.maxretries,
                                    mcpservers: (config.mcpservers ?? []) as TeamMCPConfig[],
                                });
                            }}
                        />
                    </div>
                )}
            </div>

            <div className="border-t border-[#1e1e2e] bg-[#12121a] shrink-0">
                <button
                    className="flex items-center gap-1.5 w-full px-3 py-1.5 text-xs text-slate-500 hover:text-slate-300 transition-colors duration-150 cursor-pointer"
                    onClick={() => setShowActivity(!showActivity)}
                >
                    <span className={cn("transition-transform", showActivity && "rotate-90")}>▸</span>
                    <span className="font-mono">Activity ({activities.length})</span>
                    {activities.length > 0 && (
                        <span className="truncate max-w-[400px] text-slate-600">
                            · Last: {activities[0].description}
                        </span>
                    )}
                </button>
                {showActivity && (
                    <div className="max-h-[200px] overflow-auto px-3 pb-2 space-y-0.5 border-t border-[#1e1e2e]">
                        {activities.map((a) => (
                            <div key={a.id} className="flex gap-2 text-[11px] font-mono">
                                <span className="text-slate-600 shrink-0 tabular-nums">
                                    {new Date(a.createdat * 1000).toLocaleTimeString()}
                                </span>
                                <span className="text-cyan-400/80">[{a.type}]</span>
                                <span className="text-slate-400 truncate">{a.description}</span>
                            </div>
                        ))}
                    </div>
                )}
            </div>

            {showProjectDialog && (
                <div className={cn(
                    "absolute top-0 bottom-0 left-[200px] right-[380px] flex z-10 transition-transform duration-300 ease-in-out",
                    projectVisible ? "translate-x-0" : "-translate-x-full",
                )}>
                    <ProjectDialog
                        project={editingProject}
                        onSubmit={handleProjectSubmit}
                        onCancel={closeProjectDialog}
                    />
                </div>
            )}
        </div>
    );
}
