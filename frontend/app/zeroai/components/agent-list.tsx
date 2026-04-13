// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { makeIconClass } from "@/util/util";
import clsx from "clsx";
import * as React from "react";
import type { AgentDefinition, AgentMcpTool, AgentSkill } from "../models/agent-model";
import { addAgent, defaultRoles, removeAgent, updateAgent } from "../models/agent-model";
import "./agent-list.scss";

export interface AgentListProps {
    agents: AgentDefinition[];
    activeAgentId?: string | null;
    onSelectAgent: (agentId: string) => void;
    collapsed?: boolean;
    onToggleCollapse?: () => void;
}

const DEFAULT_COLORS = ["#d97706", "#6366f1", "#2563eb", "#10b981", "#ec4899", "#8b5cf6", "#f59e0b", "#ef4444"];

const AgentItem = React.memo(
    ({
        agent,
        isActive,
        onSelect,
        onEdit,
    }: {
        agent: AgentDefinition;
        isActive: boolean;
        onSelect: () => void;
        onEdit?: () => void;
    }) => {
        return (
            <div
                className={clsx("agent-list-item", { active: isActive })}
                onClick={onSelect}
            >
                {isActive && <div className="agent-item-indicator" />}
                <div className="agent-item-main">
                    <div className="agent-item-avatar" style={{ color: agent.color }}>
                        <i className={makeIconClass(agent.icon, false)} />
                    </div>
                    <div className="agent-item-details">
                        <div className="agent-item-name">
                            <span
                                className="agent-role-badge"
                                style={{ backgroundColor: agent.color + "33", color: agent.color }}
                            >
                                {agent.role}
                            </span>
                            {agent.name}
                        </div>
                    </div>
                    {onEdit && (
                        <div className="agent-item-actions">
                            <button
                                className="agent-item-edit"
                                onClick={(e) => {
                                    e.stopPropagation();
                                    onEdit();
                                }}
                                aria-label="Edit agent"
                            >
                                <i className="fa-solid fa-pen" />
                            </button>
                        </div>
                    )}
                </div>
            </div>
        );
    }
);
AgentItem.displayName = "AgentItem";

const CreateAgentModal = React.memo(({ onClose }: { onClose: () => void }) => {
    const [name, setName] = React.useState("");
    const [role, setRole] = React.useState("");
    const [description, setDescription] = React.useState("");
    const [soul, setSoul] = React.useState("");
    const [backend, setBackend] = React.useState("claude");
    const [model, setModel] = React.useState("");
    const [templateIdx, setTemplateIdx] = React.useState(-1);

    const handleTemplateSelect = (idx: number) => {
        const t = defaultRoles[idx];
        setTemplateIdx(idx);
        setName(t.name);
        setRole(t.role);
        setDescription(t.description);
        setSoul(t.soul);
        setBackend(t.backend);
        setModel(t.model);
    };

    const handleCreate = () => {
        if (!name.trim()) return;

        const backendIcons: Record<string, string> = {
            claude: "fa-solid fa-brain",
            opencode: "fa-solid fa-code-branch",
            qwen: "fa-solid fa-sparkles",
            codex: "fa-solid fa-code",
            custom: "fa-solid fa-robot",
        };

        addAgent({
            name: name.trim(),
            role: role.trim() || "通用助手",
            description: description.trim() || `${name.trim()} agent`,
            icon: backendIcons[backend] || "fa-solid fa-robot",
            color:
                "#" +
                Math.floor(Math.random() * 16777215)
                    .toString(16)
                    .padStart(6, "0"),
            backend,
            model: model.trim() || "default",
            provider: backend,
            soul: soul.trim() || `You are a ${role.trim() || "helpful"} assistant.`,
            agentMd: `# ${name.trim()}\n\n## Role\n${role.trim() || "General assistant"}`,
            skills: [],
            mcpTools: [],
        });
        onClose();
    };

    return (
        <div className="agent-create-overlay" onClick={onClose}>
            <div className="agent-create-modal" onClick={(e) => e.stopPropagation()}>
                <div className="agent-create-header">
                    <span>New Agent Role</span>
                    <button className="agent-create-close" onClick={onClose}>
                        <i className="fa-solid fa-xmark" />
                    </button>
                </div>
                <div className="agent-create-body">
                    <div className="agent-create-templates">
                        <span className="agent-create-label">Quick Templates</span>
                        <div className="agent-template-grid">
                            {defaultRoles.map((t, i) => (
                                <button
                                    key={i}
                                    className={clsx("agent-template-card", { active: templateIdx === i })}
                                    onClick={() => handleTemplateSelect(i)}
                                >
                                    <i className={makeIconClass(t.icon, false)} style={{ color: t.color }} />
                                    <span>{t.role}</span>
                                </button>
                            ))}
                        </div>
                    </div>
                    <div className="agent-create-row">
                        <label className="agent-create-label">
                            Name
                            <input
                                type="text"
                                value={name}
                                onChange={(e) => setName(e.target.value)}
                                placeholder="e.g. Code Reviewer"
                                autoFocus
                            />
                        </label>
                        <label className="agent-create-label">
                            Role
                            <input
                                type="text"
                                value={role}
                                onChange={(e) => setRole(e.target.value)}
                                placeholder="e.g. 架构师"
                            />
                        </label>
                    </div>
                    <label className="agent-create-label">
                        Description
                        <input
                            type="text"
                            value={description}
                            onChange={(e) => setDescription(e.target.value)}
                            placeholder="What does this agent do?"
                        />
                    </label>
                    <label className="agent-create-label">
                        Soul (System Prompt)
                        <textarea
                            value={soul}
                            onChange={(e) => setSoul(e.target.value)}
                            placeholder="Define the agent's personality and expertise..."
                            rows={3}
                        />
                    </label>
                    <div className="agent-create-row">
                        <label className="agent-create-label">
                            Backend
                            <select value={backend} onChange={(e) => setBackend(e.target.value)}>
                                <option value="claude">Claude</option>
                                <option value="opencode">OpenCode</option>
                                <option value="qwen">Qwen</option>
                                <option value="codex">Codex</option>
                                <option value="custom">Custom</option>
                            </select>
                        </label>
                        <label className="agent-create-label">
                            Model
                            <input
                                type="text"
                                value={model}
                                onChange={(e) => setModel(e.target.value)}
                                placeholder="default"
                            />
                        </label>
                    </div>
                </div>
                <div className="agent-create-footer">
                    <button className="agent-create-cancel" onClick={onClose}>
                        Cancel
                    </button>
                    <button
                        className={clsx("agent-create-confirm", { disabled: !name.trim() })}
                        onClick={handleCreate}
                        disabled={!name.trim()}
                    >
                        Create
                    </button>
                </div>
            </div>
        </div>
    );
});
CreateAgentModal.displayName = "CreateAgentModal";

export const AgentList = React.memo(
    ({ agents, activeAgentId, onSelectAgent, collapsed = false, onToggleCollapse }: AgentListProps) => {
        const [showCreate, setShowCreate] = React.useState(false);
        const [editingAgent, setEditingAgent] = React.useState<AgentDefinition | null>(null);

        return (
            <div className={clsx("agent-list", { collapsed })}>
                <div className="agent-list-header">
                    <button
                        className="agent-list-toggle"
                        onClick={onToggleCollapse}
                        title={collapsed ? "Expand agents" : "Collapse agents"}
                        aria-label="Toggle agent list"
                    >
                        <i className={clsx("fa-solid", collapsed ? "fa-chevron-right" : "fa-chevron-left")} />
                    </button>
                    {!collapsed && (
                        <>
                            <div className="agent-list-title">
                                <span>Agents</span>
                                <span className="agent-count">{agents.length}</span>
                            </div>
                            <div className="agent-list-actions">
                                <button
                                    className="agent-list-create"
                                    onClick={() => setShowCreate(true)}
                                    title="Create new agent"
                                >
                                    <i className="fa-solid fa-plus" />
                                </button>
                            </div>
                        </>
                    )}
                    {collapsed && (
                        <button
                            className="agent-list-create-collapsed"
                            onClick={() => setShowCreate(true)}
                            title="Create new agent"
                        >
                            <i className="fa-solid fa-plus" />
                        </button>
                    )}
                </div>

                {!collapsed && (
                    <div className="agent-list-content">
                        <div className="agent-list-grid">
                            {agents.map((agent) => (
                                <AgentItem
                                    key={agent.id}
                                    agent={agent}
                                    isActive={agent.id === activeAgentId}
                                    onSelect={() => onSelectAgent(agent.id)}
                                    onEdit={() => setEditingAgent(agent)}
                                />
                            ))}
                        </div>
                    </div>
                )}

                {showCreate && <CreateAgentModal onClose={() => setShowCreate(false)} />}
                {editingAgent && <EditAgentModal agent={editingAgent} onClose={() => setEditingAgent(null)} />}
            </div>
        );
    }
);
AgentList.displayName = "AgentList";

const EditAgentModal = React.memo(({ agent, onClose }: { agent: AgentDefinition; onClose: () => void }) => {
    const [name, setName] = React.useState(agent.name);
    const [role, setRole] = React.useState(agent.role);
    const [description, setDescription] = React.useState(agent.description);
    const [soul, setSoul] = React.useState(agent.soul);
    const [backend, setBackend] = React.useState(agent.backend);
    const [model, setModel] = React.useState(agent.model);
    const [skills, setSkills] = React.useState<AgentSkill[]>(agent.skills);
    const [mcpTools, setMcpTools] = React.useState<AgentMcpTool[]>(agent.mcpTools);
    const [newSkillName, setNewSkillName] = React.useState("");
    const [newSkillDesc, setNewSkillDesc] = React.useState("");
    const [newMcpName, setNewMcpName] = React.useState("");
    const [newMcpUrl, setNewMcpUrl] = React.useState("");

    const addSkill = () => {
        if (!newSkillName.trim()) return;
        setSkills([
            ...skills,
            { id: "skill-" + Date.now(), name: newSkillName.trim(), description: newSkillDesc.trim(), enabled: true },
        ]);
        setNewSkillName("");
        setNewSkillDesc("");
    };

    const removeSkill = (id: string) => setSkills(skills.filter((s) => s.id !== id));

    const addMcpTool = () => {
        if (!newMcpName.trim()) return;
        setMcpTools([
            ...mcpTools,
            { id: "mcp-" + Date.now(), name: newMcpName.trim(), url: newMcpUrl.trim(), enabled: true },
        ]);
        setNewMcpName("");
        setNewMcpUrl("");
    };

    const removeMcpTool = (id: string) => setMcpTools(mcpTools.filter((t) => t.id !== id));

    const handleSave = () => {
        if (!name.trim()) return;
        updateAgent(agent.id, {
            name: name.trim(),
            role: role.trim(),
            description: description.trim(),
            soul: soul.trim(),
            backend,
            model: model.trim(),
            skills,
            mcpTools,
        });
        onClose();
    };

    return (
        <div className="agent-create-overlay" onClick={onClose}>
            <div className="agent-create-modal agent-edit-modal" onClick={(e) => e.stopPropagation()}>
                <div className="agent-create-header">
                    <span>Edit: {agent.name}</span>
                    <button className="agent-create-close" onClick={onClose}>
                        <i className="fa-solid fa-xmark" />
                    </button>
                </div>
                <div className="agent-create-body">
                    {/* Basic Info */}
                    <div className="agent-create-row">
                        <label className="agent-create-label">
                            Name
                            <input type="text" value={name} onChange={(e) => setName(e.target.value)} />
                        </label>
                        <label className="agent-create-label">
                            Role
                            <input type="text" value={role} onChange={(e) => setRole(e.target.value)} />
                        </label>
                    </div>
                    <label className="agent-create-label">
                        Description
                        <input type="text" value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Brief agent description" />
                    </label>

                    {/* Soul - Larger textarea for editing */}
                    <label className="agent-create-label">
                        Soul (System Prompt)
                        <textarea value={soul} onChange={(e) => setSoul(e.target.value)} rows={8} placeholder="Agent's system prompt and guidelines..." />
                    </label>

                    {/* Backend Configuration */}
                    <div className="agent-create-row">
                        <label className="agent-create-label">
                            Backend
                            <select value={backend} onChange={(e) => setBackend(e.target.value)}>
                                <option value="claude">Claude</option>
                                <option value="opencode">OpenCode</option>
                                <option value="qwen">Qwen</option>
                                <option value="codex">Codex</option>
                                <option value="custom">Custom</option>
                            </select>
                        </label>
                        <label className="agent-create-label">
                            Model
                            <input type="text" value={model} onChange={(e) => setModel(e.target.value)} placeholder="e.g. claude-3-5-sonnet-20241022" />
                        </label>
                    </div>

                    {/* Skills Section */}
                    <div className="agent-edit-section">
                        <div className="agent-edit-section-header">
                            <span>Skills ({skills.length})</span>
                            <span className="text-xs text-gray-400">Click + to add</span>
                        </div>
                        <div className="agent-edit-skills-list">
                            {skills.map((skill) => (
                                <div key={skill.id} className="agent-edit-item">
                                    <span className="agent-edit-item-name">{skill.name}</span>
                                    <span className="agent-edit-item-desc">{skill.description || ""}</span>
                                    <button className="agent-edit-item-remove" onClick={() => removeSkill(skill.id)} title="Remove skill">
                                        <i className="fa-solid fa-xmark" />
                                    </button>
                                </div>
                            ))}
                        </div>
                        <div className="agent-edit-add-row">
                            <input
                                type="text"
                                placeholder="Skill name"
                                value={newSkillName}
                                onChange={(e) => setNewSkillName(e.target.value)}
                                onKeyDown={(e) => e.key === "Enter" && addSkill()}
                            />
                            <input
                                type="text"
                                placeholder="Description (optional)"
                                value={newSkillDesc}
                                onChange={(e) => setNewSkillDesc(e.target.value)}
                                onKeyDown={(e) => e.key === "Enter" && addSkill()}
                            />
                            <button onClick={addSkill} disabled={!newSkillName.trim()} title="Add skill">
                                <i className="fa-solid fa-plus" />
                            </button>
                        </div>
                    </div>

                    {/* MCP Tools Section */}
                    <div className="agent-edit-section">
                        <div className="agent-edit-section-header">
                            <span>MCP Tools ({mcpTools.length})</span>
                            <span className="text-xs text-gray-400">External tools and APIs</span>
                        </div>
                        <div className="agent-edit-mcp-list">
                            {mcpTools.map((tool) => (
                                <div key={tool.id} className="agent-edit-item">
                                    <span className="agent-edit-item-name">{tool.name}</span>
                                    <span className="agent-edit-item-desc">{tool.url || ""}</span>
                                    <button className="agent-edit-item-remove" onClick={() => removeMcpTool(tool.id)} title="Remove MCP tool">
                                        <i className="fa-solid fa-xmark" />
                                    </button>
                                </div>
                            ))}
                        </div>
                        <div className="agent-edit-add-row">
                            <input
                                type="text"
                                placeholder="MCP name"
                                value={newMcpName}
                                onChange={(e) => setNewMcpName(e.target.value)}
                                onKeyDown={(e) => e.key === "Enter" && addMcpTool()}
                            />
                            <input
                                type="text"
                                placeholder="Server URL (optional)"
                                value={newMcpUrl}
                                onChange={(e) => setNewMcpUrl(e.target.value)}
                                onKeyDown={(e) => e.key === "Enter" && addMcpTool()}
                            />
                            <button onClick={addMcpTool} disabled={!newMcpName.trim()} title="Add MCP tool">
                                <i className="fa-solid fa-plus" />
                            </button>
                        </div>
                    </div>
                </div>
                <div className="agent-create-footer">
                    <button className="agent-create-cancel" onClick={onClose}>
                        Cancel
                    </button>
                    <button
                        className={clsx("agent-create-confirm", { disabled: !name.trim() })}
                        onClick={handleSave}
                        disabled={!name.trim()}
                    >
                        Save
                    </button>
                </div>
            </div>
        </div>
    );
});
EditAgentModal.displayName = "EditAgentModal";
