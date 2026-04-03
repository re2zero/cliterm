// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

export interface ParsedThinking {
    thinking: string;
    content: string;
}

const THINKING_REGEX = /<antThinking>[\s\S]*?<\/antThinking>/g;
const THINKING_TAG_OPEN = /<antThinking>/;
const THINKING_TAG_CLOSE = /<\/antThinking>/;

export function parseThinking(content: string): ParsedThinking {
    const matches = content.match(THINKING_REGEX);
    if (!matches || matches.length === 0) {
        return { thinking: "", content };
    }

    const thinking = matches
        .map((m) => m.replace(THINKING_TAG_OPEN, "").replace(THINKING_TAG_CLOSE, "").trim())
        .join("\n\n")
        .trim();

    const cleaned = content.replace(THINKING_REGEX, "").trim();
    return { thinking, content: cleaned };
}

export function hasThinking(content: string): boolean {
    return THINKING_REGEX.test(content);
}
