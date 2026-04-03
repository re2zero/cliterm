// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { globalStore } from "@/app/store/jotaiStore";
import { cn } from "@/util/util";
import { useAtom } from "jotai";
import * as React from "react";
import { useCallback, useEffect, useState } from "react";
import { inputHeightAtom } from "../models/ui-model";
import "./resizable-input.scss";

export interface ResizableInputProps {
    value: string;
    onChange: (value: string) => void;
    onSend: () => void;
    isSending?: boolean;
    disabled?: boolean;
    placeholder?: string;
    minHeight?: number;
    maxHeight?: number;
    className?: string;
}

const ResizableInput = React.memo(
    ({
        value,
        onChange,
        onSend,
        isSending = false,
        disabled = false,
        placeholder = "Type a message...",
        minHeight = 60,
        maxHeight = 400,
        className,
    }: ResizableInputProps) => {
        const [inputHeight, setInputHeight] = useAtom(inputHeightAtom);
        const textareaRef = React.useRef<HTMLTextAreaElement>(null);
        const [isResizing, setIsResizing] = useState(false);

        const autoResizeHeight = useCallback(() => {
            if (textareaRef.current) {
                const newHeight = Math.max(
                    minHeight,
                    Math.min(maxHeight, Math.max(minHeight, textareaRef.current.scrollHeight))
                );
                if (!isResizing) {
                    setInputHeight(newHeight);
                }
            }
        }, [minHeight, maxHeight, isResizing, setInputHeight]);

        useEffect(() => {
            if (textareaRef.current) {
                textareaRef.current.style.height = "auto";
                autoResizeHeight();
                const savedHeight = globalStore.get(inputHeightAtom);
                if (savedHeight !== "auto" && typeof savedHeight === "number") {
                    textareaRef.current.style.height = `${savedHeight}px`;
                }
            }
        }, [value, autoResizeHeight, inputHeightAtom]);

        const handleKeyDown = useCallback(
            (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
                if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) {
                    e.preventDefault();
                    if (value.trim() && !isSending && !disabled) {
                        onSend();
                    }
                }
            },
            [value, isSending, disabled, onSend]
        );

        const handleResizeStart = useCallback(
            (e: React.MouseEvent<HTMLDivElement>) => {
                e.preventDefault();
                setIsResizing(true);
                const startY = e.clientY;
                const startHeight = inputHeight === "auto" ? minHeight : inputHeight;

                const handleMouseMove = (moveEvent: MouseEvent) => {
                    const deltaY = moveEvent.clientY - startY;
                    let newHeight = Math.max(minHeight, Math.min(maxHeight, startHeight + deltaY));
                    setInputHeight(newHeight);
                    if (textareaRef.current) {
                        textareaRef.current.style.height = `${newHeight}px`;
                    }
                };

                const handleMouseUp = () => {
                    setIsResizing(false);
                    document.removeEventListener("mousemove", handleMouseMove);
                    document.removeEventListener("mouseup", handleMouseUp);
                };

                document.addEventListener("mousemove", handleMouseMove);
                document.addEventListener("mouseup", handleMouseUp);
            },
            [inputHeight, minHeight, maxHeight, setInputHeight]
        );

        const currentHeight = inputHeight === "auto" ? minHeight : inputHeight;

        return (
            <div
                className={cn("resizable-input", className, {
                    "is-resizing": isResizing,
                    "is-sending": isSending,
                    isDisabled: disabled,
                })}
                style={{ height: `${currentHeight}px` }}
            >
                <textarea
                    ref={textareaRef}
                    value={value}
                    onChange={(e) => onChange(e.target.value)}
                    onKeyDown={handleKeyDown}
                    placeholder={placeholder}
                    disabled={disabled}
                    className="resizable-input-textarea"
                    style={{ minHeight: `${minHeight}px`, maxHeight: `${maxHeight}px` }}
                    rows={1}
                />
                <div className="resizable-input-footer">
                    <span className="resizable-input-hint">
                        <i className="fa-solid fa-paperclip" />
                        Ctrl+Enter to send
                    </span>
                    <button
                        type="button"
                        onClick={onSend}
                        disabled={!value.trim() || isSending || disabled}
                        className={cn("resizable-input-send", {
                            "is-disabled": !value.trim() || isSending || disabled,
                        })}
                        aria-label="Send message"
                    >
                        {isSending ? (
                            <i className="fa-solid fa-circle-notch fa-spin" />
                        ) : (
                            <i className="fa-solid fa-arrow-up" />
                        )}
                    </button>
                </div>
                <div className="resizable-input-handle" onMouseDown={handleResizeStart} title="Drag to resize">
                    <i className="fa-solid fa-angles-up" />
                </div>
            </div>
        );
    }
);

ResizableInput.displayName = "ResizableInput";

export default ResizableInput;
