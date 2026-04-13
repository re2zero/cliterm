// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

interface AssistantAction {
    actions: Array<{
        type: "assign_task" | "wake_worker" | "update_task" | "create_worker" | "noop";
        task_id?: string;
        worker_id?: string;
        instruction?: string;
        message?: string;
        status?: string;
        result?: string;
        progress?: string;
        tool?: string;
        reason?: string;
    }>;
}

interface WorkerOutput {
    lines?: string[];
    totalLines?: number;
    lastUpdated?: number;
    hashChanged?: boolean;
    error?: string;
}
