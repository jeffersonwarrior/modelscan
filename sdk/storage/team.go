package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Team represents a team in the database
type Team struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Config      string                 `json:"config"`
	Metadata    map[string]interface{} `json:"metadata"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// TeamMember represents a team member relationship
type TeamMember struct {
	TeamID    string    `json:"team_id"`
	AgentID   string    `json:"agent_id"`
	Role      string    `json:"role"`
	JoinedAt  time.Time `json:"joined_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TeamRepository handles team database operations
type TeamRepository struct {
	db *sql.DB
}

// NewTeamRepository creates a new team repository
func NewTeamRepository(db *sql.DB) *TeamRepository {
	return &TeamRepository{db: db}
}

// Create creates a new team
func (r *TeamRepository) Create(ctx context.Context, team *Team) error {
	metadataJSON, _ := json.Marshal(team.Metadata)
	
	query := `
		INSERT INTO teams (id, name, description, config, metadata)
		VALUES (?, ?, ?, ?, ?)
	`
	
	_, err := r.db.ExecContext(ctx, query,
		team.ID, team.Name, team.Description, team.Config, metadataJSON)
	if err != nil {
		return fmt.Errorf("failed to create team: %w", err)
	}
	
	return nil
}

// Get retrieves a team by ID
func (r *TeamRepository) Get(ctx context.Context, id string) (*Team, error) {
	query := `
		SELECT id, name, description, config, metadata, created_at, updated_at
		FROM teams WHERE id = ?
	`
	
	team := &Team{}
	var metadataJSON []byte
	
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&team.ID, &team.Name, &team.Description, &team.Config,
		&metadataJSON, &team.CreatedAt, &team.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("team not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get team: %w", err)
	}
	
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &team.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}
	
	return team, nil
}

// Update updates a team
func (r *TeamRepository) Update(ctx context.Context, team *Team) error {
	metadataJSON, _ := json.Marshal(team.Metadata)
	
	query := `
		UPDATE teams 
		SET name = ?, description = ?, config = ?, metadata = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`
	
	result, err := r.db.ExecContext(ctx, query,
		team.Name, team.Description, team.Config, metadataJSON, team.ID)
	if err != nil {
		return fmt.Errorf("failed to update team: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("team not found: %s", team.ID)
	}
	
	return nil
}

// Delete deletes a team
func (r *TeamRepository) Delete(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()
	
	// Delete team members first
	_, err = tx.ExecContext(ctx, "DELETE FROM team_members WHERE team_id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete team members: %w", err)
	}
	
	// Delete team
	_, err = tx.ExecContext(ctx, "DELETE FROM teams WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete team: %w", err)
	}
	
	return tx.Commit()
}

// List retrieves all teams
func (r *TeamRepository) List(ctx context.Context, limit, offset int) ([]*Team, error) {
	query := `
		SELECT id, name, description, config, metadata, created_at, updated_at
		FROM teams
		ORDER BY name ASC
		LIMIT ? OFFSET ?
	`
	
	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list teams: %w", err)
	}
	defer rows.Close()
	
	var teams []*Team
	for rows.Next() {
		team := &Team{}
		var metadataJSON []byte
		
		err := rows.Scan(
			&team.ID, &team.Name, &team.Description, &team.Config,
			&metadataJSON, &team.CreatedAt, &team.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan team: %w", err)
		}
		
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &team.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}
		
		teams = append(teams, team)
	}
	
	return teams, nil
}

// AddMember adds an agent to a team
func (r *TeamRepository) AddMember(ctx context.Context, teamID, agentID, role string) error {
	query := `
		INSERT INTO team_members (team_id, agent_id, role)
		VALUES (?, ?, ?)
		ON CONFLICT(team_id, agent_id) DO UPDATE SET
		role = excluded.role,
		updated_at = CURRENT_TIMESTAMP
	`
	
	_, err := r.db.ExecContext(ctx, query, teamID, agentID, role)
	if err != nil {
		return fmt.Errorf("failed to add team member: %w", err)
	}
	
	return nil
}

// RemoveMember removes an agent from a team
func (r *TeamRepository) RemoveMember(ctx context.Context, teamID, agentID string) error {
	query := `DELETE FROM team_members WHERE team_id = ? AND agent_id = ?`
	
	result, err := r.db.ExecContext(ctx, query, teamID, agentID)
	if err != nil {
		return fmt.Errorf("failed to remove team member: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("team member not found: team_id=%s, agent_id=%s", teamID, agentID)
	}
	
	return nil
}

// GetMembers retrieves all members of a team
func (r *TeamRepository) GetMembers(ctx context.Context, teamID string) ([]*TeamMember, error) {
	query := `
		SELECT team_id, agent_id, role, joined_at, updated_at
		FROM team_members
		WHERE team_id = ?
		ORDER BY joined_at ASC
	`
	
	rows, err := r.db.QueryContext(ctx, query, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to get team members: %w", err)
	}
	defer rows.Close()
	
	var members []*TeamMember
	for rows.Next() {
		member := &TeamMember{}
		
		err := rows.Scan(
			&member.TeamID, &member.AgentID, &member.Role,
			&member.JoinedAt, &member.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan team member: %w", err)
		}
		
		members = append(members, member)
	}
	
	return members, nil
}

// GetAgentTeams retrieves all teams an agent belongs to
func (r *TeamRepository) GetAgentTeams(ctx context.Context, agentID string) ([]*Team, error) {
	query := `
		SELECT t.id, t.name, t.description, t.config, t.metadata, t.created_at, t.updated_at
		FROM teams t
		JOIN team_members tm ON t.id = tm.team_id
		WHERE tm.agent_id = ?
		ORDER BY t.name ASC
	`
	
	rows, err := r.db.QueryContext(ctx, query, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent teams: %w", err)
	}
	defer rows.Close()
	
	var teams []*Team
	for rows.Next() {
		team := &Team{}
		var metadataJSON []byte
		
		err := rows.Scan(
			&team.ID, &team.Name, &team.Description, &team.Config,
			&metadataJSON, &team.CreatedAt, &team.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan team: %w", err)
		}
		
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &team.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}
		
		teams = append(teams, team)
	}
	
	return teams, nil
}

// UpdateMemberRole updates an agent's role in a team
func (r *TeamRepository) UpdateMemberRole(ctx context.Context, teamID, agentID, role string) error {
	query := `
		UPDATE team_members
		SET role = ?, updated_at = CURRENT_TIMESTAMP
		WHERE team_id = ? AND agent_id = ?
	`
	
	result, err := r.db.ExecContext(ctx, query, role, teamID, agentID)
	if err != nil {
		return fmt.Errorf("failed to update member role: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("team member not found: team_id=%s, agent_id=%s", teamID, agentID)
	}
	
	return nil
}