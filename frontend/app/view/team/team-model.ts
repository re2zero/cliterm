// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { BlockNodeModel } from "@/app/block/blocktypes";
import { getTerminalLastLines } from "@/app/block/terminal-snapshot";
import { atoms, getSettingsKeyAtom, globalStore } from "@/app/store/global";
import type { TabModel } from "@/app/store/tab-model";
import { waveEventSubscribeSingle } from "@/app/store/wps";
import { RpcApi } from "@/app/store/wshclientapi";
import { BlockService } from "@/app/store/services";
import { TabRpcClient } from "@/app/store/wshrpcutil";
import { getWebServerEndpoint } from "@/util/endpoints";
import { fireAndForget, stringToBase64 } from "@/util/util";
import * as jotai from "jotai";
import { TeamView } from "./team";

export class TeamViewModel implements ViewModel {
    viewType = "team";
    blockId: string;
    nodeModel: BlockNodeModel;
    tabModel: TabModel;
    viewIcon = jotai.atom("users");
    viewName = jotai.atom("Team");
    noPadding = jotai.atom(false);
    viewComponent = TeamView;

    private static instance: TeamViewModel | null = null;

    pendingTasksAtom = jotai.atom<TeamTask[]>([]);
    workingTasksAtom = jotai.atom<TeamTask[]>([]);
    doneTasksAtom = jotai.atom<TeamTask[]>([]);
    failedTasksAtom = jotai.atom<TeamTask[]>([]);
    pausedTasksAtom = jotai.atom<TeamTask[]>([]);
    membersAtom = jotai.atom<TeamMember[]>([]);
    runtimeMembersAtom = jotai.atom<TeamWorker[]>([]);
    activityLogAtom = jotai.atom<TeamActivity[]>([]);
    projectsAtom = jotai.atom<TeamProject[]>([]);
    templatesAtom = jotai.atom<TeamMember[]>([]);
    memberConfigAtom = jotai.atom({
		runtime: "claude",
		concurrency: 3,
		timeout: 300,
		maxRetries: 3,
	});
    isSupervisingAtom = jotai.atom(false);
    supervisorInterval: number | null = null;
    lastLLMCallAtom = jotai.atom("");
    isProcessingAtom = jotai.atom(false);
    errorAtom = jotai.atom<string>(null as unknown as string);
    supervisionIntervalMs = jotai.atom(10000);
    private wakeAttempts = new Map<string, { count: number; lastAttempt: number }>();
    private outputSnapshots = new Map<string, { content: string; stalledCount: number }>();

    statusAtom!: jotai.Atom<TeamStatusData>;

    private eventUnsubTask?: () => void;
    private eventUnsubRuntimeMember?: () => void;
    private eventUnsubMember?: () => void;
    private eventUnsubProject?: () => void;
    private refreshTimer: number | null = null;

    constructor(initOpts: ViewModelInitType) {
        this.blockId = initOpts.blockId;
        this.nodeModel = initOpts.nodeModel;
        this.tabModel = initOpts.tabModel;

        this.statusAtom = jotai.atom((get) => {
            const pendingTasks = get(this.pendingTasksAtom) ?? [];
            const workingTasks = get(this.workingTasksAtom) ?? [];
            const runtimeMembers = get(this.runtimeMembersAtom) ?? [];
            const members = get(this.membersAtom) ?? [];
            const failedTasks = get(this.failedTasksAtom) ?? [];
            const doneTasks = get(this.doneTasksAtom) ?? [];
            return {
                pendingtasks: pendingTasks.length,
                workingtasks: workingTasks.length,
                donetasks: doneTasks.length,
                failedtasks: failedTasks.length,
                pausedtasks: (get(this.pausedTasksAtom) ?? []).length,
                activeworkers: runtimeMembers.filter((w) => w.status === "working").length,
                idleworkers: runtimeMembers.filter((w) => w.status === "idle").length,
                offlineworkers: runtimeMembers.filter((w) => w.status === "offline").length,
                totalmembers: members.length,
            };
        });
    }

    static getInstance(): TeamViewModel {
        if (!TeamViewModel.instance) {
            TeamViewModel.instance = new TeamViewModel({
                blockId: "",
                nodeModel: null as unknown as BlockNodeModel,
                tabModel: null as unknown as TabModel,
            } as ViewModelInitType);
        }
        return TeamViewModel.instance;
    }

    async init(): Promise<void> {
        await this.refreshAllData();
        const debouncedRefresh = () => {
            if (this.refreshTimer != null) {
                clearTimeout(this.refreshTimer);
            }
            this.refreshTimer = window.setTimeout(() => {
                this.refreshTimer = null;
                fireAndForget(async () => this.refreshAllData());
            }, 200);
        };
        this.eventUnsubTask = waveEventSubscribeSingle({
            eventType: "team:taskupdate",
            handler: debouncedRefresh,
        });
        this.eventUnsubRuntimeMember = waveEventSubscribeSingle({
            eventType: "team:workerupdate",
            handler: debouncedRefresh,
        });
        this.eventUnsubMember = waveEventSubscribeSingle({
            eventType: "team:memberupdate",
            handler: debouncedRefresh,
        });
        this.eventUnsubProject = waveEventSubscribeSingle({
            eventType: "team:projectupdate",
            handler: debouncedRefresh,
        });
        this.startSupervision();
    }

    dispose(): void {
        this.stopSupervision();
        if (this.refreshTimer != null) {
            clearTimeout(this.refreshTimer);
            this.refreshTimer = null;
        }
        this.eventUnsubTask?.();
        this.eventUnsubRuntimeMember?.();
        this.eventUnsubMember?.();
        this.eventUnsubProject?.();
        this.wakeAttempts.clear();
        this.outputSnapshots.clear();
    }

    startSupervision(): void {
        if (globalStore.get(this.isSupervisingAtom)) {
            return;
        }
        globalStore.set(this.isSupervisingAtom, true);
        fireAndForget(async () => this.runSupervisionCycle());
        const interval = globalStore.get(this.supervisionIntervalMs);
        this.supervisorInterval = window.setInterval(() => {
            if (!globalStore.get(this.isProcessingAtom)) {
                fireAndForget(async () => this.runSupervisionCycle());
            }
        }, interval);
    }

    stopSupervision(): void {
        globalStore.set(this.isSupervisingAtom, false);
        if (this.supervisorInterval != null) {
            clearInterval(this.supervisorInterval);
            this.supervisorInterval = null;
        }
    }

    private static readonly STALL_THRESHOLD_MS = 5 * 60 * 1000;
    private static readonly MAX_WAKE_ATTEMPTS = 2;
    private static readonly WAKE_COOLDOWN_MS = 60 * 1000;
    private static readonly STALL_SNAPSHOT_LINES = 3;
    private static readonly ANALYSIS_SNAPSHOT_LINES = 30;

    private async runSupervisionCycle(): Promise<void> {
        try {
            globalStore.set(this.isProcessingAtom, true);
            await this.refreshAllData();

            const runtimeMembers = globalStore.get(this.runtimeMembersAtom) ?? [];
            if (runtimeMembers.every((w) => w.status !== "working")) {
                const pendingTasks = globalStore.get(this.pendingTasksAtom) ?? [];
                const workingTasks = globalStore.get(this.workingTasksAtom) ?? [];
                if (pendingTasks.length === 0 && workingTasks.length === 0) {
                    return;
                }
            }

            await this.checkWorkerLiveness(runtimeMembers);
            await this.refreshAllData();

            const pendingTasks = globalStore.get(this.pendingTasksAtom) ?? [];
            const workingTasks = globalStore.get(this.workingTasksAtom) ?? [];
            const runtimeMembers2 = globalStore.get(this.runtimeMembersAtom) ?? [];

            const memberOutputs = await this.collectMemberOutputs(runtimeMembers2);

            const prompt = this.buildAnalysisPrompt(
                pendingTasks,
                workingTasks,
                globalStore.get(this.doneTasksAtom) ?? [],
                globalStore.get(this.failedTasksAtom) ?? [],
                globalStore.get(this.pausedTasksAtom) ?? [],
                memberOutputs
            );

            const action = await this.callAssistantLLM(prompt);
            globalStore.set(this.lastLLMCallAtom, new Date().toISOString());

            await this.executeAssistantActions(action, runtimeMembers2);

            globalStore.set(this.errorAtom, null);
        } catch (e) {
            const msg = String(e);
            if (msg.includes("messageid") || msg.includes("API error 500")) {
                console.warn("[team] supervision cycle error (will retry):", e);
            } else {
                globalStore.set(this.errorAtom, msg);
            }
        } finally {
            globalStore.set(this.isProcessingAtom, false);
        }
    }

    async refreshAllData(): Promise<void> {
        try {
            const tasks = await RpcApi.TeamListTasksCommand(TabRpcClient, {}) ?? [];
            const pending: TeamTask[] = [];
            const working: TeamTask[] = [];
            const done: TeamTask[] = [];
            const failed: TeamTask[] = [];
            const paused: TeamTask[] = [];
            for (const t of tasks) {
                switch (t.status) {
                    case "pending":
                    case "assigned":
                        pending.push(t);
                        break;
                    case "working":
                        working.push(t);
                        break;
                    case "done":
                        done.push(t);
                        break;
                    case "failed":
                        failed.push(t);
                        break;
                    case "paused":
                        paused.push(t);
                        break;
                }
            }
            globalStore.set(this.pendingTasksAtom, pending);
            globalStore.set(this.workingTasksAtom, working);
            globalStore.set(this.doneTasksAtom, done);
            globalStore.set(this.failedTasksAtom, failed);
            globalStore.set(this.pausedTasksAtom, paused);
        } catch (e) {
            console.warn("[team] refreshAllData tasks failed:", e);
        }

        try {
            const workers = await RpcApi.TeamListWorkersCommand(TabRpcClient, "");
            globalStore.set(this.runtimeMembersAtom, workers);
        } catch (e) {
            console.warn("[team] refreshAllData workers failed:", e);
        }

        try {
            const members = await RpcApi.TeamListMembersCommand(TabRpcClient, {});
            globalStore.set(this.membersAtom, members);
        } catch (e) {
            console.warn("[team] refreshAllData members failed:", e);
        }

        try {
            const activities = await RpcApi.TeamListActivityCommand(TabRpcClient, { limit: 50 });
            globalStore.set(this.activityLogAtom, activities);
        } catch (e) {
            console.warn("[team] refreshAllData activities failed:", e);
        }

        try {
            const projects = await RpcApi.TeamListProjectsCommand(TabRpcClient);
            globalStore.set(this.projectsAtom, projects);
        } catch (e) {
            console.warn("[team] refreshAllData projects failed:", e);
        }

        if (globalStore.get(this.templatesAtom).length === 0) {
            try {
                const templates = await RpcApi.TeamListTemplatesCommand(TabRpcClient);
                globalStore.set(this.templatesAtom, templates ?? []);
            } catch (e) {
                console.warn("[team] refreshAllData templates failed:", e);
            }
        }
    }

    async createTask(title: string, description: string, priority: string, dependsOn?: string[]): Promise<void> {
        await RpcApi.TeamCreateTaskCommand(TabRpcClient, { title, description, priority, dependson: dependsOn });
        await this.refreshAllData();
    }

    async deleteTask(taskId: string): Promise<void> {
        await RpcApi.TeamDeleteTaskCommand(TabRpcClient, taskId);
        await this.refreshAllData();
    }

    async deleteRuntimeMember(workerId: string): Promise<void> {
        await RpcApi.TeamDeleteWorkerCommand(TabRpcClient, workerId);
        await this.refreshAllData();
    }

    async createMember(data: {
        name: string;
        tool: string;
        customcmd?: string;
        description?: string;
        persona?: string;
        skills?: string[];
        mcpservers?: TeamMCPConfig[];
        capabilities?: string[];
        maxretries?: number;
        maxconcurrency?: number;
        model?: string;
        memory?: string;
        color?: string;
    }): Promise<string> {
        const result = await RpcApi.TeamCreateMemberCommand(TabRpcClient, data);
        await this.refreshAllData();
        return result?.memberid ?? "";
    }

    async updateMember(memberId: string, updates: Record<string, any>): Promise<void> {
        await RpcApi.TeamUpdateMemberCommand(TabRpcClient, { memberid: memberId, ...updates });
        await this.refreshAllData();
    }

    async deleteMember(memberId: string): Promise<void> {
        await RpcApi.TeamDeleteMemberCommand(TabRpcClient, memberId);
        await this.refreshAllData();
    }

    async getMember(memberId: string): Promise<TeamMember | null> {
        const members = globalStore.get(this.membersAtom) ?? [];
        return members.find((m) => m.memberid === memberId) ?? null;
    }

    async updateRuntimeMember(workerId: string, config: Record<string, any>): Promise<void> {
        await RpcApi.TeamUpdateWorkerCommand(TabRpcClient, { workerid: workerId, projectid: "", ...config });
        await this.refreshAllData();
    }

    async pauseTask(taskId: string): Promise<void> {
        await RpcApi.TeamPauseTaskCommand(TabRpcClient, taskId);
        await this.refreshAllData();
    }

    async resumeTask(taskId: string): Promise<void> {
        await RpcApi.TeamResumeTaskCommand(TabRpcClient, taskId);
        await this.refreshAllData();
    }

    async assignTask(taskId: string, memberId: string): Promise<void> {
        try {
            await RpcApi.TeamAssignTaskCommand(TabRpcClient, { taskid: taskId, memberid: memberId });
            await this.refreshAllData();
        } catch (e) {
            console.error("[team] assign failed:", e);
            globalStore.set(this.errorAtom, `Assign failed: ${e instanceof Error ? e.message : String(e)}`);
        }
    }

    async updateTask(taskId: string, updates: Record<string, any>): Promise<void> {
        await RpcApi.TeamUpdateTaskCommand(TabRpcClient, { taskid: taskId, ...updates });
        await this.refreshAllData();
    }

    async createProject(data: { name: string; path: string; spec?: string }): Promise<string> {
        const result = await RpcApi.TeamCreateProjectCommand(TabRpcClient, {
            name: data.name,
            path: data.path,
            spec: data.spec ?? "",
        });
        await this.refreshAllData();
        return result?.projectid ?? "";
    }

    async updateProject(projectId: string, updates: Record<string, any>): Promise<void> {
        await RpcApi.TeamUpdateProjectCommand(TabRpcClient, { projectid: projectId, ...updates });
        await this.refreshAllData();
    }

    async deleteProject(projectId: string): Promise<void> {
        await RpcApi.TeamDeleteProjectCommand(TabRpcClient, projectId);
        await this.refreshAllData();
    }

    async saveTemplate(data: { name: string; tool?: string; description?: string; persona?: string; skills?: string[]; capabilities?: string[]; customcmd?: string; maxretries?: number; mcpservers?: TeamMCPConfig[] }): Promise<void> {
        await RpcApi.TeamSaveTemplateCommand(TabRpcClient, {
            ...data,
            maxconcurrency: 3,
        } as TeamMember);
        globalStore.set(this.templatesAtom, []);
    }

    async deleteTemplate(templateName: string): Promise<void> {
        await RpcApi.TeamDeleteTemplateCommand(TabRpcClient, templateName);
        globalStore.set(this.templatesAtom, []);
    }

    async assignMemberToProject(workerId: string, projectId: string): Promise<void> {
        await RpcApi.TeamUpdateWorkerCommand(TabRpcClient, { workerid: workerId, projectid: projectId });
        await this.refreshAllData();
    }

    private async checkWorkerLiveness(runtimeMembers: TeamWorker[]): Promise<void> {
        const now = Date.now();
        const allTasks = [
            ...(globalStore.get(this.pendingTasksAtom) ?? []),
            ...(globalStore.get(this.workingTasksAtom) ?? []),
            ...(globalStore.get(this.doneTasksAtom) ?? []),
            ...(globalStore.get(this.failedTasksAtom) ?? []),
            ...(globalStore.get(this.pausedTasksAtom) ?? []),
        ];
        const taskMap = new Map(allTasks.map((t) => [t.taskid, t]));
        const workingMembers = runtimeMembers.filter((w) => w.status === "working");

        for (const worker of workingMembers) {
            const lastActive = worker.lastheartbeat * 1000;
            const stalledDuration = now - lastActive;

            if (stalledDuration < TeamViewModel.STALL_THRESHOLD_MS) {
                this.wakeAttempts.delete(worker.workerid);
                continue;
            }

            const taskId = worker.assignedtaskid;
            if (!taskId || !worker.blockid) continue;

            const task = taskMap.get(taskId);
            if (!task || task.status === "done" || task.status === "failed" || task.status === "cancelled") {
                this.wakeAttempts.delete(worker.workerid);
                continue;
            }

            const attempt = this.wakeAttempts.get(worker.workerid) ?? { count: 0, lastAttempt: 0 };
            if (attempt.count >= TeamViewModel.MAX_WAKE_ATTEMPTS) {
                await RpcApi.TeamUpdateTaskCommand(TabRpcClient, {
                    taskid: taskId,
                    status: "paused",
                    error: `worker unresponsive after ${attempt.count} wake attempts`,
                });
                await RpcApi.TeamUpdateWorkerCommand(TabRpcClient, {
                    workerid: worker.workerid,
                    status: "idle",
                    assignedtaskid: "",
                    projectid: "",
                });
                await this.logActivity("task_paused", `Task ${taskId} paused — worker ${worker.name} released`);
                this.wakeAttempts.delete(worker.workerid);
                continue;
            }

            if (now - attempt.lastAttempt < TeamViewModel.WAKE_COOLDOWN_MS) {
                continue;
            }

            const controllerStatus = await BlockService.GetControllerStatus(worker.blockid).catch(() => null);

            if (controllerStatus?.shellprocstatus === "running") {
                const currentSnapshot = getTerminalLastLines(worker.blockid, TeamViewModel.STALL_SNAPSHOT_LINES);
                const prev = this.outputSnapshots.get(worker.workerid);

                if (currentSnapshot == null) {
                    this.wakeAttempts.delete(worker.workerid);
                    this.outputSnapshots.delete(worker.workerid);
                    continue;
                }

                if (prev && prev.content === currentSnapshot) {
                    prev.stalledCount += 1;
                    if (prev.stalledCount < 2) {
                        continue;
                    }
                } else {
                    this.outputSnapshots.set(worker.workerid, { content: currentSnapshot, stalledCount: 0 });
                    this.wakeAttempts.delete(worker.workerid);
                    continue;
                }

                attempt.count += 1;
                attempt.lastAttempt = now;
                this.wakeAttempts.set(worker.workerid, attempt);
                this.outputSnapshots.delete(worker.workerid);

                const analysisSnapshot = getTerminalLastLines(worker.blockid, TeamViewModel.ANALYSIS_SNAPSHOT_LINES);
                const analysisResult = await this.analyzeWorkerOutput(worker.name, analysisSnapshot ?? currentSnapshot);
                if (analysisResult.action === "wait") {
                    this.wakeAttempts.delete(worker.workerid);
                    await this.logActivity("supervision_analysis", `Worker ${worker.name}: LLM says still working — resetting stall counter`);
                    continue;
                }
                if (analysisResult.action === "fail") {
                    await RpcApi.TeamUpdateTaskCommand(TabRpcClient, {
                        taskid: taskId,
                        status: "failed",
                        error: analysisResult.reason || "supervision detected failure",
                    });
                    await RpcApi.TeamUpdateWorkerCommand(TabRpcClient, {
                        workerid: worker.workerid,
                        status: "idle",
                        assignedtaskid: "",
                        projectid: "",
                    });
                    await this.logActivity("task_failed", `Task ${taskId} failed — supervision detected: ${analysisResult.reason}`);
                    this.wakeAttempts.delete(worker.workerid);
                    continue;
                }

                const sent = await this.sendToTerminal(worker.blockid, analysisResult.prompt + "\r");
                if (!sent) {
                    await RpcApi.TeamUpdateWorkerCommand(TabRpcClient, {
                        workerid: worker.workerid,
                        status: "offline",
                        assignedtaskid: "",
                        projectid: "",
                    });
                    await this.logActivity("worker_offline", `Worker ${worker.name} terminal gone, marked offline`);
                }
                await this.logActivity("supervision_analysis", `Worker ${worker.name}: ${analysisResult.action} — ${analysisResult.reason}`);
                continue;
            }

            attempt.count += 1;
            attempt.lastAttempt = now;
            this.wakeAttempts.set(worker.workerid, attempt);

            const sent = await this.sendToTerminal(worker.blockid, "If your task is done, call team_update_task MCP tool with status=\"done\" and result=\"summary\". If failed, call with status=\"failed\" and error=\"description\". What is your current status?\r");
            if (!sent) {
                await RpcApi.TeamUpdateWorkerCommand(TabRpcClient, {
                    workerid: worker.workerid,
                    status: "offline",
                    assignedtaskid: "",
                    projectid: "",
                });
                await this.logActivity("worker_offline", `Worker ${worker.name} block gone, marked offline`);
                this.wakeAttempts.delete(worker.workerid);
                continue;
            }
            await this.logActivity("worker_wake", `Wake attempt ${attempt.count}/${TeamViewModel.MAX_WAKE_ATTEMPTS} for ${worker.name} (stalled ${Math.round(stalledDuration / 60000)}min)`);
        }

        const workingTasksSet = new Set((globalStore.get(this.workingTasksAtom) ?? []).map((t) => t.taskid));
        const activeWorkerIds = new Set(
            workingMembers
                .filter((w) => w.assignedtaskid && workingTasksSet.has(w.assignedtaskid))
                .map((w) => w.workerid)
        );
        for (const workerId of this.wakeAttempts.keys()) {
            if (!activeWorkerIds.has(workerId)) {
                this.wakeAttempts.delete(workerId);
            }
        }
    }

    private async analyzeWorkerOutput(
        workerName: string,
        terminalOutput: string
    ): Promise<{ action: "wake" | "fail" | "wait"; prompt: string; reason: string }> {
        try {
            const response = await this.callLLMRaw(
                `You are a team supervisor. Analyze this worker's terminal output to determine its state.

Worker "${workerName}" terminal output (last ${TeamViewModel.ANALYSIS_SNAPSHOT_LINES} lines):
\`\`\`
${terminalOutput}
\`\`\`

Determine the worker's state and respond with EXACTLY one of:
1. WAIT — The worker is still actively working (e.g., compiling, downloading, processing). Just needs more time.
2. WAKE — The worker is stuck or waiting for input. It needs a nudge.
   For WAKE, provide a specific prompt to send to the worker's terminal.
3. FAIL — The worker has clearly failed (error message, crash, or unrecoverable state).

Respond in this exact format:
ACTION: WAIT|WAKE|FAIL
REASON: <brief explanation>
PROMPT: <only for WAKE — the exact text to send to the terminal, including any MCP tool calls>`
            );

            const actionMatch = response.match(/ACTION:\s*(WAIT|WAKE|FAIL)/i);
            const reasonMatch = response.match(/REASON:\s*(.+)/i);
            const promptMatch = response.match(/PROMPT:\s*(.+)/i);

            const action = actionMatch?.[1]?.toUpperCase();
            const reason = reasonMatch?.[1]?.trim() ?? "no reason provided";

            if (action === "WAIT") {
                return { action: "wait", prompt: "", reason };
            }
            if (action === "FAIL") {
                return { action: "fail", prompt: "", reason };
            }
            return {
                action: "wake",
                prompt: promptMatch?.[1]?.trim() ?? "What is your current status? Please call team_update_task MCP tool.",
                reason,
            };
        } catch (err) {
            return {
                action: "wake",
                prompt: "What is your current status? Please call team_update_task MCP tool.",
                reason: `LLM analysis failed: ${err}`,
            };
        }
    }

    private async collectMemberOutputs(runtimeMembers: TeamWorker[]): Promise<Map<string, WorkerOutput>> {
        const outputs = new Map<string, WorkerOutput>();
        for (const member of runtimeMembers) {
            if (member.status !== "working") {
                continue;
            }
            outputs.set(member.workerid, {
                lines: [],
                totalLines: 0,
                lastUpdated: member.lastheartbeat,
                hashChanged: false,
            });
        }
        return outputs;
    }

    private buildAnalysisPrompt(
        pending: TeamTask[],
        working: TeamTask[],
        done: TeamTask[],
        failed: TeamTask[],
        paused: TeamTask[],
        memberOutputs: Map<string, WorkerOutput>
    ): string {
        let prompt = "## Current State\n\n";
        prompt += `Pending Tasks: ${pending.map((t) => `"${t.title}" [${t.taskid}]`).join(", ") || "none"}\n`;
        prompt += `Working Tasks: ${working.map((t) => `"${t.title}" → worker ${t.assignedworkerid} [${t.taskid}]`).join(", ") || "none"}\n`;
        prompt += `Done Tasks: ${done.length}\n`;
        prompt += `Failed Tasks: ${failed.map((t) => `"${t.title}" [${t.taskid}]`).join(", ") || "none"}\n`;
        prompt += `Paused Tasks: ${paused.map((t) => `"${t.title}" [${t.taskid}]`).join(", ") || "none"}\n\n`;

        for (const [memberId, output] of memberOutputs) {
            if (output.error) {
                prompt += `Worker ${memberId} error: ${output.error}\n`;
            }
        }

        prompt += "## Available Actions\n";
        prompt += "Return a JSON object with an `actions` array. Each action:\n";
        prompt += "- { type: \"assign_task\", task_id, worker_id } — assign a pending task to an idle worker\n";
        prompt += "- { type: \"execute_task\", task_id, worker_id, command? } — send task command to worker terminal\n";
        prompt += "- { type: \"create_worker\", tool: \"claude\"|\"opencode\"|\"custom\", task_id } — create a new worker block\n";
        prompt += "- { type: \"wake_worker\", worker_id, message } — send text to a worker's terminal\n";
        prompt += "- { type: \"update_task\", task_id, status, result?, progress? } — update task status\n";
        prompt += "- { type: \"fail_task\", task_id, reason } — mark task failed\n";
        prompt += "- { type: \"pause_task\", task_id, reason } — pause a stalled task (releases worker)\n";
        prompt += "- { type: \"resume_task\", task_id } — resume a paused task\n";
        prompt += "- { type: \"noop\" } — do nothing\n\n";
        prompt += "## Rules\n";
        prompt += "- If idle workers exist and pending tasks exist, assign + execute.\n";
        prompt += "- If all tasks are done/failed/paused with no pending work, return noop.\n";
        prompt += "- Do NOT poll status — you will be called again automatically.\n";
        prompt += "- Use exact task_id and worker_id values from above.\n\n";
        prompt += "Respond with JSON only.";
        return prompt;
    }

    private async callLLMRaw(prompt: string): Promise<string> {
        const baseUrl = getWebServerEndpoint();
        if (!baseUrl) throw new Error("WaveAI endpoint not configured");

        const currentMode = globalStore.get(getSettingsKeyAtom("waveai:defaultmode")) ?? "waveai@balanced";
        const url = `${baseUrl}/api/post-chat-message`;
        const requestBody = {
            msg: {
                messageid: crypto.randomUUID(),
                role: "user",
                parts: [{ type: "text", content: prompt }],
            },
            chatid: crypto.randomUUID(),
            widgetaccess: false,
            aimode: currentMode,
            stream: true,
        };

        let fullResponse = "";
        const response = await fetch(url, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(requestBody),
        });

        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(`API error ${response.status}: ${errorText.slice(0, 200)}`);
        }

        const reader = response.body?.getReader();
        if (!reader) throw new Error("No response body");

        const decoder = new TextDecoder();
        while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            const chunk = decoder.decode(value);
            const lines = chunk.split("\n");
            for (const line of lines) {
                if (line.startsWith("data: ")) {
                    const data = line.slice(6);
                    if (data === "[DONE]") continue;
                    try {
                        const parsed = JSON.parse(data);
                        if (parsed.text) fullResponse += parsed.text;
                        if (parsed.error) throw new Error(parsed.error);
                    } catch (parseErr) {
                        if (String(parseErr).includes("error")) throw parseErr;
                    }
                }
            }
        }

        return fullResponse;
    }

    private async callAssistantLLM(prompt: string): Promise<AssistantAction> {
        const aiModeConfigs = globalStore.get(atoms.waveaiModeConfigAtom);
        const currentMode = globalStore.get(getSettingsKeyAtom("waveai:defaultmode")) ?? "waveai@balanced";

        try {
            const fullResponse = await this.callLLMRaw(prompt);
            return this.parseAssistantResponse(fullResponse);
        } catch (e) {
            globalStore.set(this.errorAtom, `Supervision LLM call failed: ${String(e)}`);
            return { actions: [{ type: "noop", reason: String(e) }] };
        }
    }

    private parseAssistantResponse(text: string): AssistantAction {
        const jsonMatch = text.match(/\{[\s\S]*\}/);
        if (!jsonMatch) {
            return { actions: [{ type: "noop", reason: "Failed to parse LLM response" }] };
        }
        try {
            return JSON.parse(jsonMatch[0]) as AssistantAction;
        } catch {
            return { actions: [{ type: "noop", reason: "Invalid JSON from LLM" }] };
        }
    }

    private async executeAssistantActions(action: AssistantAction, runtimeMembers: TeamWorker[]): Promise<void> {
        for (const act of action.actions) {
            try {
                switch (act.type) {
                    case "assign_task":
                        if (act.task_id && act.worker_id) {
                            const assignWorker = runtimeMembers.find((w) => w.workerid === act.worker_id);
                            if (assignWorker?.status === "offline") {
                                await RpcApi.TeamUpdateWorkerCommand(TabRpcClient, {
                                    workerid: act.worker_id,
                                    status: "idle",
                                    projectid: assignWorker.projectid ?? "",
                                });
                            }
                            await RpcApi.TeamUpdateTaskCommand(TabRpcClient, {
                                taskid: act.task_id,
                                status: "assigned",
                                assignedworkerid: act.worker_id,
                            });
                            await RpcApi.TeamUpdateWorkerCommand(TabRpcClient, {
                                workerid: act.worker_id,
                                status: "working",
                                assignedtaskid: act.task_id,
                                projectid: "",
                            });
                            await this.logActivity("task_assign", `Task ${act.task_id} assigned to ${act.worker_id}`);
                        }
                        break;

                    case "wake_worker":
                        if (act.worker_id && act.message) {
                            const wakeWorker = runtimeMembers.find((w) => w.workerid === act.worker_id);
                            if (wakeWorker?.blockid) {
                                const sent = await this.sendToTerminal(wakeWorker.blockid, act.message + "\r");
                                if (!sent) {
                                    await RpcApi.TeamUpdateWorkerCommand(TabRpcClient, {
                                        workerid: act.worker_id,
                                        status: "offline",
                                        assignedtaskid: "",
                                        projectid: "",
                                    });
                                    await this.logActivity("worker_offline", `Worker ${wakeWorker.name} block gone during wake, marked offline`);
                                    break;
                                }
                            }
                            await this.logActivity("worker_wake", `Woke member ${act.worker_id}`);
                        }
                        break;

                    case "update_task":
                        if (act.task_id && act.status) {
                            await RpcApi.TeamUpdateTaskCommand(TabRpcClient, {
                                taskid: act.task_id,
                                status: act.status,
                                result: act.result,
                                progress: act.progress != null ? Number(act.progress) : undefined,
                            });
                            await this.logActivity("task_update", `Task ${act.task_id} → ${act.status}`);
                        }
                        break;

                    case "create_worker":
                        if (act.tool && act.task_id) {
                            const workerId = await this.createRuntimeMemberBlock(act.tool, act.task_id);
                            await RpcApi.TeamUpdateTaskCommand(TabRpcClient, {
                                taskid: act.task_id,
                                status: "assigned",
                                assignedworkerid: workerId,
                            });
                            await this.logActivity("worker_create", `Created member ${workerId}`);
                        }
                        break;

                    case "execute_task":
                        if (act.worker_id && act.task_id) {
                            const command = act.command || act.instruction || "";
                            await RpcApi.TeamExecuteTaskCommand(TabRpcClient, {
                                workerid: act.worker_id,
                                taskid: act.task_id,
                                command,
                            });
                            await this.logActivity("task_execute", `Task ${act.task_id} executing on ${act.worker_id}`);
                        }
                        break;

                    case "fail_task":
                        if (act.task_id) {
                            await RpcApi.TeamUpdateTaskCommand(TabRpcClient, {
                                taskid: act.task_id,
                                status: "failed",
                                error: act.reason || "failed by supervision",
                            });
                            await this.logActivity("task_fail", `Task ${act.task_id} failed: ${act.reason || ""}`);
                        }
                        break;

                    case "pause_task":
                        if (act.task_id) {
                            await RpcApi.TeamUpdateTaskCommand(TabRpcClient, {
                                taskid: act.task_id,
                                status: "paused",
                                error: act.reason || "paused by supervision",
                            });
                            await this.logActivity("task_pause", `Task ${act.task_id} paused: ${act.reason || ""}`);
                        }
                        break;

                    case "resume_task":
                        if (act.task_id) {
                            await RpcApi.TeamUpdateTaskCommand(TabRpcClient, {
                                taskid: act.task_id,
                                status: "working",
                            });
                            const resumeWorker = runtimeMembers.find((w) => w.assignedtaskid === act.task_id);
                            if (resumeWorker?.blockid) {
                                await this.sendToTerminal(resumeWorker.blockid, "Resume your task. When done, call team_update_task MCP tool with status=\"done\" and result=\"summary\".\r");
                            }
                            await this.logActivity("task_resume", `Task ${act.task_id} resumed`);
                        }
                        break;

                    case "noop":
                        break;
                }
            } catch (e) {
                await this.logActivity("error", `Action ${act.type} failed: ${e}`);
            }
        }
    }

    async createRuntimeMember(tool: string, config?: { name?: string; maxRetries?: number; capabilities?: string[]; persona?: string; description?: string; skills?: string[]; mcpservers?: TeamMCPConfig[]; customcmd?: string; projectid?: string }): Promise<string> {
        const memberResult = await RpcApi.TeamCreateMemberCommand(TabRpcClient, {
            name: config?.name ?? `${tool} member`,
            tool,
            customcmd: config?.customcmd,
            description: config?.description,
            persona: config?.persona,
            skills: config?.skills,
            mcpservers: config?.mcpservers,
            capabilities: config?.capabilities,
            maxretries: config?.maxRetries,
            projectid: config?.projectid,
        });
        const memberId = memberResult?.memberid;
        if (!memberId) {
            throw new Error("Failed to create member: no memberId returned");
        }

        const workerResult = await RpcApi.TeamForkWorkerCommand(TabRpcClient, memberId);
        const workerId = workerResult?.workerid;
        if (!workerId) {
            throw new Error("Failed to fork worker: no workerId returned");
        }

        await this.refreshAllData();
        return workerId;
    }

    private async createRuntimeMemberBlock(tool: string, taskTitle: string): Promise<string> {
        return this.createRuntimeMember(tool);
    }

    private async sendToTerminal(blockId: string, text: string): Promise<boolean> {
        try {
            const b64data = stringToBase64(text);
            await RpcApi.ControllerInputCommand(TabRpcClient, {
                blockid: blockId,
                inputdata64: b64data,
            });
            return true;
        } catch {
            return false;
        }
    }

    private async logActivity(type: string, description: string): Promise<void> {
        await RpcApi.TeamAddActivityCommand(TabRpcClient, {
            type,
            description,
        });
    }

    private simpleHash(str: string): string {
        let hash = 0;
        for (let i = 0; i < str.length; i++) {
            const char = str.charCodeAt(i);
            hash = ((hash << 5) - hash + char) | 0;
        }
        return hash.toString(36);
    }
}
