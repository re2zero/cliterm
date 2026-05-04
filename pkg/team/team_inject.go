// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package team

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/wavetermdev/waveterm/pkg/wavebase"
)

const (
	teamSkillsSubDir       = "team-skills"
	teamTemplatesSubDir    = "team-templates"
	teamPersonaMarker      = "<!-- team:persona -->"
	taskCompletionProtocol = `

## Task Completion Protocol

You are running as a team worker. When you complete your assigned task:
- Use the team_update_task MCP tool with status="done" and result="brief summary"
- If you encounter an error, use team_update_task with status="failed" and error="description"
- For long tasks, report progress with team_update_task progress=N (0-100)
- To communicate with another worker, use team_dispatch(target="worker_name", message="your message")
- To check team status, use team_get_status
`
)

// cliSkillDirs maps tool type to its native skills directory (relative to home).
var cliSkillDirs = map[string]string{
	ToolClaude:   ".claude/skills",
	ToolOpenCode: ".config/opencode/skills",
}

// InjectWorkerConfig injects Member configuration into the Worker's CLI tool.
// It handles persona loading, skills symlinking, and MCP config injection.
func InjectWorkerConfig(worker *TeamWorker, member *TeamMember) error {
	configDir := filepath.Join(getWaveHome(), teamTemplatesSubDir)

	persona, err := loadPersona(member, configDir)
	if err != nil {
		return fmt.Errorf("loadPersona for worker %s: %w", worker.WorkerID, err)
	}
	if persona != "" {
		persona = persona + taskCompletionProtocol
	} else {
		persona = strings.TrimPrefix(taskCompletionProtocol, "\n")
	}
	if err := injectPersona(persona, member); err != nil {
		return fmt.Errorf("injectPersona for worker %s: %w", worker.WorkerID, err)
	}

	if len(member.Skills) > 0 {
		if err := linkSkills(member.Skills, member.Tool, ""); err != nil {
			return fmt.Errorf("linkSkills for worker %s: %w", worker.WorkerID, err)
		}
	}

	if len(member.McpServers) > 0 {
		if err := injectMCP(member); err != nil {
			return fmt.Errorf("injectMCP for worker %s: %w", worker.WorkerID, err)
		}
	}

	waveTeamMCP := MCPConfig{
		Name:    "wave-team",
		Type:    "stdio",
		Command: getWshPath(),
		Args:    []string{"mcp", "--tools=team"},
	}
	member.McpServers = append(member.McpServers, waveTeamMCP)
	if err := injectMCP(member); err != nil {
		return fmt.Errorf("injectMCP wave-team for worker %s: %w", worker.WorkerID, err)
	}
	member.McpServers = member.McpServers[:len(member.McpServers)-1]

	return nil
}

// loadPersona loads persona content, resolving personaPath file references.
// personaPath takes priority over inline persona. Falls back to inline on read failure.
func loadPersona(member *TeamMember, configDir string) (string, error) {
	if member.PersonaPath == "" {
		return member.Persona, nil
	}

	path := resolvePersonaPath(member.PersonaPath, configDir)
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("warning: failed to read personaPath %q: %v, falling back to inline persona", path, err)
		if member.Persona != "" {
			return member.Persona, nil
		}
		return "", fmt.Errorf("read persona file %q: %w", path, err)
	}

	return string(data), nil
}

// resolvePersonaPath resolves a personaPath relative to configDir.
// Absolute paths (starting with /) are returned as-is.
// All other paths (including ./ prefixed) resolve relative to configDir.
func resolvePersonaPath(rawPath, configDir string) string {
	if filepath.IsAbs(rawPath) {
		return rawPath
	}
	return filepath.Join(configDir, rawPath)
}

func injectPersona(persona string, member *TeamMember) error {
	switch member.Tool {
	case ToolClaude:
		return injectPersonaClaude(persona)
	case ToolOpenCode:
		return injectPersonaOpenCode(persona)
	default:
		return nil
	}
}

func injectPersonaClaude(persona string) error {
	claudeDir := filepath.Join(getWaveHome(), ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return fmt.Errorf("create claude config dir: %w", err)
	}

	claudeMdPath := filepath.Join(claudeDir, "CLAUDE.md")
	existing, err := os.ReadFile(claudeMdPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read CLAUDE.md: %w", err)
	}

	cleaned := removeMarkerSection(string(existing), teamPersonaMarker)
	section := "\n" + teamPersonaMarker + "\n" + persona + "\n" + teamPersonaMarker + "\n"
	result := strings.TrimRight(cleaned, "\n") + section

	return os.WriteFile(claudeMdPath, []byte(result), 0644)
}

func injectPersonaOpenCode(persona string) error {
	// Write persona as AGENTS.md content in current working directory.
	// The worker terminal's cwd is typically the project root.
	return os.WriteFile("AGENTS.md", []byte(persona), 0644)
}

// removeMarkerSection strips content between two identical markers (inclusive).
func removeMarkerSection(content, marker string) string {
	startTag := marker + "\n"
	endTag := "\n" + marker

	startIdx := strings.Index(content, startTag)
	if startIdx == -1 {
		return content
	}

	afterStart := content[startIdx+len(startTag):]
	endIdx := strings.Index(afterStart, endTag)
	if endIdx == -1 {
		return content
	}

	return content[:startIdx] + afterStart[endIdx+len(endTag):]
}

// linkSkills creates symlinks from CLI-native skills dirs to team skill sources.
// Priority: project-level > team global > skip (already exists natively).
// Idempotent: skips if symlink already points to the correct target.
func linkSkills(skills []string, tool string, projectDir string) error {
	cliDir := getCLISkillDir(tool)
	if cliDir == "" {
		return nil
	}

	teamGlobalDir := getTeamSkillsDir()
	var projectSkillDir string
	if projectDir != "" {
		projectSkillDir = filepath.Join(projectDir, ".wave", "skills")
	}

	for _, skill := range skills {
		if err := linkSingleSkill(skill, cliDir, teamGlobalDir, projectSkillDir); err != nil {
			return fmt.Errorf("link skill %q: %w", skill, err)
		}
	}

	return nil
}

// unlinkSkills removes skill symlinks created by linkSkills (called on Worker recycle).
func unlinkSkills(skills []string, tool string) error {
	cliDir := getCLISkillDir(tool)
	if cliDir == "" {
		return nil
	}

	for _, skill := range skills {
		linkPath := filepath.Join(cliDir, skill)

		// Only remove if it's a symlink — never delete real skill directories.
		fi, err := os.Lstat(linkPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("stat skill link %q: %w", linkPath, err)
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			continue
		}

		if err := os.Remove(linkPath); err != nil {
			return fmt.Errorf("remove skill link %q: %w", linkPath, err)
		}
	}

	return nil
}

func linkSingleSkill(name, cliDir, teamGlobalDir, projectSkillDir string) error {
	linkPath := filepath.Join(cliDir, name)

	// Skip if a native (non-symlink) directory already exists.
	fi, err := os.Lstat(linkPath)
	if err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			// Symlink exists — skip if it points to a team-skills source.
			target, _ := os.Readlink(linkPath)
			if strings.HasPrefix(target, teamGlobalDir) {
				return nil
			}
		}
		// Real directory or foreign symlink — skip (user-managed).
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}

	// Resolve source: project-level > team global.
	source := resolveSkillSource(name, projectSkillDir, teamGlobalDir)
	if source == "" {
		log.Printf("warning: skill %q not found in project or global skill dirs, skipping", name)
		return nil
	}

	if err := os.MkdirAll(cliDir, 0755); err != nil {
		return fmt.Errorf("create cli skills dir: %w", err)
	}

	return os.Symlink(source, linkPath)
}

func resolveSkillSource(name, projectSkillDir, teamGlobalDir string) string {
	if projectSkillDir != "" {
		candidate := filepath.Join(projectSkillDir, name)
		if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
			return candidate
		}
	}

	candidate := filepath.Join(teamGlobalDir, name)
	if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
		return candidate
	}

	return ""
}

// injectMCP creates MCP server config files for the Member's tool.
func getWshPath() string {
	binDir := wavebase.GetWaveAppBinPath()

	if wavebase.IsDevMode() {
		wshPath := filepath.Join(binDir, "wsh")
		if _, err := os.Stat(wshPath); err == nil {
			return wshPath
		}
	}

	// Taskfile builds wsh as wsh-<version>-<os>.x64 (amd64 is renamed to x64)
	archName := runtime.GOARCH
	if archName == "amd64" {
		archName = "x64"
	}
	platform := runtime.GOOS + "." + archName
	globPattern := filepath.Join(binDir, fmt.Sprintf("wsh-%s-*", wavebase.WaveVersion))

	matches, _ := filepath.Glob(globPattern)
	for _, match := range matches {
		if strings.Contains(filepath.Base(match), platform) {
			return match
		}
	}
	fallbackPattern := filepath.Join(binDir, fmt.Sprintf("wsh-%s", wavebase.WaveVersion))
	if matches, _ := filepath.Glob(fallbackPattern); len(matches) > 0 {
		return matches[0]
	}
	return "wsh"
}

func injectMCP(member *TeamMember) error {
	switch member.Tool {
	case ToolClaude:
		return injectMCPClaude(member.McpServers)
	case ToolOpenCode:
		return injectMCPOpenCode(member.McpServers)
	default:
		log.Printf("warning: MCP injection not supported for tool %q, skipping", member.Tool)
		return nil
	}
}

func injectMCPClaude(servers []MCPConfig) error {
	// Claude Code reads MCP servers from ~/.claude.json under "mcpServers" key.
	claudeJsonPath := filepath.Join(getWaveHome(), ".claude.json")

	existing := make(map[string]interface{})
	if data, err := os.ReadFile(claudeJsonPath); err == nil {
		json.Unmarshal(data, &existing)
	}

	mcpServers, _ := existing["mcpServers"].(map[string]interface{})
	if mcpServers == nil {
		mcpServers = make(map[string]interface{})
	}

	for _, srv := range servers {
		entry := buildClaudeMCPEntry(srv)
		if entry != nil {
			mcpServers[srv.Name] = entry
		}
	}

	existing["mcpServers"] = mcpServers
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal ~/.claude.json: %w", err)
	}

	return os.WriteFile(claudeJsonPath, append(data, '\n'), 0644)
}

func buildClaudeMCPEntry(srv MCPConfig) map[string]interface{} {
	switch srv.Type {
	case "stdio":
		entry := map[string]interface{}{
			"command": srv.Command,
			"args":    srv.Args,
		}
		if len(srv.Env) > 0 {
			entry["env"] = srv.Env
		}
		return entry
	case "http":
		entry := map[string]interface{}{
			"url": srv.URL,
		}
		if len(srv.Headers) > 0 {
			entry["headers"] = srv.Headers
		}
		return entry
	default:
		log.Printf("warning: unknown MCP type %q for server %q, skipping", srv.Type, srv.Name)
		return nil
	}
}

func injectMCPOpenCode(servers []MCPConfig) error {
	configDir := filepath.Join(getWaveHome(), ".config", "opencode")
	configPath := filepath.Join(configDir, "opencode.json")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("create opencode config dir: %w", err)
	}

	existing := make(map[string]interface{})
	if data, err := os.ReadFile(configPath); err == nil {
		json.Unmarshal(data, &existing)
	}

	mcpMap, _ := existing["mcp"].(map[string]interface{})
	if mcpMap == nil {
		mcpMap = make(map[string]interface{})
	}

	for _, srv := range servers {
		entry := buildOpenCodeMCPEntry(srv)
		if entry != nil {
			mcpMap[srv.Name] = entry
		}
	}

	existing["mcp"] = mcpMap
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal opencode.json: %w", err)
	}

	return os.WriteFile(configPath, append(data, '\n'), 0644)
}

func buildOpenCodeMCPEntry(srv MCPConfig) map[string]interface{} {
	switch srv.Type {
	case "stdio":
		cmd := []string{srv.Command}
		cmd = append(cmd, srv.Args...)
		entry := map[string]interface{}{
			"type":    "local",
			"command": cmd,
		}
		if len(srv.Env) > 0 {
			entry["environment"] = srv.Env
		}
		return entry
	case "http":
		entry := map[string]interface{}{
			"type": "remote",
			"url":  srv.URL,
		}
		if len(srv.Headers) > 0 {
			entry["headers"] = srv.Headers
		}
		return entry
	default:
		log.Printf("warning: unknown MCP type %q for server %q, skipping", srv.Type, srv.Name)
		return nil
	}
}

// getWaveHome returns the user's home directory. Overridable for testing.
var getWaveHome = func() string {
	return wavebase.GetHomeDir()
}

// InjectDefaultWorkerConfig injects minimal worker config when member is not available.
// It writes the task completion protocol persona using claude as default tool.
func InjectDefaultWorkerConfig(worker *TeamWorker) error {
	persona := strings.TrimPrefix(taskCompletionProtocol, "\n")
	defaultMember := &TeamMember{Tool: ToolClaude}
	if err := injectPersona(persona, defaultMember); err != nil {
		return fmt.Errorf("injectPersona: %w", err)
	}
	waveTeamMCP := MCPConfig{
		Name:    "wave-team",
		Type:    "stdio",
		Command: getWshPath(),
		Args:    []string{"mcp", "--tools=team"},
	}
	defaultMember.McpServers = []MCPConfig{waveTeamMCP}
	if err := injectMCP(defaultMember); err != nil {
		return fmt.Errorf("injectMCP: %w", err)
	}
	return nil
}

func getTeamSkillsDir() string {
	return filepath.Join(getWaveHome(), ".waveterm", teamSkillsSubDir)
}

func getCLISkillDir(tool string) string {
	relDir, ok := cliSkillDirs[tool]
	if !ok {
		return ""
	}
	return filepath.Join(getWaveHome(), relDir)
}

func sanitizeFileName(name string) string {
	safe := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, name)
	if safe == "" {
		return "unnamed"
	}
	return safe
}
