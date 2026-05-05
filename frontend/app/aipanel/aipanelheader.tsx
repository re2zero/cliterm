// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { handleWaveAIContextMenu } from "@/app/aipanel/aipanel-contextmenu";
import { useAtomValue } from "jotai";
import { memo } from "react";
import { cn } from "@/util/util";
import { WaveAIModel } from "./waveai-model";
import { globalStore } from "@/app/store/jotaiStore";
import * as WOS from "@/app/store/wos";
import { getLayoutModelForStaticTab } from "@/layout/lib/layoutModelHooks";

export const AIPanelHeader = memo(() => {
    const model = WaveAIModel.getInstance();
    const widgetAccess = useAtomValue(model.widgetAccessAtom);
    const teamMode = useAtomValue(model.teamModeAtom);
    const inBuilder = model.inBuilder;

    const isTeamBlockOpen = getLayoutModelForStaticTab()?.getter(getLayoutModelForStaticTab().leafs)?.some((leaf) => {
        const blockAtom = WOS.getWaveObjectAtom<Block>(WOS.makeORef("block", leaf.data.blockId));
        const blockData = globalStore.get(blockAtom);
        return blockData?.meta?.view === "team";
    }) ?? false;

    const handleKebabClick = (e: React.MouseEvent) => {
        handleWaveAIContextMenu(e, false);
    };

    const handleContextMenu = (e: React.MouseEvent) => {
        handleWaveAIContextMenu(e, false);
    };

    return (
        <div
            className="py-2 pl-3 pr-1 @xs:p-2 @xs:pl-4 border-b border-gray-600 flex items-center justify-between min-w-0"
            onContextMenu={handleContextMenu}
        >
            <h2 className="text-white text-sm @xs:text-lg font-semibold flex items-center gap-2 flex-shrink-0 whitespace-nowrap">
                <i className="fa fa-sparkles text-accent"></i>
                Wave AI
            </h2>

            <div className="flex items-center flex-shrink-0 whitespace-nowrap">
                {!inBuilder && (
                    <div className="flex items-center text-sm whitespace-nowrap">
                        <span className="text-gray-300 @xs:hidden mr-1 text-[12px]">Context</span>
                        <span className="text-gray-300 hidden @xs:inline mr-2 text-[12px]">Widget Context</span>
                        <button
                            onClick={() => {
                                model.setWidgetAccess(!widgetAccess);
                                setTimeout(() => {
                                    model.focusInput();
                                }, 0);
                            }}
                            className={`relative inline-flex h-6 w-14 items-center rounded-full transition-colors cursor-pointer ${
                                widgetAccess ? "bg-accent-600" : "bg-zinc-600"
                            }`}
                            title={`Widget Access ${widgetAccess ? "ON" : "OFF"}`}
                        >
                            <span
                                className={`absolute inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                                    widgetAccess ? "translate-x-8" : "translate-x-1"
                                }`}
                            />
                            <span
                                className={`relative z-10 text-xs text-white transition-all ${
                                    widgetAccess ? "ml-2.5 mr-6 text-left" : "ml-6 mr-1.5 text-right"
                                }`}
                            >
                                {widgetAccess ? "ON" : "OFF"}
                            </span>
                        </button>

                        <button
                            onClick={() => {
                                model.setTeamMode(!teamMode);
                                setTimeout(() => {
                                    model.focusInput();
                                }, 0);
                            }}
                            className={`ml-2 relative inline-flex h-6 w-14 items-center rounded-full transition-colors cursor-pointer ${
                                teamMode ? "bg-purple-600" : "bg-zinc-600"
                            }`}
                            title={`Team Mode ${teamMode ? "ON" : "OFF"}`}
                        >
                            <span
                                className={`absolute inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                                    teamMode ? "translate-x-8" : "translate-x-1"
                                }`}
                            />
                            <span
                                className={`relative z-10 text-xs text-white transition-all ${
                                    teamMode ? "ml-2.5 mr-6 text-left" : "ml-6 mr-1.5 text-right"
                                }`}
                            >
                                {teamMode ? "ON" : "OFF"}
                            </span>
                        </button>

                        <button
                            onClick={() => {
                                model.openTeamBlock();
                            }}
                            className={cn(
                                "ml-2 px-2.5 py-1 text-[10px] font-medium rounded transition-all cursor-pointer border",
                                isTeamBlockOpen
                                    ? "bg-accent/25 text-accent border-accent/40"
                                    : "border-white/10 text-slate-400 hover:text-slate-200 hover:border-white/20",
                            )}
                            title={isTeamBlockOpen ? "Close Team Dashboard" : "Open Team Dashboard"}
                        >
                            {isTeamBlockOpen ? "✕ Team" : "Team"}
                        </button>
                    </div>
                )}

                <button
                    onClick={handleKebabClick}
                    className="text-gray-400 hover:text-white cursor-pointer transition-colors p-1 rounded flex-shrink-0 ml-2 focus:outline-none"
                    title="More options"
                >
                    <i className="fa fa-ellipsis-vertical"></i>
                </button>
            </div>
        </div>
    );
});

AIPanelHeader.displayName = "AIPanelHeader";
