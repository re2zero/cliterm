// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { Button } from "@/app/element/button";
import { Input } from "@/app/element/input";
import { Toggle } from "@/app/element/toggle";
import { WaveStreamdown } from "@/app/element/streamdown";
import type { AgentManagementViewModel } from "./agent-management-model";
import type { Agent, MCPConnection } from "@/types/gotypes";
import { fetchSkills, skillsAtom, skillsLoadingAtom } from "@/app/zeroai/models/skills-model";
import { fetchMCPServers, mcpServersAtom, mcpServersLoadingAtom } from "@/app/zeroai/models/mcp-model";
import clsx from "clsx";
import * as jotai from "jotai";
import * as React from "react";
import dayjs from "dayjs";

// Form state
interface AgentFormData {
    name: string;
    role: string;
    soul: string;
    skills: string[];
    mcpConnections: MCPConnection[];
    enabled: boolean;
}

interface FormErrors {
    name?: string;
    role?: string;
}

// Skills and MCP servers will be fetched from backend via skills-model and mcp-model

// AgentCreateDialog component
const AgentCreateDialog = React.memo(({ model, onClose }: { model: AgentManagementViewModel; onClose: () => void }) => {
    const [formData, setFormData] = React.useState<AgentFormData>({
        name: "",
        role: "",
        soul: "",
        skills: [],
        mcpConnections: [],
        enabled: true,
    });
    const [errors, setErrors] = React.useState<FormErrors>({});
    const [loading, setLoading] = React.useState(false);

    // Load skills and MCP servers on mount
    React.useEffect(() => {
        fetchSkills();
        fetchMCPServers();
    }, []);

    // Get skills and MCP servers from global store
    const skills = jotai.useAtomValue(skillsAtom);
    const skillsLoading = jotai.useAtomValue(skillsLoadingAtom);
    const mcpServers = jotai.useAtomValue(mcpServersAtom);
    const mcpServersLoading = jotai.useAtomValue(mcpServersLoadingAtom);

    const validateForm = (): boolean => {
        const newErrors: FormErrors = {};

        if (!formData.name.trim()) {
            newErrors.name = "Name is required";
        }

        if (!formData.role.trim()) {
            newErrors.role = "Role is required";
        }

        setErrors(newErrors);
        return Object.keys(newErrors).length === 0;
    };

    const handleSubmit = async () => {
        if (!validateForm()) {
            return;
        }

        setLoading(true);
        try {
            const agentData: Omit<Agent, "id" | "createdAt" | "updatedAt"> = {
                name: formData.name.trim(),
                role: formData.role.trim(),
                soul: formData.soul.trim() || undefined,
                skills: formData.skills,
                mcpConnections: formData.mcpConnections.map((serverName) => ({
                    serverName,
                    config: {},
                })),
                enabled: formData.enabled,
            };

            await model.createAgent(agentData);
            onClose();
        } catch (error) {
            console.error("Failed to create agent:", error);
            alert("Failed to create agent. Please try again.");
        } finally {
            setLoading(false);
        }
    };

    const toggleSkill = (skill: string) => {
        setFormData((prev) => ({
            ...prev,
            skills: prev.skills.includes(skill)
                ? prev.skills.filter((s) => s !== skill)
                : [...prev.skills, skill],
        }));
    };

    const toggleMCPServer = (serverName: string) => {
        setFormData((prev) => ({
            ...prev,
            mcpConnections: prev.mcpConnections.some((mcp) => mcp.serverName === serverName)
                ? prev.mcpConnections.filter((mcp) => mcp.serverName !== serverName)
                : [...prev.mcpConnections, { serverName, config: {} }],
        }));
    };

    return (
        <div className="agent-create-overlay" onClick={onClose}>
            <div className="agent-create-modal" onClick={(e) => e.stopPropagation()}>
                {/* Header */}
                <div className="agent-create-header">
                    <span>Create Agent</span>
                    <button className="agent-create-close" onClick={onClose} disabled={loading}>
                        <i className="fa-solid fa-xmark" />
                    </button>
                </div>

                {/* Body */}
                <div className="agent-create-body">
                    {/* Name */}
                    <label className="agent-create-label">
                        <span className={clsx({ "text-error-color": errors.name })}>
                            Name {errors.name && `(${errors.name})`}
                        </span>
                        <Input
                            value={formData.name}
                            onChange={setFormData}
                            placeholder="e.g. Code Reviewer"
                            autoFocus
                            disabled={loading}
                        />
                    </label>

                    {/* Role */}
                    <label className="agent-create-label">
                        <span className={clsx({ "text-error-color": errors.role })}>
                            Role {errors.role && `(${errors.role})`}
                        </span>
                        <Input value={formData.role} onChange={setFormData} placeholder="e.g. Architect" disabled={loading} />
                    </label>

                    {/* Soul with markdown preview */}
                    <div className="agent-create-soul-section">
                        <label className="agent-create-label">
                            <span>Soul (System Prompt)</span>
                        </label>
                        <textarea
                            className="agent-soul-textarea"
                            value={formData.soul}
                            onChange={(e) => setFormData((prev) => ({ ...prev, soul: e.target.value }))}
                            placeholder="Define the agent's personality and expertise..."
                            rows={4}
                            disabled={loading}
                        />
                        <div className="agent-soul-preview">
                            <span className="agent-soul-preview-label">Preview:</span>
                            <WaveStreamdown
                                text={formData.soul || "*Agent soul will appear here...*"}
                                className="text-sm"
                            />
                        </div>
                    </div>

                    {/* Skills multi-select */}
                    <div className="agent-create-section">
                        <span className="agent-create-section-label">Skills ({formData.skills.length})</span>
                        <div className="agent-tags-grid">
                            {subskills.map((skill) => (
                                <button
                                    key={skill}
                                    className={clsx("agent-tag-button", {
                                        selected: formData.skills.includes(skill),
                                    })}
                                    onClick={() => toggleSkill(skill)}
                                    disabled={loading}
                                >
                                    {skill}
                                </button>
                            ))}
                        </div>
                    </div>

                    {/* MCP servers multi-select */}
                    <div className="agent-create-section">
                        <span className="agent-create-section-label">
                            MCP Connections ({formData.mcpConnections.length})
                        </span>
                        <div className="agent-tags-grid">
                            {submcpServers.map((server) => (
                                <button
                                    key={server}
                                    className={clsx("agent-tag-button", {
                                        selected: formData.mcpConnections.some((mcp) => mcp.serverName === server),
                                    })}
                                    onClick={() => toggleMCPServer(server)}
                                    disabled={loading}
                                >
                                    {server}
                                </button>
                            ))}
                        </div>
                    </div>

                    {/* Enabled toggle */}
                    <div className="agent-create-row">
                        <Toggle
                            checked={formData.enabled}
                            onChange={(value) => setFormData((prev) => ({ ...prev, enabled: value }))}
                            label="Enabled"
                            id="agent-create-enabled"
                        />
                    </div>
                </div>

                {/* Footer */}
                <div className="agent-create-footer">
                    <button className="agent-create-cancel" onClick={onClose} disabled={loading}>
                        Cancel
                    </button>
                    <button
                        className={clsx("agent-create-confirm", { disabled: loading })}
                        onClick={handleSubmit}
                        disabled={loading || !formData.name.trim() || !formData.role.trim()}
                    >
                        {loading ? (
                            <>
                                <i className="fa-solid fa-circle-notch fa-spin" />
                                <span>Creating...</span>
                            </>
                        ) : (
                            "Create"
                        )}
                    </button>
                </div>
            </div>
        </div>
    );
});
AgentCreateDialog.displayName = "AgentCreateDialog";

// AgentEditDialog component
const AgentEditDialog = React.memo(({ model, onClose }: { model: AgentManagementViewModel; onClose: () => void }) => {
    const dialogState = jotai.useAtomValue(model.dialogStateAtom);
    const agentToEdit = dialogState?.mode === "edit" ? dialogState.agent : null;

    const [formData, setFormData] = React.useState<AgentFormData>({
        name: "",
        role: "",
        soul: "",
        skills: [],
        mcpConnections: [],
        enabled: true,
    });
    const [errors, setErrors] = React.useState<FormErrors>({});
    const [loading, setLoading] = React.useState(false);
    const [agentLoaded, setAgentLoaded] = React.useState(false);

    // Load agent data on mount
    React.useEffect(() => {
        if (agentToEdit && !agentLoaded) {
            setFormData({
                name: agentToEdit.name,
                role: agentToEdit.role,
                soul: agentToEdit.soul || "",
                skills: agentToEdit.skills || [],
                mcpConnections: agentToEdit.mcpConnections || [],
                enabled: agentToEdit.enabled,
            });
            setAgentLoaded(true);
        }
    }, [agentToEdit, agentLoaded]);

    if (!agentToEdit) {
        return null;
    }

    const validateForm = (): boolean => {
        const newErrors: FormErrors = {};

        if (!formData.name.trim()) {
            newErrors.name = "Name is required";
        }

        if (!formData.role.trim()) {
            newErrors.role = "Role is required";
        }

        setErrors(newErrors);
        return Object.keys(newErrors).length === 0;
    };

    const handleSubmit = async () => {
        if (!validateForm()) {
            return;
        }

        setLoading(true);
        try {
            const updates: Partial<Agent> = {
                name: formData.name.trim(),
                role: formData.role.trim(),
                soul: formData.soul.trim() || undefined,
                skills: formData.skills,
                mcpConnections: formData.mcpConnections.map((mcp) => ({
                    serverName: mcp.serverName,
                    config: mcp.config || {},
                })),
                enabled: formData.enabled,
            };

            await model.updateAgent(agentToEdit.id, updates);
            onClose();
        } catch (error) {
            console.error("Failed to update agent:", error);
            alert("Failed to update agent. Please try again.");
        } finally {
            setLoading(false);
        }
    };

    const toggleSkill = (skill: string) => {
        setFormData((prev) => ({
            ...prev,
            skills: prev.skills.includes(skill)
                ? prev.skills.filter((s) => s !== skill)
                : [...prev.skills, skill],
        }));
    };

    const toggleMCPServer = (serverName: string) => {
        setFormData((prev) => ({
            ...prev,
            mcpConnections: prev.mcpConnections.some((mcp) => mcp.serverName === serverName)
                ? prev.mcpConnections.filter((mcp) => mcp.serverName !== serverName)
                : [...prev.mcpConnections, { serverName, config: {} }],
        }));
    };

    return (
        <div className="agent-create-overlay" onClick={onClose}>
            <div className="agent-create-modal agent-edit-modal" onClick={(e) => e.stopPropagation()}>
                {/* Header */}
                <div className="agent-create-header">
                    <span>Edit Agent</span>
                    <button className="agent-create-close" onClick={onClose} disabled={loading}>
                        <i className="fa-solid fa-xmark" />
                    </button>
                </div>

                {/* Body */}
                <div className="agent-create-body">
                    {/* Name */}
                    <label className="agent-create-label">
                        <span className={clsx({ "text-error-color": errors.name })}>
                            Name {errors.name && `(${errors.name})`}
                        </span>
                        <Input value={formData.name} onChange={setFormData} disabled={loading} />
                    </label>

                    {/* Role */}
                    <label className="agent-create-label">
                        <span className={clsx({ "text-error-color": errors.role })}>
                            Role {errors.role && `(${errors.role})`}
                        </span>
                        <Input value={formData.role} onChange={setFormData} disabled={loading} />
                    </label>

                    {/* Soul with markdown preview */}
                    <div className="agent-create-soul-section">
                        <label className="agent-create-label">
                            <span>Soul (System Prompt)</span>
                        </label>
                        <textarea
                            className="agent-soul-textarea"
                            value={formData.soul}
                            onChange={(e) => setFormData((prev) => ({ ...prev, soul: e.target.value }))}
                            rows={4}
                            disabled={loading}
                        />
                        <div className="agent-soul-preview">
                            <span className="agent-soul-preview-label">Preview:</span>
                            <WaveStreamdown
                                text={formData.soul || "*Agent soul will appear here...*"}
                                className="text-sm"
                            />
                        </div>
                    </div>

                    {/* Skills multi-select */}
                    <div className="agent-create-section">
                        <span className="agent-create-section-label">Skills ({formData.skills.length})</span>
                        <div className="agent-tags-grid">
                            {subskills.map((skill) => (
                                <button
                                    key={skill}
                                    className={clsx("agent-tag-button", {
                                        selected: formData.skills.includes(skill),
                                    })}
                                    onClick={() => toggleSkill(skill)}
                                    disabled={loading}
                                >
                                    {skill}
                                </button>
                            ))}
                        </div>
                    </div>

                    {/* MCP servers multi-select */}
                    <div className="agent-create-section">
                        <span className="agent-create-section-label">
                            MCP Connections ({formData.mcpConnections.length})
                        </span>
                        <div className="agent-tags-grid">
                            {submcpServers.map((server) => (
                                <button
                                    key={server}
                                    className={clsx("agent-tag-button", {
                                        selected: formData.mcpConnections.some((mcp) => mcp.serverName === server),
                                    })}
                                    onClick={() => toggleMCPServer(server)}
                                    disabled={loading}
                                }
                                >
                                    {server}
                                </button>
                            ))}
                        </div>
                    </div>

                    {/* Enabled toggle */}
                    <div className="agent-create-row">
                        <Toggle
                            checked={formData.enabled}
                            onChange={(value) => setFormData((prev) => ({ ...prev, enabled: value }))}
                            label="Enabled"
                            id="agent-edit-enabled"
                        />
                    </div>

                    {/* Metadata */}
                    <div className="agent-edit-metadata">
                        <span className="agent-edit-metadata-label">
                            Created: {dayjs(agentToEdit.createdAt / 1000).format("MMM D, YYYY HH:mm")}
                        </span>
                        <span className="agent-edit-metadata-label">
                            Updated: {dayjs(agentToEdit.updatedAt / 1000).format("MMM D, YYYY HH:mm")}
                        </span>
                    </div>
                </div>

                {/* Footer */}
                <div className="agent-create-footer">
                    <button className="agent-create-cancel" onClick={onClose} disabled={loading}>
                        Cancel
                    </button>
                    <button
                        className={clsx("agent-create-confirm", { disabled: loading })}
                        onClick={handleSubmit}
                        disabled={loading || !formData.name.trim() || !formData.role.trim()}
                    >
                        {loading ? (
                            <>
                                <i className="fa-solid fa-circle-notch fa-spin" />
                                <span>Saving...</span>
                            </>
                        ) : (
                            "Save"
                        )}
                    </button>
                </div>
            </div>
        </div>
    );
});
AgentEditDialog.displayName = "AgentEditDialog";

// AgentDialog wrapper component
export const AgentDialog = React.memo(({ model, onClose }: { model: AgentManagementViewModel; onClose: () => void }) => {
    const dialogState = jotai.useAtomValue(model.dialogStateAtom);

    if (!dialogState) {
        return null;
    }

    if (dialogState.mode === "create") {
        return <AgentCreateDialog model={model} onClose={onClose} />;
    }

    if (dialogState.mode === "edit") {
        return <AgentEditDialog model={model} onClose={onClose} />;
    }

    return null;
});
AgentDialog.displayName = "AgentDialog";
