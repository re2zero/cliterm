// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { useEffect, useState } from "react";
import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";
import { cn } from "@/util/util";

interface TaskOutputHistoryProps {
	taskId: string;
	className?: string;
}

export function TaskOutputHistory({ taskId, className }: TaskOutputHistoryProps) {
	const [outputs, setOutputs] = useState<TeamTaskOutput[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);

	useEffect(() => {
		const loadOutputs = async () => {
			try {
				setLoading(true);
				setError(null);
				const result = await RpcApi.TeamGetTaskOutputHistoryCommand(TabRpcClient, taskId);
				setOutputs(result);
			} catch (err) {
				const errorMsg = err instanceof Error ? err.message : String(err);
				setError(errorMsg);
			} finally {
				setLoading(false);
			}
		};

		loadOutputs();
	}, [taskId]);

	const formatTime = (timestamp: string) => {
		return new Date(timestamp).toLocaleTimeString();
	};

	if (loading) {
		return (
			<div className={cn("flex flex-col", className)}>
				<div className="p-4 text-sm text-tertiary">Loading output history...</div>
			</div>
		);
	}

	if (error) {
		return (
			<div className={cn("flex flex-col", className)}>
				<div className="p-4 text-sm text-red-400">Error: {error}</div>
			</div>
		);
	}

	if (outputs.length === 0) {
		return (
			<div className={cn("flex flex-col", className)}>
				<div className="p-4 text-sm text-tertiary">No output history available</div>
			</div>
		);
	}

	return (
		<div className={cn("flex flex-col", className)}>
			<div className="p-3 border-b border-border/50">
				<h3 className="text-sm font-medium text-secondary">Output History</h3>
			</div>
			<div className="flex-1 overflow-y-auto p-3">
				<div className="flex flex-col gap-2">
					{outputs.map((output, index) => (
						<div
							key={index}
							className="flex flex-col gap-1 p-2 rounded bg-base/50 border border-border/30"
						>
							<div className="flex items-center gap-2">
								<span className="text-xs text-tertiary">{formatTime(output.timestamp)}</span>
								{output.type && (
									<span className="text-xs px-1.5 py-0.5 bg-blue-500/10 text-blue-400 rounded">
										{output.type}
									</span>
								)}
							</div>
							<pre className="text-xs text-secondary whitespace-pre-wrap font-mono">
								{output.content}
							</pre>
						</div>
					))}
				</div>
			</div>
		</div>
	);
}
