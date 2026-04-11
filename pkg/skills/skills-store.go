// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package skills

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/sawka/txwrap"
)

// Skill represents a skill in the database
type Skill struct {
	ID          string
	Name        string
	Description string
	CreatedAt   int64
	UpdatedAt   int64
}

// SkillDB handles database operations for skills
type SkillDB struct {
	db *sql.DB
}

// MakeSkillDB creates a new SkillDB instance
func MakeSkillDB(db *sql.DB) *SkillDB {
	return &SkillDB{
		db: db,
	}
}

// MakeSkillDBFromSqlx creates a new SkillDB instance from sqlx.DB
func MakeSkillDBFromSqlx(dbx *sqlx.DB) *SkillDB {
	return &SkillDB{
		db: dbx.DB,
	}
}

// CreateSkill creates a new skill
func (s *SkillDB) CreateSkill(skill *Skill) error {
	if skill.Name == "" {
		return fmt.Errorf("skill name is required")
	}

	tx, err := txwrap.Wrap(s.db)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UnixMilli()

	// Insert skill
	_, err = tx.Exec(
		"INSERT INTO skills (id, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		skill.ID,
		skill.Name,
		skill.Description,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("failed to insert skill: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetSkill retrieves a single skill by id
func (s *SkillDB) GetSkill(id string) (*Skill, error) {
	var skill Skill
	err := s.db.QueryRow(
		"SELECT id, name, description, created_at, updated_at FROM skills WHERE id = ?",
		id,
	).Scan(&skill.ID, &skill.Name, &skill.Description, &skill.CreatedAt, &skill.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("skill not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get skill: %w", err)
	}
	return &skill, nil
}

// ListSkills retrieves all skills
func (s *SkillDB) ListSkills() ([]*Skill, error) {
	rows, err := s.db.Query("SELECT id, name, description, created_at, updated_at FROM skills ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("failed to list skills: %w", err)
	}
	defer rows.Close()

	var skills []*Skill
	for rows.Next() {
		var skill Skill
		if err := rows.Scan(&skill.ID, &skill.Name, &skill.Description, &skill.CreatedAt, &skill.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan skill: %w", err)
		}
		skills = append(skills, &skill)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating skills: %w", err)
	}

	return skills, nil
}

// UpdateSkill updates an existing skill
func (s *SkillDB) UpdateSkill(skill *Skill) error {
	if skill.Name == "" {
		return fmt.Errorf("skill name is required")
	}

	tx, err := txwrap.Wrap(s.db)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UnixMilli()

	// Update skill
	result, err := tx.Exec(
		"UPDATE skills SET name = ?, description = ?, updated_at = ? WHERE id = ?",
		skill.Name,
		skill.Description,
		now,
		skill.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update skill: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("skill not found: %s", skill.ID)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// DeleteSkill deletes a skill by id
func (s *SkillDB) DeleteSkill(id string) error {
	result, err := s.db.Exec("DELETE FROM skills WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete skill: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("skill not found: %s", id)
	}

	return nil
}
