// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { BlockNodeModel } from "@/app/block/blocktypes";
import { atoms, getSettingsKeyAtom, globalStore } from "@/app/store/global";
import type { TabModel } from "@/app/store/tab-model";
import { waveEventSubscribeSingle } from "@/app/store/wps";
import { RpcApi } from "@/app/store/wshclientapi";
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

    statusAtom!: jotai.Atom<TeamStatusData>;

    private eventUnsubTask?: () => void;
    private eventUnsubRuntimeMember?: () => void;
    private eventUnsubMember?: () => void;
    private eventUnsubProject?: () => void;

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
                pausedtasks: 0,
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
        this.eventUnsubTask = waveEventSubscribeSingle({
            eventType: "team:taskupdate",
            handler: () => {
                fireAndForget(async () => this.refreshAllData());
            },
        });
        this.eventUnsubRuntimeMember = waveEventSubscribeSingle({
            eventType: "team:workerupdate",
            handler: () => {
                fireAndForget(async () => this.refreshAllData());
            },
        });
        this.eventUnsubMember = waveEventSubscribeSingle({
            eventType: "team:memberupdate",
            handler: () => {
                fireAndForget(async () => this.refreshAllData());
            },
        });
        this.eventUnsubProject = waveEventSubscribeSingle({
            eventType: "team:projectupdate",
            handler: () => {
                fireAndForget(async () => this.refreshAllData());
            },
        });
    }

    dispose(): void {
        this.stopSupervision();
        this.eventUnsubTask?.();
        this.eventUnsubRuntimeMember?.();
        this.eventUnsubMember?.();
        this.eventUnsubProject?.();
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

    private async runSupervisionCycle(): Promise<void> {
        try {
            globalStore.set(this.isProcessingAtom, true);
            await this.refreshAllData();

            const pendingTasks = globalStore.get(this.pendingTasksAtom) ?? [];
            const workingTasks = globalStore.get(this.workingTasksAtom) ?? [];
            const runtimeMembers = globalStore.get(this.runtimeMembersAtom) ?? [];

            if (
                pendingTasks.length === 0 &&
                workingTasks.length === 0 &&
                runtimeMembers.every((w) => w.status !== "working")
            ) {
                return;
            }

            const memberOutputs = await this.collectMemberOutputs(runtimeMembers);

            const prompt = this.buildAnalysisPrompt(
                pendingTasks,
                workingTasks,
                globalStore.get(this.doneTasksAtom) ?? [],
                globalStore.get(this.failedTasksAtom) ?? [],
                memberOutputs
            );

            const action = await this.callAssistantLLM(prompt);
            globalStore.set(this.lastLLMCallAtom, new Date().toISOString());

            await this.executeAssistantActions(action);
        } catch (e) {
            globalStore.set(this.errorAtom, String(e));
        } finally {
            globalStore.set(this.isProcessingAtom, false);
        }
    }

    async refreshAllData(): Promise<void> {
        try {
            const tasks = await RpcApi.TeamListTasksCommand(TabRpcClient, {});
            const pending: TeamTask[] = [];
            const working: TeamTask[] = [];
            const done: TeamTask[] = [];
            const failed: TeamTask[] = [];
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
                }
            }
            globalStore.set(this.pendingTasksAtom, pending);
            globalStore.set(this.workingTasksAtom, working);
            globalStore.set(this.doneTasksAtom, done);
            globalStore.set(this.failedTasksAtom, failed);
        } catch {}

        try {
            const workers = await RpcApi.TeamListWorkersCommand(TabRpcClient, "");
            globalStore.set(this.runtimeMembersAtom, workers);
        } catch {}

        try {
            const members = await RpcApi.TeamListMembersCommand(TabRpcClient, {});
            globalStore.set(this.membersAtom, members);
        } catch {}

        try {
            const activities = await RpcApi.TeamListActivityCommand(TabRpcClient, { limit: 50 });
            globalStore.set(this.activityLogAtom, activities);
        } catch {}

        try {
            const projects = await RpcApi.TeamListProjectsCommand(TabRpcClient);
            globalStore.set(this.projectsAtom, projects);
        } catch {}

        if (globalStore.get(this.templatesAtom).length === 0) {
            try {
                const templates = await RpcApi.TeamListTemplatesCommand(TabRpcClient);
                globalStore.set(this.templatesAtom, templates ?? []);
            } catch {}
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
        await RpcApi.TeamUpdateWorkerCommand(TabRpcClient, { workerid: workerId, ...config });
        await this.refreshAllData();
    }

    async assignTask(taskId: string, memberId: string): Promise<void> {
        await RpcApi.TeamUpdateTaskCommand(TabRpcClient, {
            taskid: taskId,
            assignedworkerid: memberId,
        });
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

    async retryTask(taskId: string): Promise<void> {
        await RpcApi.TeamRetryTaskCommand(TabRpcClient, taskId);
        await this.refreshAllData();
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

    async assignMemberToProject(memberId: string, projectId: string): Promise<void> {
        await RpcApi.TeamUpdateMemberCommand(TabRpcClient, { memberid: memberId, projectid: projectId } as any);
        await this.refreshAllData();
    }

    async executeTask(taskId: string, command: string): Promise<void> {
        const runtimeMembers = globalStore.get(this.runtimeMembersAtom) ?? [];
        const task = [...(globalStore.get(this.pendingTasksAtom) ?? []), ...(globalStore.get(this.workingTasksAtom) ?? [])].find((t) => t.taskid === taskId);
        const memberId = task?.assignedworkerid ?? runtimeMembers.find((w) => w.status === "idle")?.workerid;
        if (!memberId) return;
        await RpcApi.TeamExecuteTaskCommand(TabRpcClient, { workerid: memberId, taskid: taskId, command });
        await this.refreshAllData();
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
        memberOutputs: Map<string, WorkerOutput>
    ): string {
        const safePending = pending ?? [];
        const safeWorking = working ?? [];
        const safeDone = done ?? [];
        const safeFailed = failed ?? [];

        let prompt = "## Current State\n\n";
        prompt += `Pending Tasks: ${safePending.map((t) => `"${t.title}" (${t.priority})`).join(", ") || "none"}\n`;
        prompt += `Working Tasks: ${safeWorking.map((t) => `"${t.title}" → member ${t.assignedworkerid}`).join(", ") || "none"}\n`;
        prompt += `Done Tasks: ${safeDone.map((t) => `"${t.title}"`).join(", ") || "none"}\n`;
        prompt += `Failed Tasks: ${safeFailed.map((t) => `"${t.title}"`).join(", ") || "none"}\n\n`;

        for (const [memberId, output] of memberOutputs) {
            if (output.error) {
                prompt += `Member ${memberId} error: ${output.error}\n`;
            } else if (output.hashChanged && output.lines) {
                prompt += `Member ${memberId} recent output:\n${output.lines.slice(-20).join("\n")}\n\n`;
            }
        }

        prompt += "\nAnalyze the state above and return JSON actions.";
        return prompt;
    }

    private async callAssistantLLM(prompt: string): Promise<AssistantAction> {
        const aiModeConfigs = globalStore.get(atoms.waveaiModeConfigAtom);
        const currentMode = globalStore.get(getSettingsKeyAtom("waveai:defaultmode")) ?? "waveai@balanced";
        const modeConfig = aiModeConfigs?.[currentMode];

        const model = modeConfig?.["ai:model"] ?? "claude-sonnet-4-20250514";
        const endpoint = modeConfig?.["ai:endpoint"] ?? "";
        const apiToken = modeConfig?.["ai:apitoken"] ?? "";
        const apiType = modeConfig?.["ai:apitype"] ?? "anthropic";

        const requestBody = {
            msg: {
                role: "user",
                parts: [{ type: "text", content: prompt }],
            },
            chatid: crypto.randomUUID(),
            widgetaccess: false,
            aimode: currentMode,
            stream: true,
        };

        const baseUrl = getWebServerEndpoint();
        const url = endpoint || `${baseUrl}/api/post-chat-message`;

        let fullResponse = "";

        try {
            const response = await fetch(url, {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                    ...(apiToken ? { Authorization: `Bearer ${apiToken}` } : {}),
                },
                body: JSON.stringify(requestBody),
            });

            if (!response.ok) {
                const errorText = await response.text();
                throw new Error(`API error: ${response.status} - ${errorText}`);
            }

            const reader = response.body?.getReader();
            if (!reader) {
                throw new Error("No response body");
            }

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
                            if (parsed.text) {
                                fullResponse += parsed.text;
                            }
                            if (parsed.error) {
                                throw new Error(parsed.error);
                            }
                        } catch {
                            fullResponse += data;
                        }
                    }
                }
            }
        } catch (e) {
            globalStore.set(this.errorAtom, String(e));
            return { actions: [{ type: "noop", reason: String(e) }] };
        }

        return this.parseAssistantResponse(fullResponse);
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

    private async executeAssistantActions(action: AssistantAction): Promise<void> {
        for (const act of action.actions) {
            try {
                switch (act.type) {
                    case "assign_task":
                        if (act.task_id && act.worker_id) {
                            await RpcApi.TeamUpdateTaskCommand(TabRpcClient, {
                                taskid: act.task_id,
                                status: "assigned",
                                assignedworkerid: act.worker_id,
                            });
                            await RpcApi.TeamUpdateWorkerCommand(TabRpcClient, {
                                workerid: act.worker_id,
                                status: "working",
                                assignedtaskid: act.task_id,
                            });
                            if (act.instruction) {
                                await this.sendToTerminal(act.worker_id, act.instruction + "\n");
                            }
                            await this.logActivity("task_assign", `Task ${act.task_id} assigned to ${act.worker_id}`);
                        }
                        break;

                    case "wake_worker":
                        if (act.worker_id && act.message) {
                            await this.sendToTerminal(act.worker_id, act.message + "\n");
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

                    case "noop":
                        break;
                }
            } catch (e) {
                await this.logActivity("error", `Action ${act.type} failed: ${e}`);
            }
        }
    }

    async createRuntimeMember(tool: string, config?: { name?: string; maxRetries?: number; capabilities?: string[]; persona?: string; description?: string; skills?: string[]; mcpservers?: TeamMCPConfig[]; customcmd?: string }): Promise<string> {
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

    private async sendToTerminal(blockId: string, text: string): Promise<void> {
        const b64data = stringToBase64(text);
        await RpcApi.ControllerInputCommand(TabRpcClient, {
            blockid: blockId,
            inputdata64: b64data,
        });
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

const ASSISTANT_SYSTEM_PROMPT = `You are a project management assistant embedded in Wave Terminal. You manage a team of AI coding agents that run in terminal sessions.

## Your Role
- Monitor task progress by analyzing terminal output
- Assign pending tasks to available members
- Detect stuck/errored members and send natural language prompts to wake them
- Report completion status

## Communication with Members
You communicate with members by writing text into their terminal sessions. The text you output will be typed into their terminal. Be concise and direct.

## Task Status Machine
- pending → assigned: when you assign to a member
- assigned → working: when member starts showing activity
- working → done: when member output indicates completion
- working → failed: when member shows repeated errors

## Response Format
You MUST respond with valid JSON in this exact format (no markdown, no explanation):
{"actions":[{"type":"assign_task","task_id":"...","worker_id":"...","instruction":"..."}]}

Only take actions when the situation actually warrants it. Do not wake members that are making progress. Do not reassign tasks that are being worked on.`;
