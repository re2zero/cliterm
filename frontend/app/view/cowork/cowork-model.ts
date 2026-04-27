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
import { CoworkView } from "./cowork";

export class CoworkViewModel implements ViewModel {
    viewType = "cowork";
    blockId: string;
    nodeModel: BlockNodeModel;
    tabModel: TabModel;
    viewIcon = jotai.atom("users");
    viewName = jotai.atom("Cowork");
    noPadding = jotai.atom(false);
    viewComponent = CoworkView;

    private static instance: CoworkViewModel | null = null;

    pendingTasksAtom = jotai.atom<CoworkTask[]>([]);
    workingTasksAtom = jotai.atom<CoworkTask[]>([]);
    doneTasksAtom = jotai.atom<CoworkTask[]>([]);
    failedTasksAtom = jotai.atom<CoworkTask[]>([]);
    workersAtom = jotai.atom<CoworkWorker[]>([]);
    activityLogAtom = jotai.atom<CoworkActivity[]>([]);
    workerConfigAtom = jotai.atom({
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

    statusAtom!: jotai.Atom<CoworkStatusData>;

    private eventUnsubTask?: () => void;
    private eventUnsubWorker?: () => void;

    constructor(initOpts: ViewModelInitType) {
        this.blockId = initOpts.blockId;
        this.nodeModel = initOpts.nodeModel;
        this.tabModel = initOpts.tabModel;

        this.statusAtom = jotai.atom((get) => {
            const pendingTasks = get(this.pendingTasksAtom) ?? [];
            const workingTasks = get(this.workingTasksAtom) ?? [];
            const workers = get(this.workersAtom) ?? [];
            const failedTasks = get(this.failedTasksAtom) ?? [];
            const doneTasks = get(this.doneTasksAtom) ?? [];
            return {
                pendingtasks: pendingTasks.length,
                workingtasks: workingTasks.length,
                donetasks: doneTasks.length,
                failedtasks: failedTasks.length,
                activeworkers: workers.filter((w) => w.status === "working").length,
                idleworkers: workers.filter((w) => w.status === "idle").length,
                totalworkers: workers.length,
            };
        });
    }

    static getInstance(): CoworkViewModel {
        if (!CoworkViewModel.instance) {
            CoworkViewModel.instance = new CoworkViewModel({
                blockId: "",
                nodeModel: null as unknown as BlockNodeModel,
                tabModel: null as unknown as TabModel,
            } as ViewModelInitType);
        }
        return CoworkViewModel.instance;
    }

    async init(): Promise<void> {
        await this.refreshAllData();
        this.eventUnsubTask = waveEventSubscribeSingle({
            eventType: "cowork:taskupdate",
            handler: () => {
                fireAndForget(async () => this.refreshAllData());
            },
        });
        this.eventUnsubWorker = waveEventSubscribeSingle({
            eventType: "cowork:workerupdate",
            handler: () => {
                fireAndForget(async () => this.refreshAllData());
            },
        });
    }

    dispose(): void {
        this.stopSupervision();
        this.eventUnsubTask?.();
        this.eventUnsubWorker?.();
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
            const workers = globalStore.get(this.workersAtom) ?? [];

            if (
                pendingTasks.length === 0 &&
                workingTasks.length === 0 &&
                workers.every((w) => w.status !== "working")
            ) {
                return;
            }

            const workerOutputs = await this.collectWorkerOutputs(workers);

            const prompt = this.buildAnalysisPrompt(
                pendingTasks,
                workingTasks,
                globalStore.get(this.doneTasksAtom) ?? [],
                globalStore.get(this.failedTasksAtom) ?? [],
                workerOutputs
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
            const tasks = await RpcApi.CoworkListTasksCommand(TabRpcClient, {});
            const pending: CoworkTask[] = [];
            const working: CoworkTask[] = [];
            const done: CoworkTask[] = [];
            const failed: CoworkTask[] = [];
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
            const workers = await RpcApi.CoworkListWorkersCommand(TabRpcClient);
            globalStore.set(this.workersAtom, workers);
        } catch {}

        try {
            const activities = await RpcApi.CoworkListActivityCommand(TabRpcClient, { limit: 50 });
            globalStore.set(this.activityLogAtom, activities);
        } catch {}
    }

    async createTask(title: string, description: string, priority: string): Promise<void> {
        await RpcApi.CoworkCreateTaskCommand(TabRpcClient, { title, description, priority });
        await this.refreshAllData();
    }

    async deleteTask(taskId: string): Promise<void> {
        await RpcApi.CoworkDeleteTaskCommand(TabRpcClient, taskId);
        await this.refreshAllData();
    }

    async deleteWorker(workerId: string): Promise<void> {
        await RpcApi.CoworkDeleteWorkerCommand(TabRpcClient, workerId);
        await this.refreshAllData();
    }

    async assignTask(taskId: string, workerId: string): Promise<void> {
        await RpcApi.CoworkUpdateTaskCommand(TabRpcClient, {
            taskid: taskId,
            assignedworker: workerId,
        });
        await this.refreshAllData();
    }

    async pauseTask(taskId: string): Promise<void> {
        await RpcApi.CoworkPauseTaskCommand(TabRpcClient, taskId);
        await this.refreshAllData();
    }

    async resumeTask(taskId: string): Promise<void> {
        await RpcApi.CoworkResumeTaskCommand(TabRpcClient, taskId);
        await this.refreshAllData();
    }

    private async collectWorkerOutputs(workers: CoworkWorker[]): Promise<Map<string, WorkerOutput>> {
        const outputs = new Map<string, WorkerOutput>();
        for (const worker of workers) {
            if (worker.status !== "working") {
                continue;
            }
            outputs.set(worker.workerid, {
                lines: [],
                totalLines: 0,
                lastUpdated: worker.lastactiveat,
                hashChanged: false,
            });
        }
        return outputs;
    }

    private buildAnalysisPrompt(
        pending: CoworkTask[],
        working: CoworkTask[],
        done: CoworkTask[],
        failed: CoworkTask[],
        workerOutputs: Map<string, WorkerOutput>
    ): string {
        const safePending = pending ?? [];
        const safeWorking = working ?? [];
        const safeDone = done ?? [];
        const safeFailed = failed ?? [];

        let prompt = "## Current State\n\n";
        prompt += `Pending Tasks: ${safePending.map((t) => `"${t.title}" (${t.priority})`).join(", ") || "none"}\n`;
        prompt += `Working Tasks: ${safeWorking.map((t) => `"${t.title}" → worker ${t.assignedworker}`).join(", ") || "none"}\n`;
        prompt += `Done Tasks: ${safeDone.map((t) => `"${t.title}"`).join(", ") || "none"}\n`;
        prompt += `Failed Tasks: ${safeFailed.map((t) => `"${t.title}"`).join(", ") || "none"}\n\n`;

        for (const [workerId, output] of workerOutputs) {
            if (output.error) {
                prompt += `Worker ${workerId} error: ${output.error}\n`;
            } else if (output.hashChanged && output.lines) {
                prompt += `Worker ${workerId} recent output:\n${output.lines.slice(-20).join("\n")}\n\n`;
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
                            await RpcApi.CoworkUpdateTaskCommand(TabRpcClient, {
                                taskid: act.task_id,
                                status: "assigned",
                                assignedworker: act.worker_id,
                            });
                            await RpcApi.CoworkUpdateWorkerCommand(TabRpcClient, {
                                workerid: act.worker_id,
                                status: "working",
                                assignedtask: act.task_id,
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
                            await this.logActivity("worker_wake", `Woke worker ${act.worker_id}`);
                        }
                        break;

                    case "update_task":
                        if (act.task_id && act.status) {
                            await RpcApi.CoworkUpdateTaskCommand(TabRpcClient, {
                                taskid: act.task_id,
                                status: act.status,
                                result: act.result,
                                progress: act.progress,
                            });
                            await this.logActivity("task_update", `Task ${act.task_id} → ${act.status}`);
                        }
                        break;

                    case "create_worker":
                        if (act.tool && act.task_id) {
                            const workerId = await this.createWorkerBlock(act.tool, act.task_id);
                            await RpcApi.CoworkUpdateTaskCommand(TabRpcClient, {
                                taskid: act.task_id,
                                status: "assigned",
                                assignedworker: workerId,
                            });
                            await this.logActivity("worker_create", `Created worker ${workerId}`);
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

    private async createWorkerBlock(tool: string, taskTitle: string): Promise<string> {
        const blockInfo = await RpcApi.BlockInfoCommand(TabRpcClient, this.blockId);
        const tabId = blockInfo.tabid;

        const workerBlockDef: BlockDef = {
            meta: {
                view: "term",
            } as Record<string, string>,
        };
        (workerBlockDef.meta as Record<string, string>)["cowork:worker"] = "true";
        (workerBlockDef.meta as Record<string, string>)["cowork:tool"] = tool;

        const oref = await RpcApi.CreateSubBlockCommand(TabRpcClient, {
            parentblockid: this.blockId,
            blockdef: workerBlockDef,
        });

        const workerBlockId = oref as string;

        await new Promise((resolve) => setTimeout(resolve, 2000));

        const startCmd = this.getWorkerStartCommand(tool);
        await this.sendToTerminal(workerBlockId, startCmd + "\n");

        await RpcApi.CoworkRegisterWorkerCommand(TabRpcClient, {
            workerid: workerBlockId,
            name: `${tool} (${taskTitle.substring(0, 30)})`,
            tool,
            blockid: workerBlockId,
            tabid: tabId,
        });

        return workerBlockId;
    }

    private getWorkerStartCommand(tool: string): string {
        switch (tool) {
            case "claude":
                return "claude";
            case "opencode":
                return "opencode";
            case "cursor":
                return "cursor-agent";
            case "aider":
                return "aider";
            default:
                return tool;
        }
    }

    private async sendToTerminal(blockId: string, text: string): Promise<void> {
        const b64data = stringToBase64(text);
        await RpcApi.ControllerInputCommand(TabRpcClient, {
            blockid: blockId,
            inputdata64: b64data,
        });
    }

    private async logActivity(type: string, description: string): Promise<void> {
        await RpcApi.CoworkAddActivityCommand(TabRpcClient, {
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
- Assign pending tasks to available workers
- Detect stuck/errored workers and send natural language prompts to wake them
- Report completion status

## Communication with Workers
You communicate with workers by writing text into their terminal sessions. The text you output will be typed into their terminal. Be concise and direct.

## Task Status Machine
- pending → assigned: when you assign to a worker
- assigned → working: when worker starts showing activity
- working → done: when worker output indicates completion
- working → failed: when worker shows repeated errors

## Response Format
You MUST respond with valid JSON in this exact format (no markdown, no explanation):
{"actions":[{"type":"assign_task","task_id":"...","worker_id":"...","instruction":"..."}]}

Only take actions when the situation actually warrants it. Do not wake workers that are making progress. Do not reassign tasks that are being worked on.`;
