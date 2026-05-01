// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package team

import (
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/wavetermdev/waveterm/pkg/wavebase"
)

//go:embed team_defaults/*.yaml
var defaultTemplatesFS embed.FS

// memberYAML represents the on-disk YAML format for a Member template.
// Separate from TeamMember to isolate YAML field names from JSON serialization.
type memberYAML struct {
	Name           string       `yaml:"name"`
	Description    string       `yaml:"description"`
	Tool           string       `yaml:"tool"`
	CustomCmd      string       `yaml:"customCmd,omitempty"`
	Model          string       `yaml:"model,omitempty"`
	Color          string       `yaml:"color,omitempty"`
	Persona        string       `yaml:"persona,omitempty"`
	PersonaPath    string       `yaml:"personaPath,omitempty"`
	Skills         []string     `yaml:"skills,omitempty"`
	McpServers     []MCPConfig  `yaml:"mcpServers,omitempty"`
	Capabilities   []string     `yaml:"capabilities,omitempty"`
	MaxConcurrency int          `yaml:"maxConcurrency,omitempty"`
	MaxRetries     int          `yaml:"maxRetries,omitempty"`
	Memory         string       `yaml:"memory,omitempty"`
}

// projectMemberYAML is a single entry in a project-level .wave/team.yaml.
// Either a full member definition or a template reference with overrides.
type projectMemberYAML struct {
	// Template name references a global template. When set, only override fields are applied.
	Template       string       `yaml:"template,omitempty"`
	Name           string       `yaml:"name,omitempty"`
	Description    string       `yaml:"description,omitempty"`
	Tool           string       `yaml:"tool,omitempty"`
	CustomCmd      string       `yaml:"customCmd,omitempty"`
	Model          string       `yaml:"model,omitempty"`
	Color          string       `yaml:"color,omitempty"`
	Persona        string       `yaml:"persona,omitempty"`
	PersonaPath    string       `yaml:"personaPath,omitempty"`
	Skills         []string     `yaml:"skills,omitempty"`
	McpServers     []MCPConfig  `yaml:"mcpServers,omitempty"`
	Capabilities   []string     `yaml:"capabilities,omitempty"`
	MaxConcurrency *int         `yaml:"maxConcurrency,omitempty"`
	MaxRetries     *int         `yaml:"maxRetries,omitempty"`
	Memory         string       `yaml:"memory,omitempty"`
}

// projectDefYAML represents a project definition in .wave/team.yaml.
type projectDefYAML struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
	Spec string `yaml:"spec,omitempty"`
}

// projectConfigYAML is the top-level structure of .wave/team.yaml.
type projectConfigYAML struct {
	Projects []projectDefYAML       `yaml:"projects,omitempty"`
	Members  []projectMemberYAML    `yaml:"members,omitempty"`
}

// ParseMemberYAML unmarshals YAML bytes into a TeamMember.
// Applies sensible defaults for zero-value fields.
func ParseMemberYAML(data []byte) (*TeamMember, error) {
	var raw memberYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal member YAML: %w", err)
	}
	return yamlToMember(raw), nil
}

// yamlToMember converts a parsed memberYAML to a TeamMember with defaults applied.
func yamlToMember(raw memberYAML) *TeamMember {
	m := &TeamMember{
		Name:           raw.Name,
		Tool:           raw.Tool,
		CustomCmd:      raw.CustomCmd,
		Description:    raw.Description,
		Persona:        raw.Persona,
		PersonaPath:    raw.PersonaPath,
		Skills:         raw.Skills,
		McpServers:     raw.McpServers,
		Capabilities:   raw.Capabilities,
		Model:          raw.Model,
		Color:          raw.Color,
		MaxConcurrency: raw.MaxConcurrency,
		MaxRetries:     raw.MaxRetries,
		Memory:         raw.Memory,
	}
	applyDefaults(m)
	return m
}

// applyDefaults fills zero-value fields with sensible defaults.
func applyDefaults(m *TeamMember) {
	if m.Tool == "" {
		m.Tool = ToolClaude
	}
	if m.MaxConcurrency <= 0 {
		m.MaxConcurrency = 3
	}
	if m.MaxRetries <= 0 {
		m.MaxRetries = 3
	}
	if m.Memory == "" {
		m.Memory = MemorySession
	}
	if m.Skills == nil {
		m.Skills = []string{}
	}
	if m.McpServers == nil {
		m.McpServers = []MCPConfig{}
	}
	if m.Capabilities == nil {
		m.Capabilities = []string{}
	}
}

// GetGlobalTemplatesDir returns the path to ~/.waveterm/team-templates/.
func GetGlobalTemplatesDir() string {
	return filepath.Join(getWaveHome(), ".waveterm", teamTemplatesSubDir)
}

// LoadGlobalTemplates reads all *.yaml files from ~/.waveterm/team-templates/
// and parses each as a TeamMember. Returns an empty slice if the directory
// doesn't exist (not an error — user may rely on defaults or project config).
func LoadGlobalTemplates() ([]TeamMember, error) {
	dir := GetGlobalTemplatesDir()

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read global templates dir %q: %w", dir, err)
	}

	var members []TeamMember
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !isYAMLFile(entry.Name()) {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			log.Printf("warning: skip global template %q: %v", entry.Name(), err)
			continue
		}

		m, err := ParseMemberYAML(data)
		if err != nil {
			log.Printf("warning: skip global template %q: %v", entry.Name(), err)
			continue
		}
		memberFilenameToName(m, entry.Name())
		members = append(members, *m)
	}

	return members, nil
}

// LoadProjectConfig reads .wave/team.yaml from the given project directory.
// Returns project-defined members (already resolved against global templates).
// Returns an empty slice if the file doesn't exist (not an error).
func LoadProjectConfig(projectDir string) ([]TeamMember, error) {
	configPath := filepath.Join(projectDir, ".wave", "team.yaml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read project config %q: %w", configPath, err)
	}

	var cfg projectConfigYAML
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal project config %q: %w", configPath, err)
	}

	if len(cfg.Members) == 0 {
		return nil, nil
	}

	// Load global templates for template references.
	globalTemplates, err := LoadGlobalTemplates()
	if err != nil {
		log.Printf("warning: failed to load global templates for project config: %v", err)
	}

	return resolveProjectMembers(cfg.Members, globalTemplates)
}

// LoadProjectsFromConfig reads .wave/team.yaml from the given project directory.
// Returns project definitions found in the config.
// Returns an empty slice if the file doesn't exist or has no projects (not an error).
func LoadProjectsFromConfig(projectDir string) ([]TeamProject, error) {
	configPath := filepath.Join(projectDir, ".wave", "team.yaml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read project config %q: %w", configPath, err)
	}

	var cfg projectConfigYAML
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal project config %q: %w", configPath, err)
	}

	if len(cfg.Projects) == 0 {
		return nil, nil
	}

	var projects []TeamProject
	for _, p := range cfg.Projects {
		if p.Name == "" {
			log.Printf("warning: project definition has no name, skipping")
			continue
		}
		if p.Path == "" {
			log.Printf("warning: project %q has no path, skipping", p.Name)
			continue
		}
		projects = append(projects, TeamProject{
			Name: p.Name,
			Path: p.Path,
			Spec: p.Spec,
		})
	}

	return projects, nil
}

// GetEffectiveConfig merges project-level and global-level members.
// Project-level members take priority over global ones with the same name.
func GetEffectiveConfig(projectDir string) ([]TeamMember, error) {
	projectMembers, err := LoadProjectConfig(projectDir)
	if err != nil {
		return nil, err
	}

	globalMembers, err := LoadGlobalTemplates()
	if err != nil {
		return nil, err
	}

	if len(projectMembers) == 0 {
		return globalMembers, nil
	}

	// Build a map of project members by name for deduplication.
	projectMap := make(map[string]TeamMember, len(projectMembers))
	for _, m := range projectMembers {
		projectMap[m.Name] = m
	}

	var result []TeamMember
	// Global members first, skip if overridden by project.
	for _, m := range globalMembers {
		if _, ok := projectMap[m.Name]; !ok {
			result = append(result, m)
		}
	}
	// Project members last (higher priority).
	result = append(result, projectMembers...)

	return result, nil
}

// LoadDefaultTemplates returns the 4 built-in default member templates.
// These are embedded in the binary via embed.FS.
func LoadDefaultTemplates() ([]TeamMember, error) {
	entries, err := defaultTemplatesFS.ReadDir("team_defaults")
	if err != nil {
		return nil, fmt.Errorf("read embedded default templates: %w", err)
	}

	var members []TeamMember
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		data, err := defaultTemplatesFS.ReadFile(filepath.Join("team_defaults", entry.Name()))
		if err != nil {
			log.Printf("warning: skip embedded template %q: %v", entry.Name(), err)
			continue
		}

		m, err := ParseMemberYAML(data)
		if err != nil {
			log.Printf("warning: skip embedded template %q: %v", entry.Name(), err)
			continue
		}
		memberFilenameToName(m, entry.Name())
		members = append(members, *m)
	}

	return members, nil
}

// resolveProjectMembers converts project-level member definitions into TeamMembers.
// For template references, the global template is loaded and overrides are merged on top.
// For full definitions, they are parsed directly.
func resolveProjectMembers(raw []projectMemberYAML, globalTemplates []TeamMember) ([]TeamMember, error) {
	globalMap := make(map[string]*TeamMember, len(globalTemplates))
	for i := range globalTemplates {
		gm := &globalTemplates[i]
		globalMap[gm.Name] = gm
	}

	var members []TeamMember
	for _, rawMember := range raw {
		if rawMember.Template != "" {
			base, ok := globalMap[rawMember.Template]
			if !ok {
				log.Printf("warning: project member references unknown template %q, skipping", rawMember.Template)
				continue
			}
			m := cloneMember(base)
			applyProjectOverrides(m, rawMember)
			members = append(members, *m)
		} else {
			m := projectYAMLToMember(rawMember)
			if m.Name == "" {
				log.Printf("warning: project member has no name and no template, skipping")
				continue
			}
			members = append(members, *m)
		}
	}

	return members, nil
}

// applyProjectOverrides merges non-zero fields from a projectMemberYAML onto a TeamMember.
func applyProjectOverrides(m *TeamMember, raw projectMemberYAML) {
	if raw.Name != "" {
		m.Name = raw.Name
	}
	if raw.Tool != "" {
		m.Tool = raw.Tool
	}
	if raw.CustomCmd != "" {
		m.CustomCmd = raw.CustomCmd
	}
	if raw.Description != "" {
		m.Description = raw.Description
	}
	if raw.Persona != "" {
		m.Persona = raw.Persona
	}
	if raw.PersonaPath != "" {
		m.PersonaPath = raw.PersonaPath
	}
	if raw.Model != "" {
		m.Model = raw.Model
	}
	if raw.Color != "" {
		m.Color = raw.Color
	}
	if raw.Memory != "" {
		m.Memory = raw.Memory
	}
	if len(raw.Skills) > 0 {
		m.Skills = raw.Skills
	}
	if len(raw.McpServers) > 0 {
		m.McpServers = raw.McpServers
	}
	if len(raw.Capabilities) > 0 {
		m.Capabilities = raw.Capabilities
	}
	if raw.MaxConcurrency != nil {
		m.MaxConcurrency = *raw.MaxConcurrency
	}
	if raw.MaxRetries != nil {
		m.MaxRetries = *raw.MaxRetries
	}
}

// projectYAMLToMember converts a full projectMemberYAML (no template reference) to TeamMember.
func projectYAMLToMember(raw projectMemberYAML) *TeamMember {
	m := &TeamMember{
		Name:           raw.Name,
		Tool:           raw.Tool,
		CustomCmd:      raw.CustomCmd,
		Description:    raw.Description,
		Persona:        raw.Persona,
		PersonaPath:    raw.PersonaPath,
		Skills:         raw.Skills,
		McpServers:     raw.McpServers,
		Capabilities:   raw.Capabilities,
		Model:          raw.Model,
		Color:          raw.Color,
		MaxConcurrency: derefInt(raw.MaxConcurrency, 0),
		MaxRetries:     derefInt(raw.MaxRetries, 0),
		Memory:         raw.Memory,
	}
	applyDefaults(m)
	return m
}

// cloneMember creates a shallow copy of a TeamMember.
func cloneMember(m *TeamMember) *TeamMember {
	clone := *m
	return &clone
}

// memberFilenameToName sets the member name from the YAML filename if the name is empty.
// Strips the .yaml/.yml extension from the filename.
func memberFilenameToName(m *TeamMember, filename string) {
	if m.Name != "" {
		return
	}
	name := filename
	ext := filepath.Ext(name)
	if ext == ".yaml" || ext == ".yml" {
		name = name[:len(name)-len(ext)]
	}
	m.Name = name
}

func isYAMLFile(name string) bool {
	ext := filepath.Ext(name)
	return ext == ".yaml" || ext == ".yml"
}

func derefInt(p *int, fallback int) int {
	if p == nil {
		return fallback
	}
	return *p
}

// getWaveDataDir returns ~/.waveterm. Extracted for testability.
var getWaveDataDir = func() string {
	return filepath.Join(wavebase.GetHomeDir(), ".waveterm")
}
