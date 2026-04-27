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

interface TaskOutput {
	timestamp: number;
	content: string;
	type?: string;
}

export function TaskOutputHistory({ taskId, className }: TaskOutputHistoryProps) {
	const [outputs, setOutputs] = useState<TaskOutput[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);

	useEffect(() => {
		const loadOutputs = async () => {
			try {
				setLoading(true);
				setError(null);
				const result = await RpcApi.CoworkGetTaskOutputHistoryCommand(TabRpcClient, taskId);
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

	const formatTime = (timestamp: number) => {
		return new Date(timestamp * 1000).toLocaleTimeString();
	};

	if (loading) {
		return (
			<div className={cn("flex flex-col", className)}>
				<div className="p-4 text-sm text-gray-500">Loading output history...</div>
			</div>
		);
	}

	if (error) {
		return (
			<div className={cn("flex flex-col", className)}>
				<div className="p-4 text-sm text-red-500">Error: {error}</div>
			</div>
		);
	}

	if (outputs.length === 0) {
		return (
			<div className={cn("flex flex-col", className)}>
				<div className="p-4 text-sm text-gray-500">No output history available</div>
			</div>
		);
	}

	return (
		<div className={cn("flex flex-col", className)}>
			<div className="p-3 border-b border-gray-200">
				<h3 className="text-sm font-medium text-gray-700">Output History</h3>
			</div>
			<div className="flex-1 overflow-y-auto p-3">
				<div className="flex flex-col gap-2">
					{outputs.map((output, index) => (
						<div
							key={index}
							className="flex flex-col gap-1 p-2 rounded bg-gray-50 border border-gray-200"
						>
							<div className="flex items-center gap-2">
								<span className="text-xs text-gray-500">{formatTime(output.timestamp)}</span>
								{output.type && (
									<span className="text-xs px-1.5 py-0.5 bg-blue-100 text-blue-700 rounded">
										{output.type}
									</span>
								)}
							</div>
							<pre className="text-xs text-gray-700 whitespace-pre-wrap font-mono">
								{output.content}
							</pre>
						</div>
					))}
				</div>
			</div>
		</div>
	);
}
