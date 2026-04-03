// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { cn, makeIconClass } from "@/util/util";
import * as React from "react";
import "./zeroai-header.scss";

export interface ZeroAIHeaderProps {
    showSettings?: boolean;
    onToggleSettings?: () => void;
    className?: string;
}

export const ZeroAIHeader = React.memo(({ showSettings = false, onToggleSettings, className }: ZeroAIHeaderProps) => {
    return (
        <div className={cn("zeroai-header", className)}>
            <div className="zeroai-header-title">
                {showSettings && (
                    <button className="zeroai-header-back" onClick={onToggleSettings} title="Back to chat">
                        <i className="fa-solid fa-arrow-left" />
                    </button>
                )}
                <i className={makeIconClass("robot", false)} />
                <span>{showSettings ? "Settings" : "ZeroAI"}</span>
            </div>
            {!showSettings && (
                <button
                    className={cn("zeroai-header-btn", showSettings && "active")}
                    onClick={onToggleSettings}
                    title="Settings"
                >
                    <i className={makeIconClass("gear", false)} />
                </button>
            )}
        </div>
    );
});

ZeroAIHeader.displayName = "ZeroAIHeader";
