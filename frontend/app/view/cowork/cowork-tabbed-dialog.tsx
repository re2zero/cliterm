// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { useState } from "react";
import { CoworkViewModel } from "./cowork-model";
import { RuntimeDetectionPanel } from "./runtime-detection-panel";
import { WorkerConfigPanel } from "./worker-config-panel";
import { cn } from "@/util/util";

export interface CoworkTabbedDialogProps {
	className?: string;
	onClose?: () => void;
}

type TabId = "create-worker" | "runtime" | "config" | "output";

interface Tab {
	id: TabId;
	label: string;
}

const TABS: Tab[] = [
	{ id: "create-worker", label: "Create Worker" },
	{ id: "runtime", label: "Runtime" },
	{ id: "config", label: "Config" },
	{ id: "output", label: "Output" },
];

export function CoworkTabbedDialog({ className, onClose }: CoworkTabbedDialogProps) {
	const [activeTab, setActiveTab] = useState<TabId>("create-worker");
	const [workerConfig, setWorkerConfig] = useState({
		runtime: "claude",
		concurrency: 3,
		timeout: 300,
		maxRetries: 3,
	});

	const handleTabChange = (tabId: TabId) => {
		setActiveTab(tabId);
	};

	return (
		<div className={cn("flex flex-col rounded border border-border/50 bg-base shadow-lg", className)}>
			<div className="flex items-center justify-between p-3 border-b border-border/50">
				<div className="flex gap-1">
					{TABS.map((tab) => (
						<button
							key={tab.id}
							className={cn(
								"px-3 py-1.5 text-sm rounded transition-colors cursor-pointer",
								activeTab === tab.id
									? "bg-accent text-primary font-medium"
									: "text-gray-600 hover:text-gray-900 hover:bg-gray-100"
							)}
							onClick={() => handleTabChange(tab.id)}
						>
							{tab.label}
						</button>
					))}
				</div>
				{onClose && (
					<button
						className="text-gray-500 hover:text-gray-700 cursor-pointer"
						onClick={onClose}
					>
						✕
					</button>
				)}
			</div>

			<div className="flex-1 overflow-auto">
				{activeTab === "create-worker" && (
					<div className="p-4">
						<h3 className="text-lg font-semibold mb-4">Create Worker</h3>
						<div className="flex flex-col gap-4">
							<div className="p-3 bg-yellow-50 border border-yellow-200 rounded text-sm text-yellow-800">
								<p>Create Worker functionality is available in the main Workers panel.</p>
								<p className="mt-2">Use the "Add Worker" button in the Workers section to create new workers.</p>
							</div>
						</div>
					</div>
				)}

				{activeTab === "runtime" && (
					<RuntimeDetectionPanel className="border-0 rounded-none shadow-none" />
				)}

				{activeTab === "config" && (
					<WorkerConfigPanel
						config={workerConfig}
						onConfigChange={(newConfig) => setWorkerConfig({ ...workerConfig, ...newConfig })}
					/>
				)}

				{activeTab === "output" && (
					<div className="p-4">
						<h3 className="text-lg font-semibold mb-4">Task Output History</h3>
						<div className="p-3 bg-blue-50 border border-blue-200 rounded text-sm text-blue-800">
							<p>Output history is available per-task.</p>
							<p className="mt-2">Click on a task in the Tasks section to view its output history.</p>
						</div>
					</div>
				)}
			</div>
		</div>
	);
}
