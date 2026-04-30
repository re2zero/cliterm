// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { formatFileSizeError, isAcceptableFile, validateFileSize } from "@/app/aipanel/ai-utils";
import { waveAIHasFocusWithin } from "@/app/aipanel/waveai-focus-utils";
import { type WaveAIModel } from "@/app/aipanel/waveai-model";
import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";
import { Tooltip } from "@/element/tooltip";
import { cn } from "@/util/util";
import { useAtom, useAtomValue } from "jotai";
import { memo, useCallback, useEffect, useRef, useState } from "react";

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
    const [mentionQuery, setMentionQuery] = useState("");
    const [mentionStartPos, setMentionStartPos] = useState(-1);
    const [mentionSelectedIdx, setMentionSelectedIdx] = useState(0);
    const [workersCache, setWorkersCache] = useState<CoworkWorker[] | null>(null);
    const workersFetchRef = useRef(false);

    let placeholder: string;
    if (!isChatEmpty) {
        placeholder = "Continue...";
    } else if (model.inBuilder) {
        placeholder = "What would you like to build...";
    } else {
        placeholder = "Ask Wave AI anything...";
    }

    const fetchWorkers = useCallback(async () => {
        if (workersFetchRef.current) return;
        workersFetchRef.current = true;
        try {
            const workers = await RpcApi.CoworkListWorkersCommand(TabRpcClient);
            setWorkersCache(workers);
        } catch {
            setWorkersCache([]);
        }
    }, []);

    const filteredWorkers = (() => {
        if (!workersCache) return [];
        const q = mentionQuery.toLowerCase();
        return workersCache.filter((w) => w.name.toLowerCase().includes(q));
    })();

    const mentionItems = [
        { type: "all" as const, name: "All Workers", icon: "fa-users", status: "" },
        ...filteredWorkers.map((w) => ({
            type: "worker" as const,
            name: w.name,
            icon: "fa-robot",
            status: w.status,
        })),
    ];

    const closeMention = useCallback(() => {
        setShowMention(false);
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
            const newValue = before + "@" + name + " " + after;
            setInput(newValue);
            closeMention();
            requestAnimationFrame(() => {
                const pos = mentionStartPos + name.length + 2;
                textarea.selectionStart = pos;
                textarea.selectionEnd = pos;
            });
        },
        [input, mentionStartPos, setInput, closeMention]
    );

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
                            const beforeCursor = val.slice(0, cursorPos);
                            const atIdx = beforeCursor.lastIndexOf("@");
                            if (
                                atIdx >= 0 &&
                                (atIdx === 0 || /\s/.test(beforeCursor[atIdx - 1]))
                            ) {
                                const queryText = beforeCursor.slice(atIdx + 1);
                                if (!/\s/.test(queryText)) {
                                    setShowMention(true);
                                    setMentionStartPos(atIdx);
                                    setMentionQuery(queryText);
                                    setMentionSelectedIdx(0);
                                    if (!workersFetchRef.current) {
                                        fetchWorkers();
                                    }
                                } else {
                                    closeMention();
                                }
                            } else {
                                closeMention();
                            }
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
                        <div className="absolute bottom-full left-0 mb-1 bg-zinc-900 border border-gray-600 rounded shadow-lg max-h-48 overflow-y-auto z-50 min-w-48">
                            {mentionItems.map((item, idx) => (
                                <div
                                    key={item.type === "all" ? "all" : item.name}
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
                                        @{item.type === "all" ? "all" : item.name}
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
