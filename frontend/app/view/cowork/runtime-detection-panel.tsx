// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { useAtomValue } from "jotai";
import { useEffect, useState } from "react";
import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";
import { cn } from "@/util/util";

interface RuntimeDetectionPanelProps {
	className?: string;
}

export function RuntimeDetectionPanel({ className }: RuntimeDetectionPanelProps) {
	const [runtimes, setRuntimes] = useState<AIRuntime[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);

	useEffect(() => {
		const detectRuntimes = async () => {
			try {
				setLoading(true);
				setError(null);
				const result = await RpcApi.CoworkDetectRuntimesCommand(TabRpcClient);
				setRuntimes(result.runtimes);
			} catch (err) {
				const errorMsg = err instanceof Error ? err.message : String(err);
				setError(errorMsg);
			} finally {
				setLoading(false);
			}
		};

		detectRuntimes();
	}, []);

	if (loading) {
		return (
			<div className={cn("flex flex-col", className)}>
				<div className="p-4 text-sm text-tertiary">Detecting AI runtimes...</div>
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

	if (runtimes.length === 0) {
		return (
			<div className={cn("flex flex-col", className)}>
				<div className="p-4 text-sm text-tertiary">No AI runtimes detected</div>
			</div>
		);
	}

	return (
		<div className={cn("flex flex-col", className)}>
			<div className="p-3 border-b border-border/50">
				<h3 className="text-sm font-medium text-secondary">Detected AI Runtimes</h3>
			</div>
			<div className="flex-1 overflow-y-auto">
				{runtimes.map((runtime, index) => (
					<div
						key={runtime.name}
						className={cn(
							"flex items-center justify-between p-3",
							index !== runtimes.length - 1 && "border-b border-border/30"
						)}
					>
						<div className="flex flex-col">
							<span className="text-sm font-medium text-primary">{runtime.display_name}</span>
							<span className="text-xs text-tertiary">{runtime.command}</span>
							{runtime.version && (
								<span className="text-xs text-secondary">{runtime.version}</span>
							)}
						</div>
						<div
							className={cn(
								"px-2 py-1 text-xs font-medium rounded",
								runtime.status === "online"
									? "bg-green-500/10 text-green-400"
									: "bg-base/50 text-tertiary"
							)}
						>
							{runtime.status}
						</div>
					</div>
				))}
			</div>
		</div>
	);
}
