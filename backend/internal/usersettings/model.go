// Package usersettings implements preferences owned by an authenticated user.
package usersettings

import "time"

// Settings contains the preferences that affect product and agent behavior.
type Settings struct {
	UserID              string     `db:"user_id" json:"-"`
	Timezone            *string    `db:"timezone" json:"timezone"`
	AgentModel          string     `db:"agent_model" json:"agentModel"`
	AgentThinkingEffort string     `db:"agent_thinking_effort" json:"agentThinkingEffort"`
	Version             int64      `db:"version" json:"version"`
	CreatedAt           *time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt           *time.Time `db:"updated_at" json:"updatedAt"`
	LastModifiedBy      string     `db:"last_modified_by" json:"lastModifiedBy"`
}

// Update replaces the editable preferences using optimistic concurrency.
type Update struct {
	Timezone            string
	AgentModel          string
	AgentThinkingEffort string
	Version             int64
}

// View includes settings and the models the server allows the user to select.
type View struct {
	Settings                      Settings `json:"settings"`
	AvailableAgentModels          []string `json:"availableAgentModels"`
	AvailableAgentThinkingEfforts []string `json:"availableAgentThinkingEfforts"`
}
