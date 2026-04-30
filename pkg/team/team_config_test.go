// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package team

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMemberYAML(t *testing.T) {
	yamlContent := `
name: "Test Member"
description: "A test member"
tool: opencode
model: "anthropic/claude-sonnet"
color: "#FF0000"
persona: "You are a test assistant."
skills:
  - go-testing
  - debugging
capabilities:
  - Read
  - Write
  - Bash
maxConcurrency: 5
maxRetries: 2
memory: persistent
`
	m, err := ParseMemberYAML([]byte(yamlContent))
	if err != nil {
		t.Fatalf("ParseMemberYAML failed: %v", err)
	}

	if m.Name != "Test Member" {
		t.Errorf("Name = %q, want %q", m.Name, "Test Member")
	}
	if m.Description != "A test member" {
		t.Errorf("Description = %q, want %q", m.Description, "A test member")
	}
	if m.Tool != ToolOpenCode {
		t.Errorf("Tool = %q, want %q", m.Tool, ToolOpenCode)
	}
	if m.Model != "anthropic/claude-sonnet" {
		t.Errorf("Model = %q, want %q", m.Model, "anthropic/claude-sonnet")
	}
	if m.Color != "#FF0000" {
		t.Errorf("Color = %q, want %q", m.Color, "#FF0000")
	}
	if m.Persona != "You are a test assistant." {
		t.Errorf("Persona = %q, want %q", m.Persona, "You are a test assistant.")
	}
	if len(m.Skills) != 2 || m.Skills[0] != "go-testing" || m.Skills[1] != "debugging" {
		t.Errorf("Skills = %v, want [go-testing, debugging]", m.Skills)
	}
	if len(m.Capabilities) != 3 {
		t.Errorf("Capabilities len = %d, want 3", len(m.Capabilities))
	}
	if m.MaxConcurrency != 5 {
		t.Errorf("MaxConcurrency = %d, want 5", m.MaxConcurrency)
	}
	if m.MaxRetries != 2 {
		t.Errorf("MaxRetries = %d, want 2", m.MaxRetries)
	}
	if m.Memory != MemoryPersistent {
		t.Errorf("Memory = %q, want %q", m.Memory, MemoryPersistent)
	}
}

func TestParseMemberYAML_Empty(t *testing.T) {
	m, err := ParseMemberYAML([]byte("{}"))
	if err != nil {
		t.Fatalf("ParseMemberYAML empty object failed: %v", err)
	}

	// Defaults should be applied.
	if m.Tool != ToolClaude {
		t.Errorf("Tool default = %q, want %q", m.Tool, ToolClaude)
	}
	if m.MaxConcurrency != 3 {
		t.Errorf("MaxConcurrency default = %d, want 3", m.MaxConcurrency)
	}
	if m.MaxRetries != 3 {
		t.Errorf("MaxRetries default = %d, want 3", m.MaxRetries)
	}
	if m.Memory != MemorySession {
		t.Errorf("Memory default = %q, want %q", m.Memory, MemorySession)
	}
	if m.Skills == nil {
		t.Error("Skills should be initialized to empty slice, not nil")
	}
	if m.McpServers == nil {
		t.Error("McpServers should be initialized to empty slice, not nil")
	}
	if m.Capabilities == nil {
		t.Error("Capabilities should be initialized to empty slice, not nil")
	}
}

func TestParseMemberYAML_WithMcpServers(t *testing.T) {
	yamlContent := `
name: "MCP Test"
tool: claude
mcpServers:
  - name: context7
    type: stdio
    command: npx
    args: ["-y", "@upstash/context7-mcp"]
    env:
      NODE_ENV: production
  - name: remote-api
    type: http
    url: "https://api.example.com/mcp"
    headers:
      Authorization: "Bearer token123"
`
	m, err := ParseMemberYAML([]byte(yamlContent))
	if err != nil {
		t.Fatalf("ParseMemberYAML failed: %v", err)
	}

	if len(m.McpServers) != 2 {
		t.Fatalf("McpServers len = %d, want 2", len(m.McpServers))
	}

	srv0 := m.McpServers[0]
	if srv0.Name != "context7" || srv0.Type != "stdio" || srv0.Command != "npx" {
		t.Errorf("McpServers[0] = %+v, unexpected values", srv0)
	}
	if len(srv0.Args) != 2 || srv0.Args[0] != "-y" {
		t.Errorf("McpServers[0].Args = %v, want [-y, @upstash/context7-mcp]", srv0.Args)
	}
	if srv0.Env["NODE_ENV"] != "production" {
		t.Errorf("McpServers[0].Env = %v, want NODE_ENV=production", srv0.Env)
	}

	srv1 := m.McpServers[1]
	if srv1.Name != "remote-api" || srv1.Type != "http" || srv1.URL != "https://api.example.com/mcp" {
		t.Errorf("McpServers[1] = %+v, unexpected values", srv1)
	}
	if srv1.Headers["Authorization"] != "Bearer token123" {
		t.Errorf("McpServers[1].Headers = %v, unexpected", srv1.Headers)
	}
}

func TestParseMemberYAML_InvalidYAML(t *testing.T) {
	_, err := ParseMemberYAML([]byte("name: [invalid yaml"))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestMemberFilenameToName(t *testing.T) {
	tests := []struct {
		filename string
		name     string
		want     string
	}{
		{"go-backend.yaml", "Go Backend", "Go Backend"},
		{"reviewer.yaml", "", "reviewer"},
		{"frontend.yml", "", "frontend"},
		{"custom.txt", "", "custom.txt"},
	}
	for _, tc := range tests {
		m := &TeamMember{Name: tc.name}
		memberFilenameToName(m, tc.filename)
		if m.Name != tc.want {
			t.Errorf("filename=%q name=%q -> got %q, want %q", tc.filename, tc.name, m.Name, tc.want)
		}
	}
}

func TestLoadGlobalTemplates(t *testing.T) {
	dir := t.TempDir()
	origGetWaveHome := getWaveHome
	getWaveHome = func() string { return dir }
	defer func() { getWaveHome = origGetWaveHome }()

	templatesDir := filepath.Join(dir, ".waveterm", teamTemplatesSubDir)
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write two valid templates.
	writeFile(t, filepath.Join(templatesDir, "dev.yaml"), `
name: "Dev"
tool: claude
`)
	writeFile(t, filepath.Join(templatesDir, "reviewer.yaml"), `
name: "Reviewer"
tool: opencode
maxConcurrency: 2
`)

	// Write an invalid YAML file (should be skipped with warning).
	writeFile(t, filepath.Join(templatesDir, "broken.yaml"), "name: [invalid")

	// Write a non-YAML file (should be skipped).
	writeFile(t, filepath.Join(templatesDir, "readme.txt"), "not yaml")

	members, err := LoadGlobalTemplates()
	if err != nil {
		t.Fatalf("LoadGlobalTemplates failed: %v", err)
	}

	if len(members) != 2 {
		t.Fatalf("got %d members, want 2", len(members))
	}

	names := map[string]bool{}
	for _, m := range members {
		names[m.Name] = true
	}
	if !names["Dev"] || !names["Reviewer"] {
		t.Errorf("members = %v, want Dev and Reviewer", names)
	}

	// Check defaults applied.
	for _, m := range members {
		if m.MaxConcurrency <= 0 {
			t.Errorf("member %q MaxConcurrency = %d, want > 0", m.Name, m.MaxConcurrency)
		}
	}
}

func TestLoadGlobalTemplates_NonexistentDir(t *testing.T) {
	dir := t.TempDir()
	origGetWaveHome := getWaveHome
	getWaveHome = func() string { return dir }
	defer func() { getWaveHome = origGetWaveHome }()

	members, err := LoadGlobalTemplates()
	if err != nil {
		t.Fatalf("LoadGlobalTemplates should return nil, nil for nonexistent dir: %v", err)
	}
	if members != nil {
		t.Errorf("expected nil slice, got %v", members)
	}
}

func TestLoadProjectConfig(t *testing.T) {
	dir := t.TempDir()

	// Setup global templates dir with one template.
	globalDir := filepath.Join(dir, ".waveterm", teamTemplatesSubDir)
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(globalDir, "go-backend.yaml"), `
name: "Go Backend Developer"
tool: opencode
model: "anthropic/claude-sonnet"
maxConcurrency: 3
`)

	// Setup project config with one full member and one template override.
	waveDir := filepath.Join(dir, ".wave")
	if err := os.MkdirAll(waveDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(waveDir, "team.yaml"), `
members:
  - name: "Project Dev"
    tool: claude
    persona: "You work on this specific project."
    skills: [react, typescript]
  - template: "Go Backend Developer"
    model: "anthropic/claude-opus"
    maxConcurrency: 5
`)

	origGetWaveHome := getWaveHome
	getWaveHome = func() string { return dir }
	defer func() { getWaveHome = origGetWaveHome }()

	members, err := LoadProjectConfig(dir)
	if err != nil {
		t.Fatalf("LoadProjectConfig failed: %v", err)
	}

	if len(members) != 2 {
		t.Fatalf("got %d members, want 2", len(members))
	}

	// Full member.
	if members[0].Name != "Project Dev" {
		t.Errorf("members[0].Name = %q, want %q", members[0].Name, "Project Dev")
	}
	if members[0].Tool != ToolClaude {
		t.Errorf("members[0].Tool = %q, want %q", members[0].Tool, ToolClaude)
	}
	if len(members[0].Skills) != 2 {
		t.Errorf("members[0].Skills = %v, want 2 skills", members[0].Skills)
	}

	// Template override.
	if members[1].Name != "Go Backend Developer" {
		t.Errorf("members[1].Name = %q, want %q", members[1].Name, "Go Backend Developer")
	}
	if members[1].Tool != ToolOpenCode {
		t.Errorf("members[1].Tool = %q (should be inherited from template)", members[1].Tool)
	}
	if members[1].Model != "anthropic/claude-opus" {
		t.Errorf("members[1].Model = %q, want %q (override)", members[1].Model, "anthropic/claude-opus")
	}
	if members[1].MaxConcurrency != 5 {
		t.Errorf("members[1].MaxConcurrency = %d, want 5 (override)", members[1].MaxConcurrency)
	}
}

func TestLoadProjectConfig_NonexistentFile(t *testing.T) {
	dir := t.TempDir()

	members, err := LoadProjectConfig(dir)
	if err != nil {
		t.Fatalf("LoadProjectConfig should return nil, nil for nonexistent file: %v", err)
	}
	if members != nil {
		t.Errorf("expected nil, got %v", members)
	}
}

func TestLoadProjectConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	waveDir := filepath.Join(dir, ".wave")
	if err := os.MkdirAll(waveDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(waveDir, "team.yaml"), "members: [invalid")

	_, err := LoadProjectConfig(dir)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestGetEffectiveConfig(t *testing.T) {
	dir := t.TempDir()

	// Global templates.
	globalDir := filepath.Join(dir, ".waveterm", teamTemplatesSubDir)
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(globalDir, "shared.yaml"), `
name: "Shared"
tool: claude
`)
	writeFile(t, filepath.Join(globalDir, "override-me.yaml"), `
name: "Override Me"
tool: opencode
`)

	// Project config overrides "Override Me".
	waveDir := filepath.Join(dir, ".wave")
	if err := os.MkdirAll(waveDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(waveDir, "team.yaml"), `
members:
  - name: "Override Me"
    tool: cursor
    color: "#FF0000"
`)

	origGetWaveHome := getWaveHome
	getWaveHome = func() string { return dir }
	defer func() { getWaveHome = origGetWaveHome }()

	members, err := GetEffectiveConfig(dir)
	if err != nil {
		t.Fatalf("GetEffectiveConfig failed: %v", err)
	}

	if len(members) != 2 {
		t.Fatalf("got %d members, want 2", len(members))
	}

	// Find members by name.
	byName := map[string]TeamMember{}
	for _, m := range members {
		byName[m.Name] = m
	}

	shared := byName["Shared"]
	if shared.Tool != ToolClaude {
		t.Errorf("Shared.Tool = %q, want %q (from global)", shared.Tool, ToolClaude)
	}

	override := byName["Override Me"]
	if override.Tool != "cursor" {
		t.Errorf("Override Me.Tool = %q, want cursor (from project)", override.Tool)
	}
	if override.Color != "#FF0000" {
		t.Errorf("Override Me.Color = %q, want #FF0000", override.Color)
	}
}

func TestGetEffectiveConfig_NoProject(t *testing.T) {
	dir := t.TempDir()

	globalDir := filepath.Join(dir, ".waveterm", teamTemplatesSubDir)
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(globalDir, "only-global.yaml"), `
name: "Only Global"
tool: claude
`)

	origGetWaveHome := getWaveHome
	getWaveHome = func() string { return dir }
	defer func() { getWaveHome = origGetWaveHome }()

	members, err := GetEffectiveConfig(dir)
	if err != nil {
		t.Fatalf("GetEffectiveConfig failed: %v", err)
	}
	if len(members) != 1 || members[0].Name != "Only Global" {
		t.Errorf("expected [Only Global], got %v", members)
	}
}

func TestLoadDefaultTemplates(t *testing.T) {
	members, err := LoadDefaultTemplates()
	if err != nil {
		t.Fatalf("LoadDefaultTemplates failed: %v", err)
	}

	if len(members) != 4 {
		t.Fatalf("got %d default templates, want 4", len(members))
	}

	names := map[string]bool{}
	for _, m := range members {
		names[m.Name] = true
		// Every default template should have defaults applied.
		if m.Tool == "" {
			t.Errorf("template %q has empty tool", m.Name)
		}
		if m.MaxConcurrency <= 0 {
			t.Errorf("template %q has invalid MaxConcurrency", m.Name)
		}
	}

	expected := []string{"Go Backend Developer", "Frontend Developer", "Code Reviewer", "General Assistant"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing default template %q", name)
		}
	}
}

func TestApplyDefaults(t *testing.T) {
	tests := []struct {
		name    string
		input   TeamMember
		want    TeamMember
	}{
		{
			name:  "all defaults",
			input: TeamMember{},
			want: TeamMember{
				Tool:           ToolClaude,
				MaxConcurrency: 3,
				MaxRetries:     3,
				Memory:         MemorySession,
				Skills:         []string{},
				McpServers:     []MCPConfig{},
				Capabilities:   []string{},
			},
		},
		{
			name: "partial values preserved",
			input: TeamMember{
				Name:           "Test",
				Tool:           ToolCursor,
				MaxConcurrency: 10,
			},
			want: TeamMember{
				Name:           "Test",
				Tool:           ToolCursor,
				MaxConcurrency: 10,
				MaxRetries:     3,
				Memory:         MemorySession,
				Skills:         []string{},
				McpServers:     []MCPConfig{},
				Capabilities:   []string{},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			applyDefaults(&tc.input)
			if tc.input.Tool != tc.want.Tool {
				t.Errorf("Tool = %q, want %q", tc.input.Tool, tc.want.Tool)
			}
			if tc.input.MaxConcurrency != tc.want.MaxConcurrency {
				t.Errorf("MaxConcurrency = %d, want %d", tc.input.MaxConcurrency, tc.want.MaxConcurrency)
			}
			if tc.input.MaxRetries != tc.want.MaxRetries {
				t.Errorf("MaxRetries = %d, want %d", tc.input.MaxRetries, tc.want.MaxRetries)
			}
			if tc.input.Memory != tc.want.Memory {
				t.Errorf("Memory = %q, want %q", tc.input.Memory, tc.want.Memory)
			}
		})
	}
}

func TestGetGlobalTemplatesDir(t *testing.T) {
	origGetWaveHome := getWaveHome
	getWaveHome = func() string { return "/home/testuser" }
	defer func() { getWaveHome = origGetWaveHome }()

	dir := GetGlobalTemplatesDir()
	expected := filepath.Join("/home/testuser", ".waveterm", teamTemplatesSubDir)
	if dir != expected {
		t.Errorf("GetGlobalTemplatesDir = %q, want %q", dir, expected)
	}
}

func TestIsYAMLFile(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"config.yaml", true},
		{"config.yml", true},
		{"config.YAML", false},
		{"config.json", false},
		{"readme", false},
		{".hidden.yaml", true},
	}
	for _, tc := range tests {
		got := isYAMLFile(tc.name)
		if got != tc.want {
			t.Errorf("isYAMLFile(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
