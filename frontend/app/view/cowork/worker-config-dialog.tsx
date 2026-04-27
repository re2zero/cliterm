// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { useAtomValue } from "jotai";
import { useState } from "react";
import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";
import { cn } from "@/util/util";

interface WorkerConfigDialogProps {
	className?: string;
	onWorkerCreated?: (workerId: string) => void;
	onCancel?: () => void;
}

interface WorkerConfig {
	tool: string;
	runtime: string;
	concurrency: number;
	timeout: number;
	maxRetries: number;
}

const DEFAULT_CONFIG: WorkerConfig = {
	tool: "claude",
	runtime: "claude",
	concurrency: 3,
	timeout: 300,
	maxRetries: 3,
};

const WORKER_TEMPLATES: Record<string, Omit<WorkerConfig, "tool"> & { name: string; description: string }> = {
	standard: {
		name: "Standard Worker",
		description: "Balanced performance for general tasks",
		runtime: "claude",
		concurrency: 3,
		timeout: 300,
		maxRetries: 3,
	},
	quick: {
		name: "Quick Worker",
		description: "Fast tasks with low latency",
		runtime: "claude",
		concurrency: 1,
		timeout: 120,
		maxRetries: 1,
	},
	power: {
		name: "Power Worker",
		description: "Heavy tasks with high throughput",
		runtime: "claude",
		concurrency: 5,
		timeout: 600,
		maxRetries: 5,
	},
};

export function WorkerConfigDialog({ className, onWorkerCreated, onCancel }: WorkerConfigDialogProps) {
	const [config, setConfig] = useState<WorkerConfig>(DEFAULT_CONFIG);
	const [selectedTemplate, setSelectedTemplate] = useState<string>("standard");
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState<string | null>(null);

	const handleConfigChange = <K extends keyof WorkerConfig>(key: K, value: WorkerConfig[K]) => {
		setConfig((prev) => ({ ...prev, [key]: value }));
	};

	const handleTemplateSelect = (templateKey: string) => {
		setSelectedTemplate(templateKey);
		const template = WORKER_TEMPLATES[templateKey];
		if (template) {
			setConfig({
				tool: template.runtime,
				runtime: template.runtime,
				concurrency: template.concurrency,
				timeout: template.timeout,
				maxRetries: template.maxRetries,
			});
		}
	};

	const handleSubmit = async () => {
		try {
			setLoading(true);
			setError(null);

			const result = await RpcApi.CoworkExecuteTaskCommand(TabRpcClient, {
				workerid: "",
				taskid: "",
				command: `create-worker --runtime ${config.runtime} --concurrency ${config.concurrency} --timeout ${config.timeout} --max-retries ${config.maxRetries}`,
			});

			if (result.success && result.blockid) {
				onWorkerCreated?.(result.blockid);
			} else {
				setError(result.error || "Failed to create worker");
			}
		} catch (err) {
			const errorMsg = err instanceof Error ? err.message : String(err);
			setError(errorMsg);
		} finally {
			setLoading(false);
		}
	};

	return (
		<div className={cn("flex flex-col gap-4 p-6", className)}>
			<div className="flex items-center justify-between">
				<h2 className="text-lg font-semibold text-gray-900">Create Worker</h2>
				{onCancel && (
					<button
						className="text-gray-400 hover:text-gray-600 cursor-pointer"
						onClick={onCancel}
						disabled={loading}
					>
						✕
					</button>
				)}
			</div>

			{error && (
				<div className="px-4 py-2 bg-red-50 border border-red-200 rounded text-sm text-red-600">
					{error}
				</div>
			)}

			<div className="flex flex-col gap-2">
				<label className="text-sm font-medium text-gray-700">Worker Template</label>
				<div className="grid grid-cols-3 gap-2">
					{(Object.keys(WORKER_TEMPLATES) as Array<keyof typeof WORKER_TEMPLATES>).map((key) => {
						const template = WORKER_TEMPLATES[key];
						return (
							<button
								key={key}
								onClick={() => handleTemplateSelect(key)}
								disabled={loading}
								className={cn(
									"px-3 py-2 text-sm rounded border transition-colors cursor-pointer",
									selectedTemplate === key
										? "bg-accent/20 border-accent text-accent"
										: "bg-white border-gray-300 text-gray-700 hover:bg-gray-50"
								)}
							>
								<div className="font-medium">{template.name}</div>
								<div className="text-xs text-gray-500 mt-0.5">{template.description}</div>
							</button>
						);
					})}
				</div>
			</div>

			<div className="flex flex-col gap-3">
				<div className="flex flex-col gap-1">
					<label className="text-sm font-medium text-gray-700">Runtime</label>
					<select
						value={config.runtime}
						onChange={(e) => handleConfigChange("runtime", e.target.value)}
						disabled={loading}
						className="px-3 py-2 border border-gray-300 rounded focus:ring-2 focus:ring-accent focus:outline-none"
					>
						<option value="claude">Claude Code</option>
						<option value="opencode">OpenCode</option>
						<option value="cursor">Cursor Agent</option>
						<option value="aider">Aider</option>
					</select>
				</div>

				<div className="flex flex-col gap-1">
					<label className="text-sm font-medium text-gray-700">
						Concurrency (1-10)
					</label>
					<input
						type="number"
						min={1}
						max={10}
						value={config.concurrency}
						onChange={(e) => handleConfigChange("concurrency", parseInt(e.target.value) || 1)}
						disabled={loading}
						className="px-3 py-2 border border-gray-300 rounded focus:ring-2 focus:ring-accent focus:outline-none"
					/>
				</div>

				<div className="flex flex-col gap-1">
					<label className="text-sm font-medium text-gray-700">Timeout (seconds)</label>
					<input
						type="number"
						min={10}
						max={3600}
						value={config.timeout}
						onChange={(e) => handleConfigChange("timeout", parseInt(e.target.value) || 300)}
						disabled={loading}
						className="px-3 py-2 border border-gray-300 rounded focus:ring-2 focus:ring-accent focus:outline-none"
					/>
				</div>

				<div className="flex flex-col gap-1">
					<label className="text-sm font-medium text-gray-700">Max Retries</label>
					<input
						type="number"
						min={0}
						max={10}
						value={config.maxRetries}
						onChange={(e) => handleConfigChange("maxRetries", parseInt(e.target.value) || 0)}
						disabled={loading}
						className="px-3 py-2 border border-gray-300 rounded focus:ring-2 focus:ring-accent focus:outline-none"
					/>
				</div>
			</div>

			<div className="flex items-center gap-2 pt-2">
				<button
					onClick={handleSubmit}
					disabled={loading}
					className="flex-1 px-4 py-2 bg-accent/80 text-primary rounded hover:bg-accent transition-colors cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
				>
					{loading ? "Creating..." : "Create Worker"}
				</button>
				{onCancel && (
					<button
						onClick={onCancel}
						disabled={loading}
						className="px-4 py-2 border border-gray-300 text-gray-700 rounded hover:bg-gray-50 transition-colors cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
					>
						Cancel
					</button>
				)}
			</div>
		</div>
	);
}
