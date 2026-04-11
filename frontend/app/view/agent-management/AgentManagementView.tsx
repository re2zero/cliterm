// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { Button } from "@/app/element/button";
import * as jotai from "jotai";
import * as React from "react";
import type { AgentManagementViewModel, AgentViewState } from "./agent-management-model";
import { AgentDialog } from "./AgentDialog";
import dayjs from "dayjs";

type AgentManagementViewProps = {
    model: AgentManagementViewModel;
};

function AgentManagementView({ model }: AgentManagementViewProps) {
    const agents = jotai.useAtomValue(model.agentsAtom);
    const loading = jotai.useAtomValue(model.loadingAtom);
    const error = jotai.useAtomValue(model.errorAtom);
    const dialogState = jotai.useAtomValue(model.dialogStateAtom);
    const setDialogState = jotai.useSetAtom(model.dialogStateAtom);

    // Load agents on mount
    React.useEffect(() => {
        model.loadAgents();
    }, [model]);

    const handleCreateAgent = React.useCallback(() => {
        model.openCreateDialog();
    }, [model]);

    const handleEditAgent = React.useCallback(
        (agent: AgentViewState) => {
            // Convert AgentViewState back to Agent (remove display properties)
            const { displayColor, displayIcon, ...agentData } = agent;
            model.openEditDialog(agentData);
        },
        [model]
    );

    const handleDeleteAgent = React.useCallback(
        async (agentId: string) => {
            if (confirm("Are you sure you want to delete this agent?")) {
                try {
                    await model.deleteAgent(agentId);
                } catch (error) {
                    console.error("Failed to delete agent:", error);
                    alert("Failed to delete agent. Please try again.");
                }
            }
        },
        [model]
    );

    const handleRefresh = React.useCallback(async () => {
        await model.loadAgents();
    }, [model]);

    return (
        <div className="w-full h-full flex flex-col bg-main-bg overflow-hidden">
            {/* Header */}
            <div className="p-4 border-b border-border-color">
                <div className="flex items-center justify-between mb-2">
                    <h1 className="text-xl font-semibold text-main-text-color">Agent Management</h1>
                    <div className="flex items-center gap-2">
                        <Button className="outline grey text-sm" onClick={handleRefresh}>
                            <i className="fa-solid fa-rotate" />
                            <span>Refresh</span>
                        </Button>
                        <Button className="solid accent text-sm" onClick={handleCreateAgent}>
                            <i className="fa-solid fa-plus" />
                            <span>Create Agent</span>
                        </Button>
                    </div>
                </div>
                <p className="text-sm text-secondary-text-color">
                    Manage your AI agents with soul prompts, skills, and MCP configurations.
                </p>
            </div>

            {/* Content */}
            <div className="flex-grow overflow-y-auto p-4">
                {loading ? (
                    /* Loading state */
                    <div className="flex flex-col items-center justify-center h-full text-secondary-text-color">
                        <i className="fa-solid fa-circle-notch fa-spin text-2xl mb-2" />
                        <span>Loading agents...</span>
                    </div>
                ) : error ? (
                    /* Error state */
                    <div className="flex flex-col items-center justify-center h-full text-error-color">
                        <i className="fa-solid fa-circle-exclamation text-2xl mb-2" />
                        <span className="mb-2">{error}</span>
                        <Button className="outline grey text-sm" onClick={handleRefresh}>
                            <i className="fa-solid fa-rotate" />
                            <span>Retry</span>
                        </Button>
                    </div>
                ) : agents.length === 0 ? (
                    /* Empty state */
                    <div className="flex flex-col items-center justify-center h-full text-secondary-text-color">
                        <i className="fa-solid fa-robot text-4xl mb-4 text-accent-color" />
                        <span className="text-lg font-medium text-main-text-color mb-2">No agents configured</span>
                        <span className="text-sm mb-4">Create your first agent to get started</span>
                        <Button className="solid accent text-sm" onClick={handleCreateAgent}>
                            <i className="fa-solid fa-plus" />
                            <span>Create Agent</span>
                        </Button>
                    </div>
                ) : (
                    /* Agent list */
                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                        {agents.map((agent) => (
                            <div
                                key={agent.id}
                                className="border border-border-color rounded-lg p-4 bg-secondary-bg hover:bg-secondary-bg-hover transition-colors"
                            >
                                {/* Header: name, role, status */}
                                <div className="flex items-start justify-between mb-3">
                                    <div className="flex items-center gap-2">
                                        <div
                                            className="w-8 h-8 rounded-full flex items-center justify-center"
                                            style={{ backgroundColor: agent.displayColor + "33", color: agent.displayColor }}
                                        >
                                            <i className={agent.displayIcon} />
                                        </div>
                                        <div>
                                            <div className="font-medium text-main-text-color">{agent.name}</div>
                                            <div
                                                className="text-xs px-2 py-0.5 rounded"
                                                style={{
                                                    backgroundColor: agent.displayColor + "33",
                                                    color: agent.displayColor,
                                                }}
                                            >
                                                {agent.role}
                                            </div>
                                        </div>
                                    </div>
                                    {/* Status indicator */}
                                    <div className="flex items-center gap-1">
                                        <div
                                            className={`w-2 h-2 rounded-full ${
                                                agent.enabled ? "bg-success-color" : "bg-error-color"
                                            }`}
                                        />
                                        <span className="text-xs text-secondary-text-color">
                                            {agent.enabled ? "Enabled" : "Disabled"}
                                        </span>
                                    </div>
                                </div>

                                {/* Skills count */}
                                {agent.skills && agent.skills.length > 0 && (
                                    <div className="flex items-center gap-1 text-sm text-secondary-text-color mb-2">
                                        <i className="fa-solid fa-wand-magic-sparkles" />
                                        <span>{agent.skills.length} skill{agent.skills.length !== 1 ? "s" : ""}</span>
                                    </div>
                                )}

                                {/* MCP connections count */}
                                {agent.mcpConnections && agent.mcpConnections.length > 0 && (
                                    <div className="flex items-center gap-1 text-sm text-secondary-text-color mb-2">
                                        <i className="fa-solid fa-diagram-project" />
                                        <span>
                                            {agent.mcpConnections.length} MCP connection
                                            {agent.mcpConnections.length !== 1 ? "s" : ""}
                                        </span>
                                    </div>
                                )}

                                {/* Timestamps */}
                                <div className="text-xs text-secondary-text-color mb-3">
                                    Created: {dayjs(agent.createdAt / 1000).format("MMM D, YYYY")}
                                </div>

                                {/* Action buttons */}
                                <div className="flex items-center justify-end gap-2 pt-3 border-t border-border-color">
                                    <Button className="outline grey text-xs px-2 py-1" onClick={() => handleEditAgent(agent)}>
                                        <i className="fa-solid fa-pen" />
                                        <span>Edit</span>
                                    </Button>
                                    <Button
                                        className="outline red text-xs px-2 py-1"
                                        onClick={() => handleDeleteAgent(agent.id)}
                                    >
                                        <i className="fa-solid fa-trash" />
                                        <span>Delete</span>
                                    </Button>
                                </div>
                            </div>
                        ))}
                    </div>
                )}
            </div>

            {/* Dialog */}
            {dialogState && <AgentDialog model={model} onClose={() => model.closeDialog()} />}
        </div>
    );
}

AgentManagementView.displayName = "AgentManagementView";

export { AgentManagementView };
export default React.memo(AgentManagementView);
