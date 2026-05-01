// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package team

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wavetermdev/waveterm/pkg/wavebase"
)

func TestLoadPersona_Inline(t *testing.T) {
	member := &TeamMember{
		Persona:     "You are a helpful assistant.",
		PersonaPath: "",
	}

	content, err := loadPersona(member, "/nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "You are a helpful assistant." {
		t.Errorf("expected inline persona, got %q", content)
	}
}

func TestLoadPersona_Empty(t *testing.T) {
	member := &TeamMember{}

	content, err := loadPersona(member, "/nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "" {
		t.Errorf("expected empty, got %q", content)
	}
}

func TestLoadPersona_FilePath(t *testing.T) {
	tmpDir := t.TempDir()
	personaFile := filepath.Join(tmpDir, "persona.md")
	if err := os.WriteFile(personaFile, []byte("File persona content."), 0644); err != nil {
		t.Fatal(err)
	}

	member := &TeamMember{
		Persona:     "inline fallback",
		PersonaPath: "persona.md",
	}

	content, err := loadPersona(member, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "File persona content." {
		t.Errorf("expected file persona, got %q", content)
	}
}

func TestLoadPersona_FilePath_Absolute(t *testing.T) {
	tmpDir := t.TempDir()
	personaFile := filepath.Join(tmpDir, "abs-persona.md")
	if err := os.WriteFile(personaFile, []byte("Absolute path persona."), 0644); err != nil {
		t.Fatal(err)
	}

	member := &TeamMember{
		Persona:     "inline fallback",
		PersonaPath: personaFile,
	}

	content, err := loadPersona(member, "/nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "Absolute path persona." {
		t.Errorf("expected absolute path persona, got %q", content)
	}
}

func TestLoadPersona_FileNotFound_FallbackInline(t *testing.T) {
	member := &TeamMember{
		Persona:     "fallback content",
		PersonaPath: "nonexistent.md",
	}

	content, err := loadPersona(member, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "fallback content" {
		t.Errorf("expected fallback, got %q", content)
	}
}

func TestLoadPersona_FileNotFound_NoFallback(t *testing.T) {
	member := &TeamMember{
		Persona:     "",
		PersonaPath: "nonexistent.md",
	}

	_, err := loadPersona(member, t.TempDir())
	if err == nil {
		t.Fatal("expected error when both personaPath and persona are unavailable")
	}
}

func TestResolvePersonaPath(t *testing.T) {
	tests := []struct {
		name      string
		rawPath   string
		configDir string
		want      string
	}{
		{"absolute", "/etc/persona.md", "/config", "/etc/persona.md"},
		{"relative", "persona.md", "/config", "/config/persona.md"},
		{"dot-slash", "./persona.md", "/config", "/config/persona.md"},
		{"nested", "sub/persona.md", "/config", "/config/sub/persona.md"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolvePersonaPath(tt.rawPath, tt.configDir)
			if got != tt.want {
				t.Errorf("resolvePersonaPath(%q, %q) = %q, want %q", tt.rawPath, tt.configDir, got, tt.want)
			}
		})
	}
}

func TestRemoveMarkerSection(t *testing.T) {
	marker := "<!-- team:persona -->"

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			"no markers",
			"hello world",
			"hello world",
		},
		{
			"with section",
			"before\n<!-- team:persona -->\npersona content\n<!-- team:persona -->\nafter",
			"before\n\nafter",
		},
		{
			"only section",
			"<!-- team:persona -->\npersona\n<!-- team:persona -->",
			"",
		},
		{
			"open marker no close",
			"<!-- team:persona -->\norphan",
			"<!-- team:persona -->\norphan",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := removeMarkerSection(tt.content, marker)
			if got != tt.want {
				t.Errorf("removeMarkerSection:\ngot:  %q\nwant: %q", got, tt.want)
			}
		})
	}
}

func TestLinkSkills_GlobalSource(t *testing.T) {
	tmpDir := t.TempDir()
	cliDir := filepath.Join(tmpDir, "cli-skills")
	teamSkillsDir := filepath.Join(tmpDir, "team-skills", "go-testing")
	if err := os.MkdirAll(teamSkillsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Use linkSingleSkill directly to control paths
	err := linkSingleSkill("go-testing", cliDir, filepath.Join(tmpDir, "team-skills"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	linkPath := filepath.Join(cliDir, "go-testing")
	fi, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("symlink not created: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("expected symlink")
	}

	target, _ := os.Readlink(linkPath)
	if target != teamSkillsDir {
		t.Errorf("symlink target = %q, want %q", target, teamSkillsDir)
	}
}

func TestLinkSkills_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	cliDir := filepath.Join(tmpDir, "cli-skills")
	teamSkillsDir := filepath.Join(tmpDir, "team-skills", "go-testing")
	if err := os.MkdirAll(teamSkillsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// First link
	if err := linkSingleSkill("go-testing", cliDir, filepath.Join(tmpDir, "team-skills"), ""); err != nil {
		t.Fatal(err)
	}

	// Second link should be idempotent
	if err := linkSingleSkill("go-testing", cliDir, filepath.Join(tmpDir, "team-skills"), ""); err != nil {
		t.Fatalf("second link failed: %v", err)
	}
}

func TestLinkSkills_SkipExistingRealDir(t *testing.T) {
	tmpDir := t.TempDir()
	cliDir := filepath.Join(tmpDir, "cli-skills")
	realSkillDir := filepath.Join(cliDir, "existing-skill")
	if err := os.MkdirAll(realSkillDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create a file to prove it's a real dir
	if err := os.WriteFile(filepath.Join(realSkillDir, "keep.txt"), []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}

	// Should skip without error
	if err := linkSingleSkill("existing-skill", cliDir, filepath.Join(tmpDir, "team-skills"), ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Original dir should still exist with its file
	data, err := os.ReadFile(filepath.Join(realSkillDir, "keep.txt"))
	if err != nil {
		t.Fatal("real skill dir was modified")
	}
	if string(data) != "keep" {
		t.Errorf("file content changed: %q", string(data))
	}
}

func TestLinkSkills_ProjectOverridesGlobal(t *testing.T) {
	tmpDir := t.TempDir()
	cliDir := filepath.Join(tmpDir, "cli-skills")

	globalSkillDir := filepath.Join(tmpDir, "team-skills", "my-skill")
	if err := os.MkdirAll(globalSkillDir, 0755); err != nil {
		t.Fatal(err)
	}

	projectSkillDir := filepath.Join(tmpDir, "project", ".wave", "skills", "my-skill")
	if err := os.MkdirAll(projectSkillDir, 0755); err != nil {
		t.Fatal(err)
	}

	err := linkSingleSkill("my-skill", cliDir, filepath.Join(tmpDir, "team-skills"), filepath.Join(tmpDir, "project", ".wave", "skills"))
	if err != nil {
		t.Fatal(err)
	}

	target, _ := os.Readlink(filepath.Join(cliDir, "my-skill"))
	if target != projectSkillDir {
		t.Errorf("expected project skill source, got %q", target)
	}
}

func TestUnlinkSkills(t *testing.T) {
	tmpDir := t.TempDir()
	cliDir := filepath.Join(tmpDir, ".claude", "skills")
	teamSkillDir := filepath.Join(tmpDir, ".waveterm", "team-skills", "go-testing")
	if err := os.MkdirAll(teamSkillDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create symlink directly
	if err := os.MkdirAll(cliDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(teamSkillDir, filepath.Join(cliDir, "go-testing")); err != nil {
		t.Fatal(err)
	}

	// Mock home dir so unlinkSkills resolves the correct path
	origHome := getWaveHome
	t.Cleanup(func() { getWaveHome = origHome })
	getWaveHome = func() string { return tmpDir }

	// Unlink should remove the symlink
	if err := unlinkSkills([]string{"go-testing"}, "claude"); err != nil {
		t.Fatalf("unlink failed: %v", err)
	}

	if _, err := os.Lstat(filepath.Join(cliDir, "go-testing")); !os.IsNotExist(err) {
		t.Error("symlink should be removed")
	}

	// Original team skill should still exist
	if _, err := os.Stat(teamSkillDir); err != nil {
		t.Error("team skill source should still exist")
	}
}

func TestUnlinkSkills_SkipRealDir(t *testing.T) {
	tmpDir := t.TempDir()
	cliDir := filepath.Join(tmpDir, "cli-skills")
	realDir := filepath.Join(cliDir, "native-skill")
	if err := os.MkdirAll(realDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Should not remove real directories
	if err := unlinkSkills([]string{"native-skill"}, "claude"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(realDir); err != nil {
		t.Error("real skill directory should not be removed")
	}
}

func TestUnlinkSkills_NotExist(t *testing.T) {
	err := unlinkSkills([]string{"nonexistent"}, "claude")
	if err != nil {
		t.Fatalf("should not error on missing skill: %v", err)
	}
}

func TestSanitizeFileName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"normal", "context7", "context7"},
		{"with-hyphen", "my-server", "my-server"},
		{"with-underscore", "my_server", "my_server"},
		{"with-spaces", "my server", "my_server"},
		{"path-injection", "../../etc/passwd", "______etc_passwd"},
		{"empty", "", "unnamed"},
		{"special-chars", "test@#$%", "test____"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeFileName(tt.in)
			if got != tt.want {
				t.Errorf("sanitizeFileName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestInjectPersonaClaude(t *testing.T) {
	tmpDir := t.TempDir()

	// Override home dir for test
	origHome := wavebase.GetHomeDir
	t.Cleanup(func() { getWaveHome = origHome })
	getWaveHome = func() string { return tmpDir }

	persona := "You are a Go expert.\nFocus on testing."

	if err := injectPersonaClaude(persona); err != nil {
		t.Fatalf("injectPersonaClaude failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, ".claude", "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if content == "" {
		t.Fatal("CLAUDE.md is empty")
	}

	// Check markers are present
	if count := countOccurrences(content, teamPersonaMarker); count != 2 {
		t.Errorf("expected 2 markers, got %d in:\n%s", count, content)
	}
}

func TestInjectPersonaClaude_ReplacesExisting(t *testing.T) {
	tmpDir := t.TempDir()

	origHome := getWaveHome
	t.Cleanup(func() { getWaveHome = origHome })
	getWaveHome = func() string { return tmpDir }

	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write initial CLAUDE.md with a persona section
	initial := "# My Project\n\n" + teamPersonaMarker + "\nOld persona\n" + teamPersonaMarker + "\nSome notes."
	if err := os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	if err := injectPersonaClaude("New persona."); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(claudeDir, "CLAUDE.md"))
	content := string(data)

	// Old persona should be gone
	if strings.Contains(content, "Old persona") {
		t.Error("old persona should be replaced")
	}
	// New persona should be present
	if !strings.Contains(content, "New persona.") {
		t.Error("new persona should be present")
	}
	// Other content preserved
	if !strings.Contains(content, "# My Project") {
		t.Error("non-persona content should be preserved")
	}
	if !strings.Contains(content, "Some notes.") {
		t.Error("non-persona content should be preserved")
	}
}

func TestInjectPersonaOpenCode(t *testing.T) {
	// Create in temp dir and cd to it
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	persona := "You are a frontend developer."

	if err := injectPersonaOpenCode(persona); err != nil {
		t.Fatalf("injectPersonaOpenCode failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != persona {
		t.Errorf("AGENTS.md content = %q, want %q", string(data), persona)
	}
}

func TestInjectMCPClaude(t *testing.T) {
	tmpDir := t.TempDir()

	origHome := getWaveHome
	t.Cleanup(func() { getWaveHome = origHome })
	getWaveHome = func() string { return tmpDir }

	member := &TeamMember{
		Tool: ToolClaude,
		McpServers: []MCPConfig{
			{Name: "context7", Type: "stdio", Command: "npx", Args: []string{"-y", "@context7/mcp"}},
			{Name: "remote-api", Type: "http", URL: "https://api.example.com/mcp", Headers: map[string]string{"Authorization": "Bearer token"}},
		},
	}

	if err := injectMCP(member); err != nil {
		t.Fatalf("injectMCP failed: %v", err)
	}

	// Check stdio server file
	stdioFile := filepath.Join(tmpDir, ".claude", "mcp", "context7.json")
	data, err := os.ReadFile(stdioFile)
	if err != nil {
		t.Fatalf("missing stdio MCP file: %v", err)
	}
	stdioContent := string(data)
	if !strings.Contains(stdioContent, `"command": "npx"`) {
		t.Errorf("stdio config missing command: %s", stdioContent)
	}

	// Check http server file
	httpFile := filepath.Join(tmpDir, ".claude", "mcp", "remote-api.json")
	data, err = os.ReadFile(httpFile)
	if err != nil {
		t.Fatalf("missing http MCP file: %v", err)
	}
	httpContent := string(data)
	if !strings.Contains(httpContent, `"url": "https://api.example.com/mcp"`) {
		t.Errorf("http config missing url: %s", httpContent)
	}
}

func TestInjectMCPClaude_SanitizesFileName(t *testing.T) {
	tmpDir := t.TempDir()

	origHome := getWaveHome
	t.Cleanup(func() { getWaveHome = origHome })
	getWaveHome = func() string { return tmpDir }

	member := &TeamMember{
		Tool:       ToolClaude,
		McpServers: []MCPConfig{{Name: "../../etc/evil", Type: "stdio", Command: "echo", Args: nil}},
	}

	if err := injectMCP(member); err != nil {
		t.Fatal(err)
	}

	// Should not create file outside mcp dir
	entries, err := os.ReadDir(filepath.Join(tmpDir, ".claude", "mcp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}
	if entries[0].Name() != "______etc_evil.json" {
		t.Errorf("unexpected filename: %s", entries[0].Name())
	}
}

func TestInjectMCPOpenCode(t *testing.T) {
	tmpDir := t.TempDir()

	origHome := getWaveHome
	t.Cleanup(func() { getWaveHome = origHome })
	getWaveHome = func() string { return tmpDir }

	member := &TeamMember{
		Tool: ToolOpenCode,
		McpServers: []MCPConfig{
			{Name: "playwright", Type: "stdio", Command: "npx", Args: []string{"-y", "@playwright/mcp@latest"}},
		},
	}

	if err := injectMCP(member); err != nil {
		t.Fatalf("injectMCP failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, ".config", "opencode", "mcp.json"))
	if err != nil {
		t.Fatalf("missing opencode MCP file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "playwright") {
		t.Errorf("opencode MCP config missing server name: %s", content)
	}
	if !strings.Contains(content, `"type": "stdio"`) {
		t.Errorf("opencode MCP config missing type: %s", content)
	}
}

func TestLinkSkills_UnsupportedTool(t *testing.T) {
	err := linkSkills([]string{"go-testing"}, "cursor", "")
	if err != nil {
		t.Errorf("unsupported tool should return nil, got: %v", err)
	}
}

func TestUnlinkSkills_UnsupportedTool(t *testing.T) {
	err := unlinkSkills([]string{"go-testing"}, "cursor")
	if err != nil {
		t.Errorf("unsupported tool should return nil, got: %v", err)
	}
}

func TestInjectMCP_UnknownType(t *testing.T) {
	tmpDir := t.TempDir()

	origHome := getWaveHome
	t.Cleanup(func() { getWaveHome = origHome })
	getWaveHome = func() string { return tmpDir }

	member := &TeamMember{
		Tool:       ToolClaude,
		McpServers: []MCPConfig{{Name: "test", Type: "unknown-type"}},
	}

	// Should not error, just skip with a warning
	err := injectMCP(member)
	if err != nil {
		t.Fatalf("unknown MCP type should not error, got: %v", err)
	}
}

func TestInjectMCP_UnsupportedTool(t *testing.T) {
	member := &TeamMember{
		Tool:       ToolAider,
		McpServers: []MCPConfig{{Name: "test", Type: "stdio", Command: "echo"}},
	}

	err := injectMCP(member)
	if err != nil {
		t.Fatalf("unsupported tool MCP should not error, got: %v", err)
	}
}

func TestInjectPersona_DefaultTool(t *testing.T) {
	err := injectPersona("test persona", &TeamMember{Tool: "aider"})
	if err != nil {
		t.Fatalf("unsupported tool should return nil, got: %v", err)
	}
}

func TestInjectPersona_ClaudeTool(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := getWaveHome
	t.Cleanup(func() { getWaveHome = origHome })
	getWaveHome = func() string { return tmpDir }

	err := injectPersona("test persona for claude", &TeamMember{Tool: ToolClaude})
	if err != nil {
		t.Fatalf("injectPersona for claude failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, ".claude", "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "test persona for claude") {
		t.Error("persona not found in CLAUDE.md")
	}
}

func TestInjectPersona_OpenCodeTool(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	err := injectPersona("test persona for opencode", &TeamMember{Tool: ToolOpenCode})
	if err != nil {
		t.Fatalf("injectPersona for opencode failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "test persona for opencode" {
		t.Errorf("AGENTS.md = %q, want %q", string(data), "test persona for opencode")
	}
}

func TestGetTeamSkillsDir(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := getWaveHome
	t.Cleanup(func() { getWaveHome = origHome })
	getWaveHome = func() string { return tmpDir }

	dir := getTeamSkillsDir()
	expected := filepath.Join(tmpDir, ".waveterm", teamSkillsSubDir)
	if dir != expected {
		t.Errorf("getTeamSkillsDir() = %q, want %q", dir, expected)
	}
}

func TestGetCLISkillDir(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := getWaveHome
	t.Cleanup(func() { getWaveHome = origHome })
	getWaveHome = func() string { return tmpDir }

	tests := []struct {
		tool string
		want string
	}{
		{ToolClaude, filepath.Join(tmpDir, ".claude/skills")},
		{ToolOpenCode, filepath.Join(tmpDir, ".config/opencode/skills")},
		{ToolCursor, ""},
		{ToolAider, ""},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			got := getCLISkillDir(tt.tool)
			if got != tt.want {
				t.Errorf("getCLISkillDir(%q) = %q, want %q", tt.tool, got, tt.want)
			}
		})
	}
}

func TestInjectWorkerConfig_Minimal(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := getWaveHome
	t.Cleanup(func() { getWaveHome = origHome })
	getWaveHome = func() string { return tmpDir }

	worker := &TeamWorker{
		WorkerID: "test-worker-id",
		MemberID: "test-member-id",
		Name:     "test-worker-1",
	}

	member := &TeamMember{
		MemberID: "test-member-id",
		Name:     "test-member",
		Tool:     ToolClaude,
		Persona:  "You are a test assistant.",
	}

	err := InjectWorkerConfig(worker, member)
	if err != nil {
		t.Fatalf("InjectWorkerConfig failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, ".claude", "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "test assistant") {
		t.Error("persona not injected into CLAUDE.md")
	}
}

func TestInjectWorkerConfig_WithSkills(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := getWaveHome
	t.Cleanup(func() { getWaveHome = origHome })
	getWaveHome = func() string { return tmpDir }

	skillDir := filepath.Join(tmpDir, ".waveterm", teamSkillsSubDir, "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}

	worker := &TeamWorker{WorkerID: "w1", Name: "w1"}
	member := &TeamMember{
		Name:   "m1",
		Tool:   ToolClaude,
		Skills: []string{"my-skill"},
	}

	err := InjectWorkerConfig(worker, member)
	if err != nil {
		t.Fatalf("InjectWorkerConfig with skills failed: %v", err)
	}

	linkPath := filepath.Join(tmpDir, ".claude", "skills", "my-skill")
	fi, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("skill symlink not created: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink for skill")
	}
}

func TestInjectWorkerConfig_WithMCP(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := getWaveHome
	t.Cleanup(func() { getWaveHome = origHome })
	getWaveHome = func() string { return tmpDir }

	worker := &TeamWorker{WorkerID: "w1", Name: "w1"}
	member := &TeamMember{
		Name:       "m1",
		Tool:       ToolClaude,
		McpServers: []MCPConfig{{Name: "test-srv", Type: "stdio", Command: "echo"}},
	}

	err := InjectWorkerConfig(worker, member)
	if err != nil {
		t.Fatalf("InjectWorkerConfig with MCP failed: %v", err)
	}

	mcpFile := filepath.Join(tmpDir, ".claude", "mcp", "test-srv.json")
	if _, err := os.Stat(mcpFile); err != nil {
		t.Fatalf("MCP config file not created: %v", err)
	}
}

func TestInjectWorkerConfig_NoPersonaNoSkillsNoMCP(t *testing.T) {
	worker := &TeamWorker{WorkerID: "w1", Name: "w1"}
	member := &TeamMember{Name: "m1", Tool: ToolAider}

	err := InjectWorkerConfig(worker, member)
	if err != nil {
		t.Fatalf("InjectWorkerConfig with empty member should succeed: %v", err)
	}
}

func TestInjectWorkerConfig_PersonaContainsTaskProtocol(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := getWaveHome
	t.Cleanup(func() { getWaveHome = origHome })
	getWaveHome = func() string { return tmpDir }

	worker := &TeamWorker{WorkerID: "w1", Name: "w1"}
	member := &TeamMember{
		Name:    "m1",
		Tool:    ToolClaude,
		Persona: "You are a code reviewer.",
	}

	err := InjectWorkerConfig(worker, member)
	if err != nil {
		t.Fatalf("InjectWorkerConfig failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, ".claude", "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if !strings.Contains(content, "Task Completion Protocol") {
		t.Error("CLAUDE.md missing task completion protocol")
	}
	if !strings.Contains(content, "team_update_task") {
		t.Error("CLAUDE.md missing team_update_task instruction")
	}
	if !strings.Contains(content, "team_dispatch") {
		t.Error("CLAUDE.md missing team_dispatch instruction")
	}
}

func TestInjectWorkerConfig_AutoWaveTeamMCP(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := getWaveHome
	t.Cleanup(func() { getWaveHome = origHome })
	getWaveHome = func() string { return tmpDir }

	worker := &TeamWorker{WorkerID: "w1", Name: "w1"}
	member := &TeamMember{
		Name:    "m1",
		Tool:    ToolClaude,
		Persona: "You are a developer.",
	}

	err := InjectWorkerConfig(worker, member)
	if err != nil {
		t.Fatalf("InjectWorkerConfig failed: %v", err)
	}

	mcpFile := filepath.Join(tmpDir, ".claude", "mcp", "wave-team.json")
	data, err := os.ReadFile(mcpFile)
	if err != nil {
		t.Fatalf("wave-team MCP config not created: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, `"command": "wsh"`) {
		t.Errorf("wave-team MCP missing wsh command: %s", content)
	}
	if !strings.Contains(content, "team-mcp-server") {
		t.Errorf("wave-team MCP missing team-mcp-server arg: %s", content)
	}
}

func TestInjectWorkerConfig_AutoWaveTeamMCP_DoesNotMutateMember(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := getWaveHome
	t.Cleanup(func() { getWaveHome = origHome })
	getWaveHome = func() string { return tmpDir }

	worker := &TeamWorker{WorkerID: "w1", Name: "w1"}
	member := &TeamMember{
		Name:       "m1",
		Tool:       ToolClaude,
		Persona:    "You are a developer.",
		McpServers: []MCPConfig{{Name: "existing", Type: "stdio", Command: "echo"}},
	}

	originalCount := len(member.McpServers)
	_ = InjectWorkerConfig(worker, member)

	if len(member.McpServers) != originalCount {
		t.Errorf("member.McpServers was mutated: got %d items, want %d", len(member.McpServers), originalCount)
	}
}

func countOccurrences(s, substr string) int {
	count := 0
	idx := 0
	for {
		i := strings.Index(s[idx:], substr)
		if i == -1 {
			break
		}
		count++
		idx += i + len(substr)
	}
	return count
}
