// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import * as jotai from "jotai";
import * as React from "react";
import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";
import { CoworkViewModel } from "./cowork-model";
import { BoardView } from "./board-view";
import { StatusStrip } from "./status-strip";
import { RuntimeBar } from "./runtime-bar";
import { TaskDetail } from "./task-detail";
import { WorkerList, WorkerEditor, WorkerDetailPanel } from "./worker-panel";
import type { WorkerFormData } from "./worker-panel";
import { cn } from "@/util/util";

interface CoworkViewProps {
    blockId: string;
    blockRef: React.RefObject<HTMLDivElement>;
    contentRef: React.RefObject<HTMLDivElement>;
    model: CoworkViewModel;
}

type WorkerEditorTarget = { type: "new" } | { type: "edit"; workerId: string };

export function CoworkView({ model }: CoworkViewProps) {
    React.useEffect(() => {
        model.init();
        return () => model.dispose();
    }, [model]);

    const pendingTasks = jotai.useAtomValue(model.pendingTasksAtom) ?? [];
    const workingTasks = jotai.useAtomValue(model.workingTasksAtom) ?? [];
    const doneTasks = jotai.useAtomValue(model.doneTasksAtom) ?? [];
    const failedTasks = jotai.useAtomValue(model.failedTasksAtom) ?? [];
    const allTasks = [...pendingTasks, ...workingTasks, ...doneTasks, ...failedTasks];
    const workers = jotai.useAtomValue(model.workersAtom) ?? [];
    const activities = jotai.useAtomValue(model.activityLogAtom) ?? [];
    const status = jotai.useAtomValue(model.statusAtom);
    const isSupervising = jotai.useAtomValue(model.isSupervisingAtom) ?? false;
    const isProcessing = jotai.useAtomValue(model.isProcessingAtom) ?? false;
    const error = jotai.useAtomValue(model.errorAtom) ?? null;

    const [selectedTask, setSelectedTask] = React.useState<CoworkTask | null>(null);
    const [selectedWorkerId, setSelectedWorkerId] = React.useState<string | null>(null);
    const [showCreateTask, setShowCreateTask] = React.useState(false);
    const [showActivity, setShowActivity] = React.useState(false);
    const [editorTarget, setEditorTarget] = React.useState<WorkerEditorTarget | null>(null);
    const [editorVisible, setEditorVisible] = React.useState(false);

    const editorWorker = editorTarget?.type === "edit" ? workers.find((w) => w.workerid === editorTarget.workerId) : undefined;

    const openEditor = (target: WorkerEditorTarget) => {
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
    const handleTaskClick = (task: CoworkTask) => { setSelectedTask(task); };
    const handleRetryTask = (taskId: string) => { model.retryTask(taskId); };

    return (
        <div className="flex flex-col h-full overflow-hidden" style={{ colorScheme: "dark" }}>
            <div className="flex items-center justify-between px-4 py-2 border-b border-border/50">
                <h2 className="text-sm font-semibold text-primary">Cowork</h2>
                <div className="flex items-center gap-1.5">
                    <button
                        className={cn(
                            "px-2.5 py-1 rounded text-xs font-medium transition-colors cursor-pointer",
                            isSupervising
                                ? "bg-green-600 text-white hover:bg-green-700"
                                : "bg-accent/80 text-primary hover:bg-accent",
                        )}
                        onClick={toggleSupervision}
                        disabled={isProcessing}
                    >
                        {isSupervising ? "👑 Supervising" : "👑 Auto"}
                    </button>
                    <button
                        className="px-2 py-1 rounded text-secondary hover:text-primary transition-colors cursor-pointer text-xs"
                        onClick={handleRefresh}
                    >⟳</button>
                    {isProcessing && <span className="text-[11px] text-muted-foreground animate-pulse">Processing...</span>}
                    {error && <span className="text-[11px] text-red-400 truncate max-w-[200px]">{error}</span>}
                    <span className="w-px h-4 bg-border/50 mx-1" />
                    <button
                        className="px-2.5 py-1 rounded bg-accent/80 text-primary hover:bg-accent transition-colors cursor-pointer text-xs font-medium"
                        onClick={() => setShowCreateTask(true)}
                    >+ Task</button>
                </div>
            </div>

            <RuntimeBar />

            <StatusStrip status={status} />

            <div className="flex-1 flex min-h-0 relative overflow-hidden">
                <WorkerList
                    workers={workers}
                    selectedWorkerId={selectedWorkerId}
                    onSelectWorker={(id) => setSelectedWorkerId(id === selectedWorkerId ? null : id)}
                    onEditWorker={(workerId) => { setSelectedWorkerId(null); openEditor({ type: "edit", workerId }); }}
                    onDeleteWorker={(workerId) => { model.deleteWorker(workerId); if (selectedWorkerId === workerId) setSelectedWorkerId(null); }}
                    onNewWorker={() => { setSelectedWorkerId(null); openEditor({ type: "new" }); }}
                />

                <div className={cn(
                    "flex-1 min-w-0 flex transition-all duration-300 ease-in-out",
                    (editorVisible || selectedWorkerId) && "translate-x-full opacity-0 pointer-events-none",
                )}>
                    <div className="flex-1 min-w-0">
                        <BoardView
                            pendingTasks={pendingTasks}
                            workingTasks={workingTasks}
                            doneTasks={doneTasks}
                            failedTasks={failedTasks}
                            allTasks={allTasks}
                            workers={workers}
                            onTaskClick={handleTaskClick}
                            onRetryTask={handleRetryTask}
                        />
                    </div>
                    {selectedTask && (
                        <TaskDetail
                            task={selectedTask}
                            workers={workers}
                            allTasks={allTasks}
                            activities={activities}
                            onClose={() => setSelectedTask(null)}
                            onUpdate={(taskId, updates) => { model.updateTask(taskId, updates); }}
                            onExecute={(taskId, command) => { model.executeTask(taskId, command); }}
                            onPause={(taskId) => { model.pauseTask(taskId); }}
                            onResume={(taskId) => { model.resumeTask(taskId); }}
                            onRetry={(taskId) => { model.retryTask(taskId); }}
                            onDelete={(taskId) => { model.deleteTask(taskId); setSelectedTask(null); }}
                        />
                    )}
                </div>

                {selectedWorkerId && (() => {
                    const w = workers.find((w) => w.workerid === selectedWorkerId);
                    if (!w) return null;
                    return (
                        <div className={cn(
                            "absolute top-0 bottom-0 left-[140px] right-0 flex z-10 transition-transform duration-300 ease-in-out",
                            selectedWorkerId ? "translate-x-0" : "-translate-x-full",
                        )}>
                            <WorkerDetailPanel
                                worker={w}
                                allTasks={allTasks}
                                onClose={() => setSelectedWorkerId(null)}
                                onEdit={() => { setSelectedWorkerId(null); openEditor({ type: "edit", workerId: w.workerid }); }}
                                onTaskClick={(task) => { setSelectedWorkerId(null); setSelectedTask(task); }}
                            />
                        </div>
                    );
                })()}

                {editorTarget && (
                    <div className={cn(
                        "absolute top-0 bottom-0 left-[140px] right-0 flex z-10 transition-transform duration-300 ease-in-out",
                        editorVisible ? "translate-x-0" : "-translate-x-full",
                    )}>
                        <WorkerEditor
                            worker={editorWorker}
                            workers={workers}
                            onClose={closeEditor}
                            onSubmit={editorTarget.type === "edit"
                                ? async (_tool, config) => { await model.updateWorker(editorTarget.workerId, config as any); }
                                : async (tool, config) => { await model.createWorker(tool, config as any); }
                            }
                        />
                    </div>
                )}
            </div>

            <div className="border-t border-border/50">
                <button
                    className="flex items-center gap-1.5 w-full px-4 py-1.5 text-xs text-muted-foreground hover:text-primary transition-colors cursor-pointer"
                    onClick={() => setShowActivity(!showActivity)}
                >
                    <span className={cn("transition-transform", showActivity && "rotate-90")}>▸</span>
                    <span>Activity ({activities.length})</span>
                    {activities.length > 0 && (
                        <span className="truncate max-w-[300px]">
                            · Last: {activities[0].description}
                        </span>
                    )}
                </button>
                {showActivity && (
                    <div className="max-h-[200px] overflow-auto px-4 pb-2 space-y-0.5">
                        {activities.map((a) => (
                            <div key={a.id} className="flex gap-2 text-[11px]">
                                <span className="text-muted-foreground shrink-0 tabular-nums">
                                    {new Date(a.createdat * 1000).toLocaleTimeString()}
                                </span>
                                <span className="text-secondary">[{a.type}]</span>
                                <span className="text-primary truncate">{a.description}</span>
                            </div>
                        ))}
                    </div>
                )}
            </div>

            {showCreateTask && (
                <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50"
                    onClick={(e) => e.target === e.currentTarget && setShowCreateTask(false)}>
                    <CreateTaskInline
                        allTasks={allTasks}
                        onSubmit={async (title, desc, priority, deps) => {
                            await model.createTask(title, desc, priority, deps);
                            setShowCreateTask(false);
                        }}
                        onCancel={() => setShowCreateTask(false)}
                    />
                </div>
            )}
        </div>
    );
}

function CreateTaskInline({ allTasks, onSubmit, onCancel }: {
    allTasks: CoworkTask[];
    onSubmit: (title: string, desc: string, priority: string, deps?: string[]) => Promise<void>;
    onCancel: () => void;
}) {
    const [title, setTitle] = React.useState("");
    const [desc, setDesc] = React.useState("");
    const [priority, setPriority] = React.useState("medium");
    const [deps, setDeps] = React.useState<string[]>([]);
    const [submitting, setSubmitting] = React.useState(false);

    const handleSubmit = async () => {
        if (!title.trim() || submitting) return;
        setSubmitting(true);
        await onSubmit(title, desc, priority, deps.length > 0 ? deps : undefined);
    };

    const inputCls = "w-full bg-base border border-border/50 rounded text-sm text-primary focus:outline-none focus:ring-1 focus:ring-accent px-2.5 py-1.5";

    return (
        <div className="bg-card border border-border/50 shadow-2xl rounded-lg w-full max-w-md p-5" style={{ colorScheme: "dark" }}>
            <div className="flex items-center justify-between mb-4">
                <h3 className="text-sm font-semibold text-primary">New Task</h3>
                <button className="text-muted-foreground hover:text-primary cursor-pointer text-sm" onClick={onCancel}>✕</button>
            </div>
            <input className={inputCls} placeholder="Task title *" value={title} onChange={(e) => setTitle(e.target.value)} onKeyDown={(e) => e.key === "Enter" && handleSubmit()} autoFocus />
            <textarea className={cn(inputCls, "mt-2 resize-none")} rows={2} placeholder="Description (optional)" value={desc} onChange={(e) => setDesc(e.target.value)} />
            <div className="flex gap-2 mt-2">
                <select className="bg-base border border-border/50 rounded text-sm text-primary px-2 py-1.5" value={priority} onChange={(e) => setPriority(e.target.value)}>
                    <option value="low">Low</option>
                    <option value="medium">Medium</option>
                    <option value="high">High</option>
                    <option value="urgent">Urgent</option>
                </select>
            </div>
            {allTasks.length > 0 && (
                <div className="mt-2">
                    <label className="text-[11px] text-muted-foreground mb-1 block">Depends on:</label>
                    <div className="flex flex-wrap gap-1">
                        {allTasks.filter((t) => t.status !== "done").map((t) => (
                            <button key={t.taskid}
                                className={cn(
                                    "px-1.5 py-0.5 text-[11px] rounded border cursor-pointer transition-colors",
                                    deps.includes(t.taskid) ? "bg-accent/20 border-accent text-accent" : "border-border/50 text-muted-foreground hover:border-accent/50",
                                )}
                                onClick={() => setDeps((prev) => prev.includes(t.taskid) ? prev.filter((d) => d !== t.taskid) : [...prev, t.taskid])}
                            >
                                {t.title.slice(0, 20)}
                            </button>
                        ))}
                    </div>
                </div>
            )}
            <div className="flex justify-end gap-2 mt-4">
                <button className="px-3 py-1.5 rounded text-sm text-muted-foreground hover:text-primary cursor-pointer" onClick={onCancel}>Cancel</button>
                <button className="px-3 py-1.5 rounded bg-accent/80 text-primary hover:bg-accent text-sm font-medium cursor-pointer disabled:opacity-50" onClick={handleSubmit} disabled={!title.trim() || submitting}>
                    {submitting ? "Creating..." : "Create Task"}
                </button>
            </div>
        </div>
    );
}
