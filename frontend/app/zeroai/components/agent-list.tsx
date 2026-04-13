// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { makeIconClass } from "@/util/util";
import clsx from "clsx";
import * as React from "react";
import * as jotai from "jotai";
import type { AgentDefinition, AgentMcpTool, AgentSkill } from "../models/agent-model";
import { addAgent, defaultRoles, removeAgent, updateAgent } from "../models/agent-model";
import { fetchSkills, skillsAtom, skillsLoadingAtom } from "../models/skills-model";
import { fetchMCPServers, mcpServersAtom, mcpServersLoadingAtom } from "../models/mcp-model";
// SkillInfo and MCPServerInfo are global types from frontend/types/gotypes.d.ts (declare global)
import "./agent-list.scss";

export interface AgentListProps {
    agents: AgentDefinition[];
    activeAgentId?: string | null;
    onSelectAgent: (agentId: string) => void;
    collapsed?: boolean;
    onToggleCollapse?: () => void;
}

const DEFAULT_COLORS = ["#d97706", "#6366f1", "#2563eb", "#10b981", "#ec4899", "#8b5cf6", "#f59e0b", "#ef4444"];

const CollapsedAgentItem = React.memo(
    ({
        agent,
        isActive,
        onSelectAgent,
        onEditAgent,
        onRunAgent,
    }: {
        agent: AgentDefinition;
        isActive: boolean;
        onSelectAgent: (id: string) => void;
        onEditAgent: (agent: AgentDefinition) => void;
        onRunAgent: (agent: AgentDefinition) => void;
    }) => {
        const [showMenu, setShowMenu] = React.useState(false);
        const menuRef = React.useRef<HTMLDivElement>(null);

        React.useEffect(() => {
            const handleClickOutside = (event: MouseEvent) => {
                if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
                    setShowMenu(false);
                }
            };

            if (showMenu) {
                document.addEventListener("mousedown", handleClickOutside);
                return () => document.removeEventListener("mousedown", handleClickOutside);
            }
        }, [showMenu]);

        const handleClick = () => {
            onSelectAgent(agent.id);
        };

        const handleDoubleClick = (e: React.MouseEvent) => {
            e.stopPropagation();
            onSelectAgent(agent.id);
            onRunAgent(agent);
        };

        const handleContextMenu = (e: React.MouseEvent) => {
            e.preventDefault();
            setShowMenu(true);
        };

        const handleRun = (e: React.MouseEvent) => {
            e.stopPropagation();
            onSelectAgent(agent.id);
            onRunAgent(agent);
            setShowMenu(false);
        };

        const handleEdit = (e: React.MouseEvent) => {
            e.stopPropagation();
            onSelectAgent(agent.id);
            onEditAgent(agent);
            setShowMenu(false);
        };

        return (
            <button
                className={clsx("agent-list-collapsed-icon", { active: isActive })}
                onClick={handleClick}
                onDoubleClick={handleDoubleClick}
                onContextMenu={handleContextMenu}
                title={agent.name}
                style={{ color: agent.color }}
            >
                <i className={makeIconClass(agent.icon, false)} />
                {showMenu && (
                    <div className="agent-list-collapsed-menu" ref={menuRef}>
                        <button className="agent-list-collapsed-menu-item" onClick={handleRun}>
                            <i className="fa-solid fa-play" />
                            <span>Run</span>
                        </button>
                        <button className="agent-list-collapsed-menu-item" onClick={handleEdit}>
                            <i className="fa-solid fa-pen" />
                            <span>Edit</span>
                        </button>
                    </div>
                )}
            </button>
        );
    }
);
CollapsedAgentItem.displayName = "CollapsedAgentItem";

const AgentItem = React.memo(
    ({
        agent,
        isActive,
        onSelect,
        onEdit,
        onDelete,
        onRun,
    }: {
        agent: AgentDefinition;
        isActive: boolean;
        onSelect: () => void;
        onEdit?: () => void;
        onDelete?: () => void;
        onRun?: () => void;
    }) => {
        const [showMenu, setShowMenu] = React.useState(false);
        const menuRef = React.useRef<HTMLDivElement>(null);

        // Close menu when clicking outside
        React.useEffect(() => {
            const handleClickOutside = (event: MouseEvent) => {
                if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
                    setShowMenu(false);
                }
            };

            if (showMenu) {
                document.addEventListener("mousedown", handleClickOutside);
                return () => document.removeEventListener("mousedown", handleClickOutside);
            }
        }, [showMenu]);

        const handleMenuAction = (action: "run" | "edit" | "delete") => {
            setShowMenu(false);
            if (action === "run" && onRun) {
                onRun();
            } else if (action === "edit" && onEdit) {
                onEdit();
            } else if (action === "delete" && onDelete) {
                onDelete();
            }
        };

        const handleRun = (e: React.MouseEvent) => {
            e.stopPropagation();
            if (onRun) {
                onRun();
            }
        };

        return (
            <div
                className={clsx("agent-list-item", { active: isActive })}
                onClick={onSelect}
                onDoubleClick={handleRun}
            >
                {isActive && <div className="agent-item-indicator" />}
                <div className="agent-item-main">
                    <div className="agent-item-avatar" style={{ backgroundColor: agent.color + "22", color: agent.color }}>
                        <i className={makeIconClass(agent.icon, false)} />
                    </div>
                    <div className="agent-item-details">
                        <div className="agent-item-name">{agent.name}</div>
                        <div className="agent-item-role" style={{ color: agent.color }}>
                            {agent.role}
                        </div>
                    </div>
                    {(onEdit || onDelete || onRun) && (
                        <div className="agent-item-actions" ref={menuRef}>
                            {onRun && (
                                <button className="agent-item-run" onClick={handleRun} title="Run agent">
                                    <i className="fa-solid fa-play" />
                                </button>
                            )}
                            <button
                                className="agent-item-menu"
                                onClick={(e) => {
                                    e.stopPropagation();
                                    setShowMenu(!showMenu);
                                }}
                                aria-label="Agent options"
                                title="Options"
                            >
                                <i className="fa-solid fa-ellipsis-vertical" />
                            </button>
                            {showMenu && (
                                <div className="agent-item-menu-dropdown">
                                    {onRun && (
                                        <button
                                            className="agent-item-menu-item"
                                            onClick={() => handleMenuAction("run")}
                                        >
                                            <i className="fa-solid fa-play" />
                                            <span>Run</span>
                                        </button>
                                    )}
                                    {onEdit && (
                                        <button
                                            className="agent-item-menu-item"
                                            onClick={() => handleMenuAction("edit")}
                                        >
                                            <i className="fa-solid fa-pen" />
                                            <span>Edit</span>
                                        </button>
                                    )}
                                    {onDelete && (
                                        <button
                                            className="agent-item-menu-item danger"
                                            onClick={() => handleMenuAction("delete")}
                                        >
                                            <i className="fa-solid fa-trash" />
                                            <span>Delete</span>
                                        </button>
                                    )}
                                </div>
                            )}
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
    const nameInputRef = React.useRef<HTMLInputElement>(null);

    // Skills and MCP selection for new agent
    const [selectedSkills, setSelectedSkills] = React.useState<string[]>([]);
    const [selectedMCPs, setSelectedMCPs] = React.useState<string[]>([]);

    // Load skills and MCP servers from backend
    React.useEffect(() => {
        fetchSkills();
        fetchMCPServers();
    }, []);

    // Get skills and MCP servers from global store
    const skills = jotai.useAtomValue(skillsAtom);
    const skillsLoading = jotai.useAtomValue(skillsLoadingAtom);
    const mcpServers = jotai.useAtomValue(mcpServersAtom);
    const mcpServersLoading = jotai.useAtomValue(mcpServersLoadingAtom);

    const toggleSkill = (skillName: string) => {
        setSelectedSkills((prev) =>
            prev.includes(skillName) ? prev.filter((s) => s !== skillName) : [...prev, skillName]
        );
    };

    const toggleMCP = (serverName: string) => {
        setSelectedMCPs((prev) =>
            prev.includes(serverName) ? prev.filter((m) => m !== serverName) : [...prev, serverName]
        );
    };

    const handleTemplateSelect = (idx: number) => {
        const t = defaultRoles[idx];
        setTemplateIdx(idx);
        setName(t.name);
        setRole(t.role);
        setDescription(t.description);
        setSoul(t.soul);
        setBackend(t.backend);
        setModel(t.model);
        // Focus the name input after selecting a template
        setTimeout(() => {
            nameInputRef.current?.focus();
        }, 0);
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
            skills: selectedSkills.map((name, idx) => ({
                id: `skill-${idx}`,
                name,
                description: "",
                enabled: true,
            })),
            mcpTools: selectedMCPs.map((name, idx) => ({
                id: `mcp-${idx}`,
                name,
                url: "",
                enabled: true,
            })),
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
                                ref={nameInputRef}
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

                    {/* Skills Section - Multi-select from backend */}
                    <div className="agent-edit-section">
                        <div className="agent-edit-section-header">
                            <span>Skills ({selectedSkills.length}/{skills.length})</span>
                            <span className="text-xs text-gray-400">Click to select/deselect</span>
                        </div>
                        {skillsLoading ? (
                            <div className="text-xs text-gray-400 py-2">Loading skills...</div>
                        ) : skills.length === 0 ? (
                            <div className="text-xs text-gray-400 py-2">No skills configured. Add skills in Settings.</div>
                        ) : (
                            <div className="agent-tags-grid">
                                {skills.map((skill) => (
                                    <button
                                        key={skill.id}
                                        type="button"
                                        className={clsx("agent-tag-button", {
                                            "agent-tag-selected": selectedSkills.includes(skill.name),
                                        })}
                                        onClick={() => toggleSkill(skill.name)}
                                    >
                                        <span className="agent-tag-indicator">
                                            {selectedSkills.includes(skill.name) ? "✓" : ""}
                                        </span>
                                        <span className="agent-tag-text">{skill.name}</span>
                                    </button>
                                ))}
                            </div>
                        )}
                    </div>

                    {/* MCP Section - Multi-select from backend */}
                    <div className="agent-edit-section">
                        <div className="agent-edit-section-header">
                            <span>MCP ({selectedMCPs.length}/{mcpServers.length})</span>
                            <span className="text-xs text-gray-400">Click to select/deselect</span>
                        </div>
                        {mcpServersLoading ? (
                            <div className="text-xs text-gray-400 py-2">Loading MCP servers...</div>
                        ) : mcpServers.length === 0 ? (
                            <div className="text-xs text-gray-400 py-2">No MCP servers configured. Add servers in Settings.</div>
                        ) : (
                            <div className="agent-tags-grid">
                                {mcpServers.map((mcp) => (
                                    <button
                                        key={mcp.id}
                                        type="button"
                                        className={clsx("agent-tag-button", {
                                            "agent-tag-selected": selectedMCPs.includes(mcp.name),
                                        })}
                                        onClick={() => toggleMCP(mcp.name)}
                                    >
                                        <span className="agent-tag-indicator">
                                            {selectedMCPs.includes(mcp.name) ? "✓" : ""}
                                        </span>
                                        <span className="agent-tag-text">{mcp.name}</span>
                                    </button>
                                ))}
                            </div>
                        )}
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

        const handleDeleteAgent = (agent: AgentDefinition) => {
            removeAgent(agent.id);
            // If the deleted agent was active, switch to null
            if (activeAgentId === agent.id) {
                onSelectAgent(null);
            }
        };

        const handleRunAgent = (agent: AgentDefinition) => {
            console.log("[agent-list] run agent:", agent.name);
            // TODO: Launch terminal block and inject agent characteristics
            // This requires integration with block system and terminal creation
            alert(`Running agent: ${agent.name}\n\nThis feature will create a terminal block and inject agent characteristics. (To be implemented)`);
        };

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
                </div>

                {/* Collapsed: Show agent avatars + create button */}
                {collapsed && (
                    <div className="agent-list-collapsed-content">
                        <div className="agent-list-collapsed-icons">
                            {agents.map((agent) => (
                                <CollapsedAgentItem
                                    key={agent.id}
                                    agent={agent}
                                    isActive={agent.id === activeAgentId}
                                    onSelectAgent={onSelectAgent}
                                    onEditAgent={setEditingAgent}
                                    onRunAgent={handleRunAgent}
                                />
                            ))}
                        </div>
                        <button
                            className="agent-list-create-collapsed"
                            onClick={() => setShowCreate(true)}
                            title="Create new agent"
                        >
                            <i className="fa-solid fa-plus" />
                        </button>
                    </div>
                )}

                {/* Expanded: Show agent list */}
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
                                    onDelete={() => handleDeleteAgent(agent)}
                                    onRun={() => handleRunAgent(agent)}
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
    const [selectedSkills, setSelectedSkills] = React.useState<string[]>(agent.skills.map(s => s.name));
    const [selectedMCPs, setSelectedMCPs] = React.useState<string[]>(agent.mcpTools.map(t => t.name));

    // Load skills and MCP servers from backend
    React.useEffect(() => {
        fetchSkills();
        fetchMCPServers();
    }, []);

    // Get skills and MCP servers from global store
    const skills = jotai.useAtomValue(skillsAtom);
    const skillsLoading = jotai.useAtomValue(skillsLoadingAtom);
    const mcpServers = jotai.useAtomValue(mcpServersAtom);
    const mcpServersLoading = jotai.useAtomValue(mcpServersLoadingAtom);

    const toggleSkill = (skillName: string) => {
        setSelectedSkills((prev) =>
            prev.includes(skillName) ? prev.filter((s) => s !== skillName) : [...prev, skillName]
        );
    };

    const toggleMCP = (serverName: string) => {
        setSelectedMCPs((prev) =>
            prev.includes(serverName) ? prev.filter((m) => m !== serverName) : [...prev, serverName]
        );
    };

    const handleSave = () => {
        if (!name.trim()) return;
        updateAgent(agent.id, {
            name: name.trim(),
            role: role.trim(),
            description: description.trim(),
            soul: soul.trim(),
            backend,
            model: model.trim(),
            skills: selectedSkills.map((name, idx) => ({
                id: `skill-${idx}`,
                name,
                description: "",
                enabled: true,
            })),
            mcpTools: selectedMCPs.map((name, idx) => ({
                id: `mcp-${idx}`,
                name,
                url: "",
                enabled: true,
            })),
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

                    {/* Skills Section - Multi-select from backend */}
                    <div className="agent-edit-section">
                        <div className="agent-edit-section-header">
                            <span>Skills ({selectedSkills.length}/{skills.length})</span>
                            <span className="text-xs text-gray-400">Click to select/deselect</span>
                        </div>
                        {skillsLoading ? (
                            <div className="text-xs text-gray-400 py-2">Loading skills...</div>
                        ) : skills.length === 0 ? (
                            <div className="text-xs text-gray-400 py-2">No skills configured. Add skills in Settings.</div>
                        ) : (
                            <div className="agent-tags-grid">
                                {skills.map((skill) => (
                                    <button
                                        key={skill.id}
                                        type="button"
                                        className={clsx("agent-tag-button", {
                                            "agent-tag-selected": selectedSkills.includes(skill.name),
                                        })}
                                        onClick={() => toggleSkill(skill.name)}
                                    >
                                        <span className="agent-tag-indicator">
                                            {selectedSkills.includes(skill.name) ? "✓" : ""}
                                        </span>
                                        <span className="agent-tag-text">{skill.name}</span>
                                    </button>
                                ))}
                            </div>
                        )}
                    </div>

                    {/* MCP Section - Multi-select from backend */}
                    <div className="agent-edit-section">
                        <div className="agent-edit-section-header">
                            <span>MCP ({selectedMCPs.length}/{mcpServers.length})</span>
                            <span className="text-xs text-gray-400">Click to select/deselect</span>
                        </div>
                        {mcpServersLoading ? (
                            <div className="text-xs text-gray-400 py-2">Loading MCP servers...</div>
                        ) : mcpServers.length === 0 ? (
                            <div className="text-xs text-gray-400 py-2">No MCP servers configured. Add servers in Settings.</div>
                        ) : (
                            <div className="agent-tags-grid">
                                {mcpServers.map((mcp) => (
                                    <button
                                        key={mcp.id}
                                        type="button"
                                        className={clsx("agent-tag-button", {
                                            "agent-tag-selected": selectedMCPs.includes(mcp.name),
                                        })}
                                        onClick={() => toggleMCP(mcp.name)}
                                    >
                                        <span className="agent-tag-indicator">
                                            {selectedMCPs.includes(mcp.name) ? "✓" : ""}
                                        </span>
                                        <span className="agent-tag-text">{mcp.name}</span>
                                    </button>
                                ))}
                            </div>
                        )}
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
