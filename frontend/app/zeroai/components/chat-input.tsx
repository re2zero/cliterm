// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { globalStore } from "@/app/store/jotaiStore";
import { cn, makeIconClass } from "@/util/util";
import { useAtom, useAtomValue } from "jotai";
import * as React from "react";
import { useCallback, useEffect, useRef, useState } from "react";
import {
    activeModelAtom,
    activeProviderAtom,
    activeProviderIdAtom,
    fetchProviders,
    providersAtom,
    setActiveProvider,
} from "../models/provider-model";
import { inputHeightAtom, showProviderSettings } from "../models/ui-model";
import "./chat-input.scss";

export interface ChatInputProps {
    value: string;
    onChange: (value: string) => void;
    onSend: () => void;
    isSending?: boolean;
    disabled?: boolean;
    placeholder?: string;
}

const ChatInput = React.memo(
    ({
        value,
        onChange,
        onSend,
        isSending = false,
        disabled = false,
        placeholder = "Ask ZeroAI...",
    }: ChatInputProps) => {
        const [inputHeight, setInputHeight] = useAtom(inputHeightAtom);
        const textareaRef = useRef<HTMLTextAreaElement>(null);
        const [isResizing, setIsResizing] = useState(false);
        const [modelOpen, setModelOpen] = useState(false);
        const modelRef = useRef<HTMLDivElement>(null);

        const providers = useAtomValue(providersAtom);
        const activeId = useAtomValue(activeProviderIdAtom);
        const activeModel = useAtomValue(activeModelAtom);
        const activeProvider = useAtomValue(activeProviderAtom);

        useEffect(() => {
            fetchProviders();
        }, []);

        useEffect(() => {
            if (!modelOpen) return;
            const handler = (e: MouseEvent) => {
                if (modelRef.current && !modelRef.current.contains(e.target as Node)) {
                    setModelOpen(false);
                }
            };
            document.addEventListener("mousedown", handler);
            return () => document.removeEventListener("mousedown", handler);
        }, [modelOpen]);

        const autoResizeHeight = useCallback(() => {
            if (textareaRef.current) {
                textareaRef.current.style.height = "auto";
                const h = Math.max(40, Math.min(260, textareaRef.current.scrollHeight));
                if (!isResizing) {
                    setInputHeight(h);
                    textareaRef.current.style.height = `${h}px`;
                }
            }
        }, [isResizing, setInputHeight]);

        useEffect(() => {
            if (textareaRef.current) {
                textareaRef.current.style.height = "auto";
                autoResizeHeight();
                const saved = globalStore.get(inputHeightAtom);
                if (saved !== "auto" && typeof saved === "number" && saved > 40) {
                    textareaRef.current.style.height = `${saved}px`;
                }
            }
        }, [value, autoResizeHeight]);

        const handleKeyDown = useCallback(
            (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
                if (e.key === "Enter" && !e.shiftKey) {
                    e.preventDefault();
                    if (value.trim() && !isSending && !disabled) onSend();
                }
            },
            [value, isSending, disabled, onSend]
        );

        const handleResizeStart = useCallback(
            (e: React.MouseEvent) => {
                e.preventDefault();
                setIsResizing(true);
                const startY = e.clientY;
                const startH = textareaRef.current?.offsetHeight ?? 40;
                const move = (ev: MouseEvent) => {
                    const h = Math.max(40, Math.min(260, startH + ev.clientY - startY));
                    setInputHeight(h);
                    if (textareaRef.current) textareaRef.current.style.height = `${h}px`;
                };
                const up = () => {
                    setIsResizing(false);
                    document.removeEventListener("mousemove", move);
                    document.removeEventListener("mouseup", up);
                };
                document.addEventListener("mousemove", move);
                document.addEventListener("mouseup", up);
            },
            [setInputHeight]
        );

        const handleSelectModel = useCallback(
            (id: string) => {
                const p = providers.find((x) => x.id === id);
                if (p?.isAvailable) setActiveProvider(id);
                setModelOpen(false);
            },
            [providers]
        );

        const displayName = activeProvider?.displayName ?? "Model";
        const displayModel = activeModel || activeProvider?.defaultModel || "";

        return (
            <div className={cn("chat-input", { "is-sending": isSending, isDisabled: disabled })}>
                <textarea
                    ref={textareaRef}
                    value={value}
                    onChange={(e) => onChange(e.target.value)}
                    onKeyDown={handleKeyDown}
                    placeholder={placeholder}
                    disabled={disabled}
                    className="chat-input-textarea"
                    rows={1}
                />

                <div className="chat-input-footer">
                    <div className="chat-input-left">
                        <div className="chat-input-model" ref={modelRef}>
                            <button
                                className="chat-input-model-btn"
                                onClick={() => setModelOpen((v) => !v)}
                                title="Change model"
                            >
                                <i
                                    className={makeIconClass(activeProvider?.displayIcon || "fa-solid fa-robot", false)}
                                />
                                <span>{displayName}</span>
                                {displayModel && <span className="chat-input-model-sep">·</span>}
                                {displayModel && <span className="chat-input-model-name">{displayModel}</span>}
                                <i
                                    className={cn(
                                        makeIconClass("fa-solid fa-chevron-down", false),
                                        "chat-input-model-chevron",
                                        { expanded: modelOpen }
                                    )}
                                />
                            </button>

                            {modelOpen && (
                                <div className="chat-input-model-dropdown">
                                    {providers.filter((p) => !p.isCustom).length > 0 && (
                                        <div className="ci-dropdown-group">
                                            <div className="ci-dropdown-label">CLI Agents</div>
                                            {providers
                                                .filter((p) => !p.isCustom)
                                                .map((p) => (
                                                    <button
                                                        key={p.id}
                                                        className={cn("ci-dropdown-item", {
                                                            active: p.id === activeId,
                                                            disabled: !p.isAvailable,
                                                        })}
                                                        onClick={() => handleSelectModel(p.id)}
                                                        disabled={!p.isAvailable}
                                                    >
                                                        <i
                                                            className={makeIconClass(
                                                                p.displayIcon || "fa-solid fa-terminal",
                                                                false
                                                            )}
                                                        />
                                                        <span>{p.displayName}</span>
                                                        {p.defaultModel && (
                                                            <span className="ci-dropdown-item-model">
                                                                {p.defaultModel}
                                                            </span>
                                                        )}
                                                        {p.id === activeId && (
                                                            <i
                                                                className={cn(
                                                                    makeIconClass("fa-solid fa-check", false),
                                                                    "ci-dropdown-check"
                                                                )}
                                                            />
                                                        )}
                                                    </button>
                                                ))}
                                        </div>
                                    )}
                                    {providers.filter((p) => p.isCustom).length > 0 && (
                                        <div className="ci-dropdown-group">
                                            <div className="ci-dropdown-label">LLM Providers</div>
                                            {providers
                                                .filter((p) => p.isCustom)
                                                .map((p) => (
                                                    <button
                                                        key={p.id}
                                                        className={cn("ci-dropdown-item", {
                                                            active: p.id === activeId,
                                                            disabled: !p.isAvailable,
                                                        })}
                                                        onClick={() => handleSelectModel(p.id)}
                                                        disabled={!p.isAvailable}
                                                    >
                                                        <i
                                                            className={makeIconClass(
                                                                p.displayIcon || "fa-solid fa-robot",
                                                                false
                                                            )}
                                                        />
                                                        <span>{p.displayName}</span>
                                                        {p.defaultModel && (
                                                            <span className="ci-dropdown-item-model">
                                                                {p.defaultModel}
                                                            </span>
                                                        )}
                                                        {p.id === activeId && (
                                                            <i
                                                                className={cn(
                                                                    makeIconClass("fa-solid fa-check", false),
                                                                    "ci-dropdown-check"
                                                                )}
                                                            />
                                                        )}
                                                    </button>
                                                ))}
                                        </div>
                                    )}
                                    <div className="ci-dropdown-divider" />
                                    <button
                                        className="ci-dropdown-item ci-dropdown-settings"
                                        onClick={() => showProviderSettings()}
                                    >
                                        <i className={makeIconClass("fa-solid fa-gear", false)} />
                                        <span>Settings</span>
                                    </button>
                                </div>
                            )}
                        </div>
                    </div>

                    <div className="chat-input-right">
                        <span className="chat-input-hint">Shift+Enter ↵</span>
                        <button
                            type="button"
                            onClick={onSend}
                            disabled={!value.trim() || isSending || disabled}
                            className={cn("chat-input-send", { "is-disabled": !value.trim() || isSending || disabled })}
                        >
                            {isSending ? (
                                <i className="fa-solid fa-circle-notch fa-spin" />
                            ) : (
                                <i className="fa-solid fa-arrow-up" />
                            )}
                        </button>
                    </div>
                </div>

                <div className="chat-input-resize" onMouseDown={handleResizeStart} title="Drag to resize">
                    <i className="fa-solid fa-angles-up" />
                </div>
            </div>
        );
    }
);

ChatInput.displayName = "ChatInput";

export default ChatInput;
