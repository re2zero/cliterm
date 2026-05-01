// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { formatFileSizeError, isAcceptableFile, validateFileSize } from "@/app/aipanel/ai-utils";
import { waveAIHasFocusWithin } from "@/app/aipanel/waveai-focus-utils";
import { type WaveAIModel } from "@/app/aipanel/waveai-model";
import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";
import { waveEventSubscribeSingle } from "@/app/store/wps";
import { Tooltip } from "@/element/tooltip";
import { cn } from "@/util/util";
import { useAtom, useAtomValue } from "jotai";
import { memo, useCallback, useEffect, useRef, useState } from "react";

type MentionMode = "member" | "project";

interface AIPanelInputProps {
    onSubmit: (e: React.FormEvent) => void;
    status: string;
    model: WaveAIModel;
}

export interface AIPanelInputRef {
    focus: () => void;
    resize: () => void;
    scrollToBottom: () => void;
}

export const AIPanelInput = memo(({ onSubmit, status, model }: AIPanelInputProps) => {
    const [input, setInput] = useAtom(model.inputAtom);
    const isFocused = useAtomValue(model.isWaveAIFocusedAtom);
    const isChatEmpty = useAtomValue(model.isChatEmptyAtom);
    const textareaRef = useRef<HTMLTextAreaElement>(null);
    const fileInputRef = useRef<HTMLInputElement>(null);
    const isPanelOpen = useAtomValue(model.getPanelVisibleAtom());
    const [showMention, setShowMention] = useState(false);
    const [mentionMode, setMentionMode] = useState<MentionMode>("member");
    const [mentionQuery, setMentionQuery] = useState("");
    const [mentionStartPos, setMentionStartPos] = useState(-1);
    const [mentionSelectedIdx, setMentionSelectedIdx] = useState(0);
    const [workersCache, setWorkersCache] = useState<TeamWorker[] | null>(null);
    const [membersCache, setMembersCache] = useState<TeamMember[] | null>(null);
    const [projectsCache, setProjectsCache] = useState<TeamProject[] | null>(null);
    const teamFetchedRef = useRef(false);
    const mentionListRef = useRef<HTMLDivElement>(null);

    let placeholder: string;
    if (!isChatEmpty) {
        placeholder = "Continue...";
    } else if (model.inBuilder) {
        placeholder = "What would you like to build...";
    } else {
        placeholder = "Ask Wave AI anything...";
    }

    const fetchTeamData = useCallback(async () => {
        try {
            const [workers, members, projects] = await Promise.all([
                RpcApi.TeamListWorkersCommand(TabRpcClient, ""),
                RpcApi.TeamListMembersCommand(TabRpcClient, {}),
                RpcApi.TeamListProjectsCommand(TabRpcClient),
            ]);
            setWorkersCache(workers);
            setMembersCache(members);
            setProjectsCache(projects);
        } catch {
            setWorkersCache([]);
            setMembersCache([]);
            setProjectsCache([]);
        }
    }, []);

    useEffect(() => {
        const handleTeamUpdate = () => {
            setWorkersCache(null);
            setMembersCache(null);
            setProjectsCache(null);
            teamFetchedRef.current = false;
        };
        const unsubMember = waveEventSubscribeSingle({
            eventType: "team:memberupdate",
            handler: handleTeamUpdate,
        });
        const unsubProject = waveEventSubscribeSingle({
            eventType: "team:projectupdate",
            handler: handleTeamUpdate,
        });
        return () => {
            unsubMember?.();
            unsubProject?.();
        };
    }, [fetchTeamData]);

    const ensureTeamData = useCallback(() => {
        if (!teamFetchedRef.current) {
            teamFetchedRef.current = true;
            fetchTeamData();
        }
    }, [fetchTeamData]);

    const filteredWorkers = (() => {
        if (!workersCache) return [];
        const q = mentionQuery.toLowerCase();
        return workersCache.filter((w) => w.name.toLowerCase().includes(q));
    })();

    const filteredMembers = (() => {
        if (!membersCache) return [];
        const q = mentionQuery.toLowerCase();
        return membersCache.filter((m) => m.name.toLowerCase().includes(q));
    })();

    const filteredProjects = (() => {
        if (!projectsCache) return [];
        const q = mentionQuery.toLowerCase();
        return projectsCache.filter((p) => p.name.toLowerCase().includes(q));
    })();

    const memberItems = [
        { type: "all" as const, name: "All Members", icon: "fa-users", status: "" },
        ...filteredWorkers.map((w) => ({
            type: "worker" as const,
            name: w.name,
            icon: "fa-robot",
            status: w.status,
        })),
    ];

    const projectItems = filteredProjects.map((p) => ({
        type: "project" as const,
        name: p.name,
        icon: "fa-folder",
        detail: p.path,
    }));

    const mentionItems = mentionMode === "member" ? memberItems : projectItems;

    const closeMention = useCallback(() => {
        setShowMention(false);
        setMentionMode("member");
        setMentionQuery("");
        setMentionStartPos(-1);
        setMentionSelectedIdx(0);
    }, []);

    const insertMention = useCallback(
        (name: string) => {
            const textarea = textareaRef.current;
            if (textarea == null || mentionStartPos < 0) return;
            const before = input.slice(0, mentionStartPos);
            const after = input.slice(textarea.selectionStart);
            const prefix = mentionMode === "member" ? "@" : "#";
            const newValue = before + prefix + name + " " + after;
            setInput(newValue);
            closeMention();
            requestAnimationFrame(() => {
                const pos = mentionStartPos + name.length + 2;
                textarea.selectionStart = pos;
                textarea.selectionEnd = pos;
            });
        },
        [input, mentionStartPos, mentionMode, setInput, closeMention]
    );

    useEffect(() => {
        if (!showMention || !mentionListRef.current) return;
        const selected = mentionListRef.current.querySelector("[data-mention-selected='true']");
        selected?.scrollIntoView({ block: "nearest" });
    }, [mentionSelectedIdx, showMention]);

    const resizeTextarea = useCallback(() => {
        const textarea = textareaRef.current;
        if (!textarea) return;

        textarea.style.height = "auto";
        const scrollHeight = textarea.scrollHeight;
        const maxHeight = 7 * 24;
        textarea.style.height = `${Math.min(scrollHeight, maxHeight)}px`;
    }, []);

    useEffect(() => {
        const inputRefObject: React.RefObject<AIPanelInputRef> = {
            current: {
                focus: () => {
                    textareaRef.current?.focus();
                },
                resize: resizeTextarea,
                scrollToBottom: () => {
                    const textarea = textareaRef.current;
                    if (textarea) {
                        textarea.scrollTop = textarea.scrollHeight;
                    }
                },
            },
        };
        model.registerInputRef(inputRefObject);
    }, [model, resizeTextarea]);

    const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
        if (showMention) {
            if (e.key === "ArrowDown") {
                e.preventDefault();
                setMentionSelectedIdx((i) => Math.min(i + 1, mentionItems.length - 1));
                return;
            }
            if (e.key === "ArrowUp") {
                e.preventDefault();
                setMentionSelectedIdx((i) => Math.max(i - 1, 0));
                return;
            }
            if (e.key === "Enter" && mentionItems[mentionSelectedIdx]) {
                e.preventDefault();
                const item = mentionItems[mentionSelectedIdx];
                insertMention(item.type === "all" ? "all" : item.name);
                return;
            }
            if (e.key === "Escape") {
                e.preventDefault();
                closeMention();
                return;
            }
        }
        const isComposing = e.nativeEvent?.isComposing || e.keyCode == 229;
        if (e.key === "Enter" && !e.shiftKey && !isComposing) {
            e.preventDefault();
            onSubmit(e as any);
        }
    };

    const handleFocus = useCallback(() => {
        model.requestWaveAIFocus();
    }, [model]);

    const handleBlur = useCallback(
        (e: React.FocusEvent) => {
            if (e.relatedTarget === null) {
                return;
            }

            if (waveAIHasFocusWithin(e.relatedTarget)) {
                return;
            }

            model.requestNodeFocus();
        },
        [model]
    );

    useEffect(() => {
        resizeTextarea();
    }, [input, resizeTextarea]);

    useEffect(() => {
        if (isPanelOpen) {
            resizeTextarea();
        }
    }, [isPanelOpen, resizeTextarea]);

    const handleUploadClick = () => {
        fileInputRef.current?.click();
    };

    const handleFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
        const files = Array.from(e.target.files || []);
        const acceptableFiles = files.filter(isAcceptableFile);

        for (const file of acceptableFiles) {
            const sizeError = validateFileSize(file);
            if (sizeError) {
                model.setError(formatFileSizeError(sizeError));
                if (e.target) {
                    e.target.value = "";
                }
                return;
            }
            await model.addFile(file);
        }

        if (acceptableFiles.length < files.length) {
            console.warn(`${files.length - acceptableFiles.length} files were rejected due to unsupported file types`);
        }

        if (e.target) {
            e.target.value = "";
        }
    };

    const detectMentionTrigger = (val: string, cursorPos: number) => {
        const beforeCursor = val.slice(0, cursorPos);

        const atIdx = beforeCursor.lastIndexOf("@");
        if (atIdx >= 0 && (atIdx === 0 || /\s/.test(beforeCursor[atIdx - 1]))) {
            const queryText = beforeCursor.slice(atIdx + 1);
            if (!/\s/.test(queryText)) {
                setMentionMode("member");
                setShowMention(true);
                setMentionStartPos(atIdx);
                setMentionQuery(queryText);
                setMentionSelectedIdx(0);
                ensureTeamData();
                return;
            }
        }

        const hashIdx = beforeCursor.lastIndexOf("#");
        if (hashIdx >= 0 && (hashIdx === 0 || /\s/.test(beforeCursor[hashIdx - 1]))) {
            const queryText = beforeCursor.slice(hashIdx + 1);
            if (!/\s/.test(queryText)) {
                setMentionMode("project");
                setShowMention(true);
                setMentionStartPos(hashIdx);
                setMentionQuery(queryText);
                setMentionSelectedIdx(0);
                ensureTeamData();
                return;
            }
        }

        closeMention();
    };

    return (
        <div className={cn("border-t", isFocused ? "border-accent/50" : "border-gray-600")}>
            <input
                ref={fileInputRef}
                type="file"
                multiple
                accept="image/*,.pdf,.txt,.md,.js,.jsx,.ts,.tsx,.go,.py,.java,.c,.cpp,.h,.hpp,.html,.css,.scss,.sass,.json,.xml,.yaml,.yml,.sh,.bat,.sql"
                onChange={handleFileChange}
                className="hidden"
            />
            <form onSubmit={onSubmit}>
                <div className="relative">
                    <textarea
                        ref={textareaRef}
                        value={input}
                        onChange={(e) => {
                            const val = e.target.value;
                            const cursorPos = e.target.selectionStart;
                            setInput(val);
                            detectMentionTrigger(val, cursorPos);
                        }}
                        onKeyDown={handleKeyDown}
                        onFocus={handleFocus}
                        onBlur={handleBlur}
                        placeholder={placeholder}
                        className={cn(
                            "w-full  text-white px-2 py-2 pr-5 focus:outline-none resize-none overflow-auto bg-zinc-800/50"
                        )}
                        style={{ fontSize: "13px" }}
                        rows={2}
                    />
                    {showMention && mentionItems.length > 0 && (
                        <div ref={mentionListRef} className="absolute bottom-full left-0 right-0 mb-1 mx-2 bg-zinc-900 border border-gray-600 rounded shadow-lg max-h-48 overflow-y-auto z-50">
                            {mentionItems.map((item, idx) => (
                                <div
                                    key={item.type === "all" ? "all" : item.name}
                                    data-mention-selected={idx === mentionSelectedIdx ? "true" : undefined}
                                    className={cn(
                                        "px-3 py-1.5 text-sm cursor-pointer hover:bg-zinc-700 flex items-center gap-2",
                                        idx === mentionSelectedIdx && "bg-zinc-700"
                                    )}
                                    onMouseDown={(e) => {
                                        e.preventDefault();
                                        insertMention(item.type === "all" ? "all" : item.name);
                                    }}
                                >
                                    <i className={cn("fa text-xs text-gray-400", item.icon)}></i>
                                    <span className="text-white">
                                        {item.type === "all" ? "all" : item.name}
                                    </span>
                                    {item.type === "all" && (
                                        <span className="text-gray-400 text-xs">All Workers</span>
                                    )}
                                    {item.type === "worker" && (
                                        <span className="text-gray-400 text-xs">{item.status}</span>
                                    )}
                                    {item.type === "worker" && (
                                        <span
                                            className={cn(
                                                "w-1.5 h-1.5 rounded-full",
                                                item.status === "working" ? "bg-green-500" : "bg-gray-500"
                                            )}
                                        ></span>
                                    )}
                                    {"detail" in item && item.detail && (
                                        <span className="text-gray-400 text-xs truncate max-w-48">{item.detail}</span>
                                    )}
                                </div>
                            ))}
                        </div>
                    )}
                    <Tooltip content="Attach files" placement="top" divClassName="absolute bottom-6.5 right-1">
                        <button
                            type="button"
                            onClick={handleUploadClick}
                            className={cn(
                                "w-5 h-5 transition-colors flex items-center justify-center text-gray-400 hover:text-accent cursor-pointer"
                            )}
                        >
                            <i className="fa fa-paperclip text-sm"></i>
                        </button>
                    </Tooltip>
                    {status === "streaming" ? (
                        <Tooltip content="Stop Response" placement="top" divClassName="absolute bottom-1.5 right-1">
                            <button
                                type="button"
                                onClick={() => model.stopResponse()}
                                className={cn(
                                    "w-5 h-5 transition-colors flex items-center justify-center",
                                    "text-green-500 hover:text-green-400 cursor-pointer"
                                )}
                            >
                                <i className="fa fa-square text-sm"></i>
                            </button>
                        </Tooltip>
                    ) : (
                        <Tooltip content="Send message (Enter)" placement="top" divClassName="absolute bottom-1.5 right-1">
                            <button
                                type="submit"
                                disabled={status !== "ready" || !input.trim()}
                                className={cn(
                                    "w-5 h-5 transition-colors flex items-center justify-center",
                                    status !== "ready" || !input.trim()
                                        ? "text-gray-400"
                                        : "text-accent/80 hover:text-accent cursor-pointer"
                                )}
                            >
                                <i className="fa fa-paper-plane text-sm"></i>
                            </button>
                        </Tooltip>
                    )}
                </div>
            </form>
        </div>
    );
});

AIPanelInput.displayName = "AIPanelInput";
