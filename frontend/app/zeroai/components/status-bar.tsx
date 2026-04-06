// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { makeIconClass } from "@/util/util";
import clsx from "clsx";
import * as React from "react";
import type { ZeroAiAgentInfo, ZeroAiSession } from "../types";
import "./status-bar.scss";

export interface StatusBarProps {
    session?: ZeroAiSession;
    agentInfo?: ZeroAiAgentInfo;
    onWorkDirClick?: () => void;
    className?: string;
}

export const StatusBar = React.memo(({ session, agentInfo, onWorkDirClick, className }: StatusBarProps) => {
    const getBackendIcon = React.useCallback((backend: string): string => {
        const iconMap: Record<string, string> = {
            claude: "fa-solid fa-brain",
            qwen: "fa-solid fa-sparkles",
            codex: "fa-solid fa-code",
            opencode: "fa-solid fa-code-branch",
            custom: "fa-solid fa-robot",
        };
        return iconMap[backend.toLowerCase()] || iconMap.custom;
    }, []);

    return (
        <div className={clsx("status-bar", className)}>
            <div className="status-bar-content">
                {session?.backend && (
                    <div className="status-bar-section status-bar-backend">
                        <div className="status-bar-item">
                            <i className={makeIconClass(getBackendIcon(session.backend), false)} />
                            <span className="status-bar-value">{session.backend}</span>
                            {session.model && (
                                <>
                                    <span className="status-bar-sep">·</span>
                                    <span className="status-bar-model-name">{session.model}</span>
                                </>
                            )}
                        </div>
                    </div>
                )}

                {session?.workDir && (
                    <div
                        className={clsx("status-bar-section status-bar-workdir", {
                            clickable: onWorkDirClick != null,
                        })}
                        onClick={onWorkDirClick}
                    >
                        <div className="status-bar-item">
                            <i className="fa-solid fa-folder" />
                            <span className="status-bar-value workdir-path" title={session.workDir}>
                                {session.workDir}
                            </span>
                        </div>
                    </div>
                )}
            </div>

            <div className="status-bar-actions">
                {agentInfo?.enabled && (
                    <div className={clsx("status-bar-item status-indicator", { online: true })}>
                        <span className="status-dot" />
                        <span className="status-text">Online</span>
                    </div>
                )}
            </div>
        </div>
    );
});

StatusBar.displayName = "StatusBar";
