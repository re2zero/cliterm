// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { makeIconClass } from "@/util/util";
import clsx from "clsx";
import { atom, useAtom, useAtomValue } from "jotai";
import * as React from "react";
import {
    activeProviderIdAtom,
    deleteProvider,
    fetchProviders,
    providerLoadingAtom,
    providersAtom,
    saveProvider,
    setActiveProvider,
    testProvider,
} from "../models/provider-model";
import type { SaveProviderRequest, ZeroAiProviderInfo } from "../types";
import "./provider-settings.scss";

export interface ProviderSettingsProps {
    className?: string;
}

type SettingsGroup = "providers" | "mcp" | "skills";
const settingsGroupAtom = atom<SettingsGroup>("providers");

export const ProviderSettings = React.memo(({ className }: ProviderSettingsProps) => {
    const [group, setGroup] = useAtom(settingsGroupAtom);
    const providers = useAtomValue(providersAtom);
    const loading = useAtomValue(providerLoadingAtom);

    React.useEffect(() => {
        fetchProviders();
    }, []);

    return (
        <div className={clsx("provider-settings", className)}>
            <div className="ps-sidebar">
                {(["providers", "mcp", "skills"] as SettingsGroup[]).map((g) => (
                    <button
                        key={g}
                        className={clsx("ps-sidebar-btn", { active: group === g })}
                        onClick={() => setGroup(g)}
                    >
                        <i
                            className={makeIconClass(
                                g === "providers"
                                    ? "fa-solid fa-plug"
                                    : g === "mcp"
                                      ? "fa-solid fa-diagram-project"
                                      : "fa-solid fa-wand-magic-sparkles",
                                false
                            )}
                        />
                        <span>{g === "providers" ? "Providers" : g === "mcp" ? "MCP" : "Skills"}</span>
                    </button>
                ))}
            </div>
            <div className="ps-content">
                {group === "providers" && <ProvidersGroup providers={providers} loading={loading} />}
                {group === "mcp" && <McpGroup />}
                {group === "skills" && <SkillsGroup />}
            </div>
        </div>
    );
});
ProviderSettings.displayName = "ProviderSettings";

const ProvidersGroup = React.memo(({ providers, loading }: { providers: ZeroAiProviderInfo[]; loading: boolean }) => {
    const builtIns = React.useMemo(() => providers.filter((p) => !p.isCustom), [providers]);
    const customs = React.useMemo(() => providers.filter((p) => p.isCustom), [providers]);

    return (
        <div className="ps-providers-stack">
            <div className="ps-providers-pane">
                <div className="ps-providers-pane-title">
                    <i className={makeIconClass("fa-solid fa-terminal", false)} />
                    <span>CLI Agents</span>
                </div>
                <CliList providers={builtIns} loading={loading} />
            </div>
            <div className="ps-providers-pane">
                <div className="ps-providers-pane-title">
                    <i className={makeIconClass("fa-solid fa-cloud", false)} />
                    <span>LLM API</span>
                </div>
                <LlmList providers={customs} loading={loading} />
            </div>
        </div>
    );
});
ProvidersGroup.displayName = "ProvidersGroup";

const CliList = React.memo(({ providers, loading }: { providers: ZeroAiProviderInfo[]; loading: boolean }) => {
    const activeId = useAtomValue(activeProviderIdAtom);
    const [connecting, setConnecting] = React.useState<string | null>(null);
    const [statuses, setStatuses] = React.useState<Record<string, "ok" | "error" | "connecting">>({});

    const handleConnect = React.useCallback(async (provider: ZeroAiProviderInfo) => {
        setActiveProvider(provider.id);
        setConnecting(provider.id);
        setStatuses((prev) => ({ ...prev, [provider.id]: "connecting" }));

        try {
            const result = await testProvider(provider.id);
            setStatuses((prev) => ({
                ...prev,
                [provider.id]: result.success ? "ok" : "error",
            }));
        } catch {
            setStatuses((prev) => ({ ...prev, [provider.id]: "error" }));
        } finally {
            setConnecting(null);
        }
    }, []);

    if (loading) {
        return (
            <div className="ps-loading">
                <i className={makeIconClass("fa-solid fa-spinner fa-spin", false)} /> Scanning...
            </div>
        );
    }

    return (
        <div className="ps-cli-list">
            {providers.map((p) => {
                const isActive = p.id === activeId;
                const isInstalled = p.isAvailable;
                const status = statuses[p.id] || (isInstalled ? "ok" : undefined);

                return (
                    <div
                        key={p.id}
                        className={clsx("ps-cli-item", {
                            active: isActive,
                            installed: isInstalled,
                        })}
                    >
                        <div className="ps-cli-item-icon">
                            <i className={makeIconClass(p.displayIcon || "fa-solid fa-terminal", false)} />
                        </div>
                        <div className="ps-cli-item-info">
                            <div className="ps-cli-item-header">
                                <span className="ps-cli-item-name">{p.displayName}</span>
                                <div className="ps-cli-item-status">
                                    {isActive && status === "connecting" && (
                                        <span className="ps-status-dot connecting" title="Connecting...">
                                            <i className="fa-solid fa-circle-notch fa-spin" />
                                        </span>
                                    )}
                                    {isActive && status === "ok" && (
                                        <span className="ps-status-dot ok" title="Connected" />
                                    )}
                                    {isActive && status === "error" && (
                                        <span className="ps-status-dot error" title="Connection failed" />
                                    )}
                                    {isActive && !status && isInstalled && (
                                        <span className="ps-status-dot ok" title="Available" />
                                    )}
                                </div>
                            </div>
                            <code className="ps-cli-item-cmd">{p.cliCommand}</code>
                            {!isInstalled && p.installHint && (
                                <div className="ps-cli-item-hint">
                                    <i className="fa-solid fa-download" />
                                    <code>{p.installHint}</code>
                                </div>
                            )}
                        </div>
                        {isInstalled && (
                            <button
                                className={clsx("ps-cli-connect", { active: isActive })}
                                onClick={() => handleConnect(p)}
                                disabled={connecting === p.id}
                            >
                                {isActive && status === "connecting" ? (
                                    <i className="fa-solid fa-circle-notch fa-spin" />
                                ) : isActive ? (
                                    "Connected"
                                ) : (
                                    "Connect"
                                )}
                            </button>
                        )}
                    </div>
                );
            })}
        </div>
    );
});
CliList.displayName = "CliList";

interface LlmFormState {
    providerId: string;
    displayName: string;
    baseUrl: string;
    apiKey: string;
    model: string;
}

const emptyLlmForm: LlmFormState = {
    providerId: "",
    displayName: "",
    baseUrl: "",
    apiKey: "",
    model: "",
};

const LlmList = React.memo(({ providers, loading }: { providers: ZeroAiProviderInfo[]; loading: boolean }) => {
    const [editing, setEditing] = React.useState<string | null>(null);
    const [form, setForm] = React.useState<LlmFormState>(emptyLlmForm);
    const [formError, setFormError] = React.useState("");
    const [testResult, setTestResult] = React.useState<{ success: boolean; msg: string } | null>(null);
    const isTesting = useAtomValue(providerLoadingAtom);

    const handleAdd = React.useCallback(() => {
        setEditing("new");
        setForm(emptyLlmForm);
        setFormError("");
        setTestResult(null);
    }, []);

    const handleEdit = React.useCallback((p: ZeroAiProviderInfo) => {
        setEditing(p.id);
        setFormError("");
        setTestResult(null);
        setForm({
            providerId: p.id,
            displayName: p.displayName,
            baseUrl: (p.envVars?.["API_BASE_URL"] as string) || "",
            apiKey: (p.envVars?.["API_KEY"] as string) || "",
            model: p.defaultModel || "",
        });
    }, []);

    const handleSave = React.useCallback(async () => {
        if (!form.displayName.trim()) {
            setFormError("Name is required");
            return;
        }
        if (!form.baseUrl.trim()) {
            setFormError("Base URL is required");
            return;
        }
        if (!form.model.trim()) {
            setFormError("Model is required");
            return;
        }

        const request: SaveProviderRequest = {
            providerId:
                editing === "new"
                    ? form.displayName
                          .toLowerCase()
                          .replace(/\s+/g, "-")
                          .replace(/[^a-z0-9-]/g, "")
                    : form.providerId,
            displayName: form.displayName,
            cliCommand: "llm-api",
            envVars: {
                API_BASE_URL: form.baseUrl,
                API_KEY: form.apiKey,
            },
            defaultModel: form.model,
            supportsStreaming: true,
        };

        try {
            await saveProvider(request);
            setEditing(null);
            setForm(emptyLlmForm);
        } catch (err) {
            setFormError(`Failed to save: ${err}`);
        }
    }, [form, editing]);

    const handleTest = React.useCallback(async () => {
        if (!form.baseUrl.trim()) {
            setFormError("Base URL is required");
            return;
        }
        setFormError("");
        try {
            const id = editing === "new" ? "temp" : form.providerId;
            const result = await testProvider(id);
            setTestResult({
                success: result.success,
                msg: result.success ? `Connected (${result.latencyMs}ms)` : result.error || "Connection failed",
            });
        } catch (err) {
            setTestResult({ success: false, msg: String(err) });
        }
    }, [form, editing]);

    const handleDelete = React.useCallback(
        async (id: string) => {
            try {
                await deleteProvider(id);
                if (editing === id) {
                    setEditing(null);
                    setForm(emptyLlmForm);
                }
            } catch (err) {
                setFormError(`Failed to delete: ${err}`);
            }
        },
        [editing]
    );

    const handleCancel = React.useCallback(() => {
        setEditing(null);
        setForm(emptyLlmForm);
        setFormError("");
        setTestResult(null);
    }, []);

    return (
        <div className="ps-llm-list">
            {formError && <div className="ps-form-error">{formError}</div>}
            {testResult && (
                <div className={clsx("ps-test-result", { success: testResult.success })}>
                    <i
                        className={makeIconClass(
                            testResult.success ? "fa-solid fa-circle-check" : "fa-solid fa-circle-xmark",
                            false
                        )}
                    />
                    <span>{testResult.msg}</span>
                </div>
            )}

            {editing && (
                <div className="ps-llm-form">
                    <div className="ps-llm-form-title">{editing === "new" ? "Add LLM Provider" : "Edit Provider"}</div>
                    <label className="ps-llm-form-field">
                        <span>Name</span>
                        <input
                            type="text"
                            value={form.displayName}
                            onChange={(e) => setForm((f) => ({ ...f, displayName: e.target.value }))}
                            placeholder="e.g., OpenAI"
                            disabled={editing !== "new"}
                        />
                    </label>
                    <label className="ps-llm-form-field">
                        <span>Base URL</span>
                        <input
                            type="text"
                            value={form.baseUrl}
                            onChange={(e) => setForm((f) => ({ ...f, baseUrl: e.target.value }))}
                            placeholder="https://api.openai.com/v1"
                        />
                    </label>
                    <label className="ps-llm-form-field">
                        <span>API Key</span>
                        <input
                            type="password"
                            value={form.apiKey}
                            onChange={(e) => setForm((f) => ({ ...f, apiKey: e.target.value }))}
                            placeholder="sk-..."
                        />
                    </label>
                    <label className="ps-llm-form-field">
                        <span>Model</span>
                        <input
                            type="text"
                            value={form.model}
                            onChange={(e) => setForm((f) => ({ ...f, model: e.target.value }))}
                            placeholder="gpt-4o"
                        />
                    </label>
                    <div className="ps-llm-form-actions">
                        <button className="ps-llm-btn test" onClick={handleTest} disabled={isTesting}>
                            {isTesting ? <i className="fa-solid fa-circle-notch fa-spin" /> : "Test"}
                        </button>
                        <button className="ps-llm-btn cancel" onClick={handleCancel}>
                            Cancel
                        </button>
                        <button className="ps-llm-btn save" onClick={handleSave}>
                            {editing === "new" ? "Add" : "Save"}
                        </button>
                    </div>
                </div>
            )}

            {!editing && (
                <>
                    {loading && <div className="ps-loading">Loading...</div>}
                    {!loading && providers.length === 0 && (
                        <div className="ps-empty">
                            <i className={makeIconClass("fa-solid fa-cloud", false)} />
                            <span>No LLM providers</span>
                            <span className="ps-empty-hint">Add OpenAI, Ollama, or any compatible API</span>
                        </div>
                    )}
                    {providers.map((p) => (
                        <div key={p.id} className="ps-llm-item">
                            <div className="ps-llm-item-info">
                                <span className="ps-llm-item-name">{p.displayName}</span>
                                <span className="ps-llm-item-model">{p.defaultModel || "—"}</span>
                            </div>
                            <div className="ps-llm-item-actions">
                                <button onClick={() => handleEdit(p)} title="Edit">
                                    <i className={makeIconClass("fa-solid fa-pen", false)} />
                                </button>
                                <button onClick={() => handleDelete(p.id)} title="Delete">
                                    <i className={makeIconClass("fa-solid fa-trash", false)} />
                                </button>
                            </div>
                        </div>
                    ))}
                    {providers.length > 0 && (
                        <button className="ps-llm-add" onClick={handleAdd}>
                            <i className={makeIconClass("fa-solid fa-plus", false)} />
                            <span>Add Provider</span>
                        </button>
                    )}
                    {providers.length === 0 && (
                        <button className="ps-llm-add" onClick={handleAdd}>
                            <i className={makeIconClass("fa-solid fa-plus", false)} />
                            <span>Add LLM Provider</span>
                        </button>
                    )}
                </>
            )}
        </div>
    );
});
LlmList.displayName = "LlmList";

const McpGroup = React.memo(() => (
    <div className="ps-placeholder-group">
        <i className={makeIconClass("fa-solid fa-diagram-project", false)} />
        <span>MCP Servers</span>
        <span className="ps-placeholder-hint">Configure Model Context Protocol servers</span>
    </div>
));
McpGroup.displayName = "McpGroup";

const SkillsGroup = React.memo(() => (
    <div className="ps-placeholder-group">
        <i className={makeIconClass("fa-solid fa-wand-magic-sparkles", false)} />
        <span>Skills</span>
        <span className="ps-placeholder-hint">Manage AI skills and custom instructions</span>
    </div>
));
SkillsGroup.displayName = "SkillsGroup";
