// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { useEffect, useState } from "react";
import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";
import { cn } from "@/util/util";

interface WorkerConfigPanelProps {
	config: {
		runtime?: string;
		concurrency?: number;
		timeout?: number;
		maxRetries?: number;
	};
	onConfigChange: (config: WorkerConfigPanelProps["config"]) => void;
}

export function WorkerConfigPanel({ config, onConfigChange }: WorkerConfigPanelProps) {
	const [runtimes, setRuntimes] = useState<AIRuntime[]>([]);

	useEffect(() => {
		const loadRuntimes = async () => {
			try {
				const result = await RpcApi.CoworkDetectRuntimesCommand(TabRpcClient);
				setRuntimes(result.runtimes);
			} catch (err) {
				console.error("Failed to load runtimes:", err);
			}
		};

		loadRuntimes();
	}, []);

	const handleFieldChange = (field: keyof WorkerConfigPanelProps["config"], value: string | number) => {
		onConfigChange({
			...config,
			[field]: value,
		});
	};

	return (
		<div className="p-4">
			<h3 className="text-lg font-semibold mb-4">Worker Configuration</h3>
			<div className="flex flex-col gap-4">
				<div className="flex flex-col gap-2">
					<label className="text-sm font-medium">Runtime</label>
					<select
						className="border border-border/50 rounded px-3 py-2 text-sm"
						value={config.runtime || ""}
						onChange={(e) => handleFieldChange("runtime", e.target.value)}
					>
						<option value="">Select Runtime</option>
						{runtimes.map((rt) => (
							<option key={rt.name} value={rt.name}>
								{rt.name} {rt.version ? `(${rt.version})` : ""} {rt.status === "online" ? "✓" : "⚠"}
							</option>
						))}
					</select>
				</div>

				<div className="flex flex-col gap-2">
					<label className="text-sm font-medium">Concurrency</label>
					<input
						type="number"
						min="1"
						max="10"
						className="border border-border/50 rounded px-3 py-2 text-sm"
						value={config.concurrency || 3}
						onChange={(e) => handleFieldChange("concurrency", parseInt(e.target.value) || 1)}
					/>
					<span className="text-xs text-gray-500">Number of tasks this worker can handle simultaneously (1-10)</span>
				</div>

				<div className="flex flex-col gap-2">
					<label className="text-sm font-medium">Timeout (seconds)</label>
					<input
						type="number"
						min="30"
						max="3600"
						className="border border-border/50 rounded px-3 py-2 text-sm"
						value={config.timeout || 300}
						onChange={(e) => handleFieldChange("timeout", parseInt(e.target.value) || 300)}
					/>
					<span className="text-xs text-gray-500">Maximum time to wait for task completion (30-3600s)</span>
				</div>

				<div className="flex flex-col gap-2">
					<label className="text-sm font-medium">Max Retries</label>
					<input
						type="number"
						min="0"
						max="10"
						className="border border-border/50 rounded px-3 py-2 text-sm"
						value={config.maxRetries || 3}
						onChange={(e) => handleFieldChange("maxRetries", parseInt(e.target.value) || 0)}
					/>
					<span className="text-xs text-gray-500">Number of retry attempts on failure (0-10)</span>
				</div>
			</div>
		</div>
	);
}
