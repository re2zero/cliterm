// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"fmt"
	"log"

	"github.com/wavetermdev/waveterm/pkg/skills"
	"github.com/wavetermdev/waveterm/pkg/wshrpc"
)

// SkillsRpcServer handles WSH RPC commands for skills
type SkillsRpcServer struct {
	store *skills.SkillDB
}

// MakeSkillsRpcServer creates a new skills RPC server
func MakeSkillsRpcServer(store *skills.SkillDB) *SkillsRpcServer {
	return &SkillsRpcServer{
		store: store,
	}
}

// WshServerImpl implements wshutil.ServerImpl interface
func (*SkillsRpcServer) WshServerImpl() {}

// SkillsListCommand lists all skills
func (s *SkillsRpcServer) SkillsListCommand(ctx context.Context, data wshrpc.CommandSkillsListData) (wshrpc.CommandSkillsListRtnData, error) {
	skillList, err := s.store.ListSkills()
	if err != nil {
		return wshrpc.CommandSkillsListRtnData{}, fmt.Errorf("failed to list skills: %w", err)
	}

	skillInfos := make([]wshrpc.SkillInfo, len(skillList))
	for i, skill := range skillList {
		skillInfos[i] = wshrpc.SkillInfo{
			ID:          skill.ID,
			Name:        skill.Name,
			Description: skill.Description,
			CreatedAt:   skill.CreatedAt,
			UpdatedAt:   skill.UpdatedAt,
		}
	}

	log.Printf("[skills] listed %d skills", len(skillInfos))
	return wshrpc.CommandSkillsListRtnData{
		Skills: skillInfos,
	}, nil
}

// SkillsRegisterCommand creates a new skill
func (s *SkillsRpcServer) SkillsRegisterCommand(ctx context.Context, data wshrpc.CommandSkillsRegisterData) (wshrpc.CommandSkillsRegisterRtnData, error) {
	skill := &skills.Skill{
		ID:          s.store.GenerateID(),
		Name:        data.Name,
		Description: data.Description,
	}

	if err := s.store.CreateSkill(skill); err != nil {
		log.Printf("[skills rpc error] failed to create skill: %v", err)
		return wshrpc.CommandSkillsRegisterRtnData{}, fmt.Errorf("failed to create skill: %w", err)
	}

	log.Printf("[skills] registered skill: %s (%s)", skill.ID, skill.Name)
	return wshrpc.CommandSkillsRegisterRtnData{
		ID: skill.ID,
	}, nil
}

// SkillsUpdateCommand updates an existing skill
func (s *SkillsRpcServer) SkillsUpdateCommand(ctx context.Context, data wshrpc.CommandSkillsUpdateData) (wshrpc.CommandSkillsUpdateRtnData, error) {
	skill, err := s.store.GetSkill(data.ID)
	if err != nil {
		log.Printf("[skills rpc error] failed to get skill for update: %v", err)
		return wshrpc.CommandSkillsUpdateRtnData{}, fmt.Errorf("failed to get skill: %w", err)
	}

	// Update fields
	skill.Name = data.Name
	skill.Description = data.Description

	if err := s.store.UpdateSkill(skill); err != nil {
		log.Printf("[skills rpc error] failed to update skill: %v", err)
		return wshrpc.CommandSkillsUpdateRtnData{}, fmt.Errorf("failed to update skill: %w", err)
	}

	log.Printf("[skills] updated skill: %s", skill.ID)
	return wshrpc.CommandSkillsUpdateRtnData{
		ID: skill.ID,
	}, nil
}

// SkillsDeleteCommand deletes a skill
func (s *SkillsRpcServer) SkillsDeleteCommand(ctx context.Context, data wshrpc.CommandSkillsDeleteData) error {
	if err := s.store.DeleteSkill(data.ID); err != nil {
		log.Printf("[skills rpc error] failed to delete skill: %v", err)
		return fmt.Errorf("failed to delete skill: %w", err)
	}

	log.Printf("[skills] deleted skill: %s", data.ID)
	return nil
}
