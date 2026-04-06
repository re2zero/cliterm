// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { globalStore } from "@/app/store/jotaiStore";
import { atom, type PrimitiveAtom } from "jotai";

export interface AgentSkill {
    id: string;
    name: string;
    description: string;
    enabled: boolean;
}

export interface AgentMcpTool {
    id: string;
    name: string;
    url: string;
    enabled: boolean;
}

export interface AgentDefinition {
    id: string;
    name: string;
    role: string;
    description: string;
    icon: string;
    color: string;
    backend: string;
    model: string;
    provider: string;
    soul: string;
    agentMd: string;
    skills: AgentSkill[];
    mcpTools: AgentMcpTool[];
    createdAt: number;
}

export const defaultRoles: Omit<AgentDefinition, "id" | "createdAt">[] = [
    {
        name: "产品经理",
        role: "产品经理",
        description: "负责需求分析、产品规划和用户故事编写",
        icon: "fa-solid fa-lightbulb",
        color: "#f59e0b",
        backend: "claude",
        model: "claude-sonnet-4-5",
        provider: "claude",
        soul: "You are an experienced Product Manager. You excel at understanding user needs, defining product requirements, and creating actionable user stories. You think systematically and always consider the business value.",
        agentMd:
            "# Product Manager Agent\n\n## Role\nAnalyze requirements, plan products, write user stories.\n\n## Skills\n- Requirements analysis\n- User story writing\n- Product roadmap planning\n- Competitive analysis",
        skills: [
            { id: "req-analysis", name: "需求分析", description: "Analyze and prioritize requirements", enabled: true },
            {
                id: "user-story",
                name: "用户故事",
                description: "Write clear user stories with acceptance criteria",
                enabled: true,
            },
        ],
        mcpTools: [],
    },
    {
        name: "架构师",
        role: "架构师",
        description: "负责系统设计、技术选型和架构决策",
        icon: "fa-solid fa-sitemap",
        color: "#6366f1",
        backend: "claude",
        model: "claude-sonnet-4-5",
        provider: "claude",
        soul: "You are a Senior Software Architect with 15+ years of experience. You design scalable, maintainable systems. You prefer simple solutions and always consider tradeoffs. You communicate clearly with diagrams and structured documents.",
        agentMd:
            "# Architect Agent\n\n## Role\nSystem design, technology selection, architecture decisions.\n\n## Principles\n- Simplicity over complexity\n- Document all architectural decisions\n- Consider scalability, maintainability, and cost",
        skills: [
            {
                id: "system-design",
                name: "系统设计",
                description: "Design system architecture with clear diagrams",
                enabled: true,
            },
            {
                id: "tech-review",
                name: "技术评审",
                description: "Review and evaluate technology choices",
                enabled: true,
            },
        ],
        mcpTools: [],
    },
    {
        name: "开发工程师",
        role: "开发工程师",
        description: "负责代码实现、单元测试和代码审查",
        icon: "fa-solid fa-code",
        color: "#10b981",
        backend: "claude",
        model: "claude-sonnet-4-5",
        provider: "claude",
        soul: "You are a Senior Software Engineer who writes clean, tested, production-ready code. You follow best practices, write meaningful tests, and always consider edge cases. You explain your code decisions clearly.",
        agentMd:
            "# Developer Agent\n\n## Role\nCode implementation, unit testing, code review.\n\n## Standards\n- Write clean, readable code\n- Always include tests\n- Follow project conventions\n- Handle edge cases",
        skills: [
            { id: "code-gen", name: "代码生成", description: "Generate production-ready code", enabled: true },
            { id: "unit-test", name: "单元测试", description: "Write comprehensive unit tests", enabled: true },
            { id: "code-review", name: "代码审查", description: "Review code for quality and security", enabled: true },
        ],
        mcpTools: [],
    },
    {
        name: "测试工程师",
        role: "测试工程师",
        description: "负责测试用例设计、自动化测试和缺陷追踪",
        icon: "fa-solid fa-vial",
        color: "#ef4444",
        backend: "claude",
        model: "claude-sonnet-4-5",
        provider: "claude",
        soul: "You are a meticulous QA Engineer. You think about edge cases, boundary conditions, and failure modes. You design comprehensive test plans and automate everything that can be automated.",
        agentMd:
            "# QA Engineer Agent\n\n## Role\nTest case design, automated testing, defect tracking.\n\n## Approach\n- Design test cases from requirements\n- Automate regression tests\n- Track and prioritize defects",
        skills: [
            { id: "test-design", name: "测试设计", description: "Design comprehensive test cases", enabled: true },
            { id: "auto-test", name: "自动化测试", description: "Write automated test scripts", enabled: true },
        ],
        mcpTools: [],
    },
];

export const agentsAtom: PrimitiveAtom<AgentDefinition[]> = atom<AgentDefinition[]>(
    defaultRoles.map((role, i) => ({
        ...role,
        id: `role-${i}`,
        createdAt: Date.now(),
    }))
);

export const activeAgentIdAtom: PrimitiveAtom<string | null> = atom<string | null>("role-2");
export const agentsExpandedAtom: PrimitiveAtom<boolean> = atom<boolean>(false);

export function addAgent(agent: Omit<AgentDefinition, "id" | "createdAt">): void {
    const id = "role-" + Date.now();
    const newAgent: AgentDefinition = { ...agent, id, createdAt: Date.now() };
    globalStore.set(agentsAtom, (prev) => [...prev, newAgent]);
}

export function updateAgent(id: string, updates: Partial<AgentDefinition>): void {
    globalStore.set(agentsAtom, (prev) => prev.map((a) => (a.id === id ? { ...a, ...updates } : a)));
}

export function removeAgent(id: string): void {
    globalStore.set(agentsAtom, (prev) => prev.filter((a) => a.id !== id));
    const current = globalStore.get(activeAgentIdAtom);
    if (current === id) {
        globalStore.set(activeAgentIdAtom, null);
    }
}

export function getActiveAgent(): AgentDefinition | null {
    const agents = globalStore.get(agentsAtom);
    const activeId = globalStore.get(activeAgentIdAtom);
    return agents.find((a) => a.id === activeId) ?? null;
}

export function setActiveAgent(id: string): void {
    globalStore.set(activeAgentIdAtom, id);
}

export function resetToDefaultRoles(): void {
    globalStore.set(
        agentsAtom,
        defaultRoles.map((role, i) => ({
            ...role,
            id: `role-${i}`,
            createdAt: Date.now(),
        }))
    );
    globalStore.set(activeAgentIdAtom, "role-2");
}
