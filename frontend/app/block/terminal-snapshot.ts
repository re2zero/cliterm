// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

type ScrollbackProvider = () => string;

const providers = new Map<string, (lineCount: number) => string>();

export function registerTerminalSnapshot(blockId: string, provider: (lineCount: number) => string): void {
    providers.set(blockId, provider);
}

export function unregisterTerminalSnapshot(blockId: string): void {
    providers.delete(blockId);
}

export function getTerminalLastLines(blockId: string, lineCount: number = 5): string | null {
    const provider = providers.get(blockId);
    if (!provider) return null;
    try {
        const result = provider(lineCount);
        return result || null;
    } catch {
        return null;
    }
}
