// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { ZeroAIPanel } from "@/app/zeroai/index";
import { atom } from "jotai";

export class ZeroAiModel implements ViewModel {
    viewType = "zeroai";
    viewIcon = atom("smart_toy");
    viewName = atom("ZeroAI");
    noPadding = atom(true);
    viewComponent = ZeroAIPanel;

    constructor(_: ViewModelInitType) {}
}
