// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

// SkillInfo is a global type from frontend/types/gotypes.d.ts (declare global)
import { globalStore } from "@/app/store/jotaiStore";
import { atom, type PrimitiveAtom } from "jotai";
import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";

// Atoms for skills state
export const skillsAtom = atom<SkillInfo[]>([]) as PrimitiveAtom<SkillInfo[]>;
export const skillsLoadingAtom = atom(false) as PrimitiveAtom<boolean>;
export const skillsErrorAtom = atom<string | null>(null) as PrimitiveAtom<string | null>;

// Fallback hardcoded skills (used before backend is ready)
const FALLBACK_SKILLS: SkillInfo[] = [
	{ id: "skill-code-analysis", name: "Code Analysis", description: "Analyze code for bugs and improvements", createdAt: 0, updatedAt: 0 },
	{ id: "skill-debugging", name: "Debugging", description: "Debug issues and find root causes", createdAt: 0, updatedAt: 0 },
	{ id: "skill-testing", name: "Testing", description: "Write and run tests", createdAt: 0, updatedAt: 0 },
	{ id: "skill-documentation", name: "Documentation", description: "Write code documentation", createdAt: 0, updatedAt: 0 },
];

// Fetch skills from backend
export async function fetchSkills(): Promise<void> {
	globalStore.set(skillsLoadingAtom, true);
	globalStore.set(skillsErrorAtom, null);

	try {
		const result = await RpcApi.SkillsListCommand(TabRpcClient, {});
		const skills = result.skills || [];
		globalStore.set(skillsAtom, skills);
		console.log("[skills-model] fetched skills:", skills.length, "skills");
	} catch (error) {
		console.error("[skills-model] fetch skills error:", error);
		// On error, use fallback skills
		console.log("[skills-model] using fallback skills");
		globalStore.set(skillsAtom, FALLBACK_SKILLS);
		globalStore.set(skillsErrorAtom, "Failed to fetch skills, using local fallback");
	} finally {
		globalStore.set(skillsLoadingAtom, false);
	}
}

// Create a new skill
export async function createSkill(name: string, description: string): Promise<string> {
	globalStore.set(skillsLoadingAtom, true);

	try {
		const result = await RpcApi.SkillsRegisterCommand(TabRpcClient, { name, description });
		const skillId = result.id;

		// Refresh skills after creation
		await fetchSkills();

		console.log("[skills-model] created skill:", skillId);
		return skillId;
	} catch (error) {
		console.error("[skills-model] create skill error:", error);
		throw new Error(`Failed to create skill: ${error instanceof Error ? error.message : String(error)}`);
	} finally {
		globalStore.set(skillsLoadingAtom, false);
	}
}

// Update an existing skill
export async function updateSkill(id: string, name: string, description: string): Promise<void> {
	globalStore.set(skillsLoadingAtom, true);

	try {
		await RpcApi.SkillsUpdateCommand(TabRpcClient, { id, name, description });

		// Refresh skills after update
		await fetchSkills();

		console.log("[skills-model] updated skill:", id);
	} catch (error) {
		console.error("[skills-model] update skill error:", error);
		throw new Error(`Failed to update skill: ${error instanceof Error ? error.message : String(error)}`);
	} finally {
		globalStore.set(skillsLoadingAtom, false);
	}
}

// Delete a skill
export async function deleteSkill(id: string): Promise<void> {
	globalStore.set(skillsLoadingAtom, true);

	try {
		await RpcApi.SkillsDeleteCommand(TabRpcClient, { id });

		// Refresh skills after deletion
		await fetchSkills();

		console.log("[skills-model] deleted skill:", id);
	} catch (error) {
		console.error("[skills-model] delete skill error:", error);
		throw new Error(`Failed to delete skill: ${error instanceof Error ? error.message : String(error)}`);
	} finally {
		globalStore.set(skillsLoadingAtom, false);
	}
}
