// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { useState } from "react";
import { cn } from "@/util/util";

export interface WorkerConfigResult {
	runtime: string;
	concurrency: number;
	timeout: number;
	maxRetries: number;
	capabilities: string[];
}

interface WorkerConfigDialogProps {
	className?: string;
	onSubmit: (config: WorkerConfigResult) => void;
	onCancel?: () => void;
}

const DEFAULT_CONFIG: WorkerConfigResult = {
	runtime: "claude",
	concurrency: 3,
	timeout: 300,
	maxRetries: 3,
	capabilities: [],
};

const PREDEFINED_CAPABILITIES = [
	"refactoring",
	"debugging",
	"testing",
	"frontend",
	"backend",
	"documentation",
	"review",
	"optimization",
];

const WORKER_TEMPLATES: Record<string, Omit<WorkerConfigResult, "capabilities"> & { label: string }> = {
	standard: { label: "Standard", runtime: "claude", concurrency: 3, timeout: 300, maxRetries: 3 },
	quick: { label: "Quick", runtime: "claude", concurrency: 1, timeout: 120, maxRetries: 1 },
	power: { label: "Power", runtime: "claude", concurrency: 5, timeout: 600, maxRetries: 5 },
};

export function WorkerConfigDialog({ className, onSubmit, onCancel }: WorkerConfigDialogProps) {
	const [config, setConfig] = useState<WorkerConfigResult>(DEFAULT_CONFIG);
	const [loading, setLoading] = useState(false);

	const set = <K extends keyof WorkerConfigResult>(key: K, value: WorkerConfigResult[K]) =>
		setConfig((prev) => ({ ...prev, [key]: value }));

	const applyTemplate = (key: string) => {
		const t = WORKER_TEMPLATES[key];
		if (t) setConfig({ ...t, capabilities: config.capabilities });
	};

	const handleSubmit = async () => {
		setLoading(true);
		try {
			await onSubmit(config);
		} finally {
			setLoading(false);
		}
	};

	return (
		<div className={cn("flex flex-col gap-4 p-5", className)}>
			<div className="flex items-center justify-between">
				<h2 className="text-base font-semibold text-primary">Create</h2>
				{onCancel && (
					<button className="text-secondary hover:text-primary cursor-pointer text-lg leading-none" onClick={onCancel} disabled={loading}>
						✕
					</button>
				)}
			</div>

			<div className="flex flex-col gap-1.5">
				<label className="text-xs font-medium text-tertiary uppercase tracking-wide">Template</label>
				<div className="grid grid-cols-3 gap-2">
					{(Object.entries(WORKER_TEMPLATES)).map(([key, t]) => (
						<button
							key={key}
							onClick={() => applyTemplate(key)}
							disabled={loading}
							className={cn(
								"px-2.5 py-1.5 text-xs rounded border transition-colors cursor-pointer text-center",
								"border-border/50 hover:border-accent/50",
							)}
						>
							<span className="text-primary">{t.label}</span>
							<span className="text-tertiary block mt-0.5">
								{t.concurrency}c / {t.timeout}s / {t.maxRetries}r
							</span>
						</button>
					))}
				</div>
			</div>

			<div className="grid grid-cols-2 gap-3">
				<div className="flex flex-col gap-1">
					<label className="text-xs font-medium text-tertiary uppercase tracking-wide">Runtime</label>
					<select
						value={config.runtime}
						onChange={(e) => set("runtime", e.target.value)}
						disabled={loading}
						className="px-2.5 py-1.5 bg-base border border-border/50 rounded text-sm text-primary focus:outline-none focus:ring-1 focus:ring-accent"
					>
						<option value="claude">Claude Code</option>
						<option value="opencode">OpenCode</option>
						<option value="cursor">Cursor Agent</option>
						<option value="aider">Aider</option>
					</select>
				</div>
				<div className="flex flex-col gap-1">
					<label className="text-xs font-medium text-tertiary uppercase tracking-wide">Concurrency</label>
					<input
						type="number"
						min={1}
						max={10}
						value={config.concurrency}
						onChange={(e) => set("concurrency", parseInt(e.target.value) || 1)}
						disabled={loading}
						className="px-2.5 py-1.5 bg-base border border-border/50 rounded text-sm text-primary focus:outline-none focus:ring-1 focus:ring-accent"
					/>
				</div>
				<div className="flex flex-col gap-1">
					<label className="text-xs font-medium text-tertiary uppercase tracking-wide">Timeout (s)</label>
					<input
						type="number"
						min={10}
						max={3600}
						value={config.timeout}
						onChange={(e) => set("timeout", parseInt(e.target.value) || 300)}
						disabled={loading}
						className="px-2.5 py-1.5 bg-base border border-border/50 rounded text-sm text-primary focus:outline-none focus:ring-1 focus:ring-accent"
					/>
				</div>
				<div className="flex flex-col gap-1">
					<label className="text-xs font-medium text-tertiary uppercase tracking-wide">Max Retries</label>
					<input
						type="number"
						min={0}
						max={10}
						value={config.maxRetries}
						onChange={(e) => set("maxRetries", parseInt(e.target.value) || 0)}
						disabled={loading}
						className="px-2.5 py-1.5 bg-base border border-border/50 rounded text-sm text-primary focus:outline-none focus:ring-1 focus:ring-accent"
					/>
				</div>
			</div>

			<div className="flex flex-col gap-1.5">
				<label className="text-xs font-medium text-tertiary uppercase tracking-wide">Capabilities</label>
				<div className="flex flex-wrap gap-1.5">
					{PREDEFINED_CAPABILITIES.map((cap) => (
						<button
							key={cap}
							type="button"
							onClick={() =>
								setConfig((prev) => ({
									...prev,
									capabilities: prev.capabilities.includes(cap)
										? prev.capabilities.filter((c) => c !== cap)
										: [...prev.capabilities, cap],
								}))
							}
							disabled={loading}
							className={cn(
								"px-2 py-0.5 text-xs rounded border transition-colors cursor-pointer",
								config.capabilities.includes(cap)
									? "bg-accent/20 border-accent/40 text-accent"
									: "bg-base border-border/50 text-secondary hover:border-border",
							)}
						>
							{cap}
						</button>
					))}
				</div>
			</div>

			<div className="flex items-center gap-2 pt-2 border-t border-border/30">
				<button
					onClick={handleSubmit}
					disabled={loading}
					className="flex-1 px-4 py-2 bg-accent/80 text-primary rounded hover:bg-accent transition-colors cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed text-sm font-medium"
				>
					{loading ? "Creating..." : "Create"}
				</button>
				{onCancel && (
					<button
						onClick={onCancel}
						disabled={loading}
						className="px-4 py-2 border border-border/50 text-secondary rounded hover:bg-base transition-colors cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed text-sm"
					>
						Cancel
					</button>
				)}
			</div>
		</div>
	);
}
