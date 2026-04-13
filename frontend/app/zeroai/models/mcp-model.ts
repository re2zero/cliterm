// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

// MCPServerInfo is a global type from frontend/types/gotypes.d.ts (declare global)
import { globalStore } from "@/app/store/jotaiStore";
import { atom, type PrimitiveAtom } from "jotai";
import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";

// Atoms for MCP servers state
export const mcpServersAtom = atom<MCPServerInfo[]>([]) as PrimitiveAtom<MCPServerInfo[]>;
export const mcpServersLoadingAtom = atom(false) as PrimitiveAtom<boolean>;
export const mcpServersErrorAtom = atom<string | null>(null) as PrimitiveAtom<string | null>;

// Fallback hardcoded MCP servers (used before backend is ready)
const FALLBACK_MCP_SERVERS: MCPServerInfo[] = [
	{ id: "mcp-filesystem", name: "Filesystem", description: "Access filesystem operations", config: {}, enabled: true, createdAt: 0, updatedAt: 0 },
	{ id: "mcp-github", name: "GitHub", description: "GitHub integration", config: {}, enabled: true, createdAt: 0, updatedAt: 0 },
	{ id: "mcp-database", name: "Database", description: "Database connectivity", config: {}, enabled: true, createdAt: 0, updatedAt: 0 },
];

// Fetch MCP servers from backend
export async function fetchMCPServers(): Promise<void> {
	globalStore.set(mcpServersLoadingAtom, true);
	globalStore.set(mcpServersErrorAtom, null);

	try {
		const result = await RpcApi.MCPServerListCommand(TabRpcClient, {});
		const servers = result.servers || [];
		globalStore.set(mcpServersAtom, servers);
		console.log("[mcp-model] fetched MCP servers:", servers.length, "servers");
	} catch (error) {
		console.error("[mcp-model] fetch MCP servers error:", error);
		// On error, use fallback MCP servers
		console.log("[mcp-model] using fallback MCP servers");
		globalStore.set(mcpServersAtom, FALLBACK_MCP_SERVERS);
		globalStore.set(mcpServersErrorAtom, "Failed to fetch MCP servers, using local fallback");
	} finally {
		globalStore.set(mcpServersLoadingAtom, false);
	}
}

// Create a new MCP server
export async function createMCPServer(name: string, description: string, config: Record<string, any> = {}, enabled: boolean = true): Promise<string> {
	globalStore.set(mcpServersLoadingAtom, true);

	try {
		const result = await RpcApi.MCPServerRegisterCommand(TabRpcClient, { name, description, config, enabled });
		const serverId = result.id;

		// Refresh MCP servers after creation
		await fetchMCPServers();

		console.log("[mcp-model] created MCP server:", serverId);
		return serverId;
	} catch (error) {
		console.error("[mcp-model] create MCP server error:", error);
		throw new Error(`Failed to create MCP server: ${error instanceof Error ? error.message : String(error)}`);
	} finally {
		globalStore.set(mcpServersLoadingAtom, false);
	}
}

// Update an existing MCP server
export async function updateMCPServer(
	id: string,
	name: string,
	description: string,
	config: Record<string, any> = {},
	enabled: boolean
): Promise<void> {
	globalStore.set(mcpServersLoadingAtom, true);

	try {
		await RpcApi.MCPServerUpdateCommand(TabRpcClient, { id, name, description, config, enabled });

		// Refresh MCP servers after update
		await fetchMCPServers();

		console.log("[mcp-model] updated MCP server:", id);
	} catch (error) {
		console.error("[mcp-model] update MCP server error:", error);
		throw new Error(`Failed to update MCP server: ${error instanceof Error ? error.message : String(error)}`);
	} finally {
		globalStore.set(mcpServersLoadingAtom, false);
	}
}

// Set MCP server enabled status
export async function setMCPServerEnabled(id: string, enabled: boolean): Promise<void> {
	globalStore.set(mcpServersLoadingAtom, true);

	try {
		await RpcApi.MCPServerSetEnabledCommand(TabRpcClient, { id, enabled });

		// Refresh MCP servers after update
		await fetchMCPServers();

		console.log("[mcp-model] set MCP server enabled:", id, "=", enabled);
	} catch (error) {
		console.error("[mcp-model] set MCP server enabled error:", error);
		throw new Error(`Failed to set MCP server enabled: ${error instanceof Error ? error.message : String(error)}`);
	} finally {
		globalStore.set(mcpServersLoadingAtom, false);
	}
}

// Delete an MCP server
export async function deleteMCPServer(id: string): Promise<void> {
	globalStore.set(mcpServersLoadingAtom, true);

	try {
		await RpcApi.MCPServerDeleteCommand(TabRpcClient, { id });

		// Refresh MCP servers after deletion
		await fetchMCPServers();

		console.log("[mcp-model] deleted MCP server:", id);
	} catch (error) {
		console.error("[mcp-model] delete MCP server error:", error);
		throw new Error(`Failed to delete MCP server: ${error instanceof Error ? error.message : String(error)}`);
	} finally {
		globalStore.set(mcpServersLoadingAtom, false);
	}
}
