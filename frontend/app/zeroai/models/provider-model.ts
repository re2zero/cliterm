// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { globalStore } from "@/app/store/jotaiStore";
import { atom, type PrimitiveAtom } from "jotai";
import { zeroAiClient } from "../store/zeroai-client";
import type { SaveProviderRequest, TestProviderResult, ZeroAiProviderInfo } from "../types";

export const providersAtom: PrimitiveAtom<ZeroAiProviderInfo[]> = atom<ZeroAiProviderInfo[]>([]);
export const providerLoadingAtom: PrimitiveAtom<boolean> = atom<boolean>(false);
export const providerTestingAtom: PrimitiveAtom<string | null> = atom<string | null>(null as string | null);

/**
 * Active provider selection atoms — controls which provider/model the chat uses
 */
export const activeProviderIdAtom = atom<string>("claude");
export const activeModelAtom = atom<string>("");

/**
 * Derived: get the currently active provider object
 */
export const activeProviderAtom = atom<ZeroAiProviderInfo | null>((get) => {
    const providers = get(providersAtom);
    const activeId = get(activeProviderIdAtom);
    return providers.find((p) => p.id === activeId) ?? null;
});

export type ProviderAction =
    | { type: "setProviders"; providers: ZeroAiProviderInfo[] }
    | { type: "addProvider"; provider: ZeroAiProviderInfo }
    | { type: "updateProvider"; providerId: string; updates: Partial<ZeroAiProviderInfo> }
    | { type: "removeProvider"; providerId: string }
    | { type: "setLoading"; loading: boolean }
    | { type: "setTesting"; providerId: string | null };

export const providerActionsAtom = atom(null, (_get, _set, action: ProviderAction) => {
    switch (action.type) {
        case "setProviders":
            globalStore.set(providersAtom, action.providers);
            break;
        case "addProvider":
            globalStore.set(providersAtom, (prev) => [...prev, action.provider]);
            break;
        case "updateProvider":
            globalStore.set(providersAtom, (prev) =>
                prev.map((p) => (p.id === action.providerId ? { ...p, ...action.updates } : p))
            );
            break;
        case "removeProvider":
            globalStore.set(providersAtom, (prev) => prev.filter((p) => p.id !== action.providerId));
            break;
        case "setLoading":
            globalStore.set(providerLoadingAtom, action.loading);
            break;
        case "setTesting":
            globalStore.set(providerTestingAtom, action.providerId);
            break;
    }
});

export function dispatchProviderAction(action: ProviderAction): void {
    globalStore.set(providerActionsAtom, action);
}

export async function fetchProviders(): Promise<ZeroAiProviderInfo[]> {
    dispatchProviderAction({ type: "setLoading", loading: true });
    try {
        const providers = await zeroAiClient.listProviders();
        dispatchProviderAction({ type: "setProviders", providers });

        // Auto-select first available provider if current selection is unavailable
        const currentId = globalStore.get(activeProviderIdAtom);
        const current = providers.find((p) => p.id === currentId);
        if (!current) {
            const firstAvailable = providers.find((p) => p.isAvailable);
            if (firstAvailable) {
                setActiveProvider(firstAvailable.id, firstAvailable.defaultModel);
            }
        }

        return providers;
    } finally {
        dispatchProviderAction({ type: "setLoading", loading: false });
    }
}

export async function saveProvider(request: SaveProviderRequest): Promise<void> {
    await zeroAiClient.saveProvider(request);
    await fetchProviders();
}

export async function deleteProvider(providerId: string): Promise<void> {
    await zeroAiClient.deleteProvider({ providerId });
    dispatchProviderAction({ type: "removeProvider", providerId });
}

export async function testProvider(providerId: string): Promise<TestProviderResult> {
    dispatchProviderAction({ type: "setTesting", providerId });
    try {
        return await zeroAiClient.testProvider(providerId);
    } finally {
        dispatchProviderAction({ type: "setTesting", providerId: null });
    }
}

export function setActiveProvider(providerId: string, model?: string): void {
    globalStore.set(activeProviderIdAtom, providerId);
    if (model) {
        globalStore.set(activeModelAtom, model);
    } else {
        const providers = globalStore.get(providersAtom);
        const provider = providers.find((p) => p.id === providerId);
        if (provider?.defaultModel) {
            globalStore.set(activeModelAtom, provider.defaultModel);
        }
    }
}

export function getActiveProviderId(): string {
    return globalStore.get(activeProviderIdAtom);
}

export function getActiveModel(): string {
    return globalStore.get(activeModelAtom);
}
