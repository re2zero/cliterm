// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { cn, makeIconClass } from "@/util/util";
import { useAtomValue } from "jotai";
import * as React from "react";
import {
    activeModelAtom,
    activeProviderAtom,
    activeProviderIdAtom,
    fetchProviders,
    providersAtom,
    setActiveProvider,
} from "../models/provider-model";
import { showProviderSettings } from "../models/ui-model";
import "./provider-selector.scss";

export const ProviderSelector = React.memo(() => {
    const providers = useAtomValue(providersAtom);
    const activeId = useAtomValue(activeProviderIdAtom);
    const activeModel = useAtomValue(activeModelAtom);
    const activeProvider = useAtomValue(activeProviderAtom);
    const [open, setOpen] = React.useState(false);
    const ref = React.useRef<HTMLDivElement>(null);

    React.useEffect(() => {
        fetchProviders();
    }, []);

    React.useEffect(() => {
        if (!open) return;
        const handler = (e: MouseEvent) => {
            if (ref.current && !ref.current.contains(e.target as Node)) {
                setOpen(false);
            }
        };
        document.addEventListener("mousedown", handler);
        return () => document.removeEventListener("mousedown", handler);
    }, [open]);

    const builtIns = React.useMemo(() => providers.filter((p) => !p.isCustom), [providers]);
    const customs = React.useMemo(() => providers.filter((p) => p.isCustom), [providers]);

    const handleSelect = React.useCallback(
        (id: string) => {
            const p = providers.find((x) => x.id === id);
            if (p?.isAvailable) {
                setActiveProvider(id);
            }
            setOpen(false);
        },
        [providers]
    );

    const displayName = activeProvider?.displayName ?? "Select Provider";
    const displayModel = activeModel || activeProvider?.defaultModel || "";

    return (
        <div className={cn("provider-selector", { open })} ref={ref}>
            <button className="provider-selector-trigger" onClick={() => setOpen((v) => !v)} title="Change provider">
                <i className={makeIconClass(activeProvider?.displayIcon || "fa-solid fa-robot", false)} />
                <span className="provider-selector-name">{displayName}</span>
                {displayModel && <span className="provider-sep">·</span>}
                {displayModel && <span className="provider-selector-model">{displayModel}</span>}
                <i className={cn(makeIconClass("fa-solid fa-chevron-down", false), "provider-chevron")} />
            </button>

            {open && (
                <div className="provider-dropdown">
                    {builtIns.length > 0 && (
                        <div className="pd-group">
                            <div className="pd-group-label">CLI Agents</div>
                            {builtIns.map((p) => (
                                <button
                                    key={p.id}
                                    className={cn("pd-item", { active: p.id === activeId, disabled: !p.isAvailable })}
                                    onClick={() => handleSelect(p.id)}
                                    disabled={!p.isAvailable}
                                >
                                    <i className={makeIconClass(p.displayIcon || "fa-solid fa-terminal", false)} />
                                    <span className="pd-item-name">{p.displayName}</span>
                                    {p.defaultModel && <span className="pd-item-model">{p.defaultModel}</span>}
                                    {p.id === activeId && (
                                        <i className={cn(makeIconClass("fa-solid fa-check", false), "pd-check")} />
                                    )}
                                    {!p.isAvailable && <span className="pd-item-unavailable">Not installed</span>}
                                </button>
                            ))}
                        </div>
                    )}

                    {customs.length > 0 && (
                        <div className="pd-group">
                            <div className="pd-group-label">LLM Providers</div>
                            {customs.map((p) => (
                                <button
                                    key={p.id}
                                    className={cn("pd-item", { active: p.id === activeId, disabled: !p.isAvailable })}
                                    onClick={() => handleSelect(p.id)}
                                    disabled={!p.isAvailable}
                                >
                                    <i className={makeIconClass(p.displayIcon || "fa-solid fa-robot", false)} />
                                    <span className="pd-item-name">{p.displayName}</span>
                                    {p.defaultModel && <span className="pd-item-model">{p.defaultModel}</span>}
                                    {p.id === activeId && (
                                        <i className={cn(makeIconClass("fa-solid fa-check", false), "pd-check")} />
                                    )}
                                    {!p.isAvailable && <span className="pd-item-unavailable">Not available</span>}
                                </button>
                            ))}
                        </div>
                    )}

                    <div className="pd-divider" />
                    <button className="pd-item pd-settings" onClick={() => showProviderSettings()}>
                        <i className={makeIconClass("fa-solid fa-gear", false)} />
                        <span>Provider Settings...</span>
                    </button>
                </div>
            )}
        </div>
    );
});
ProviderSelector.displayName = "ProviderSelector";
