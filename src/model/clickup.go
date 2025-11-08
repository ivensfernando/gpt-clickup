package model

import "time"

// WorkspaceClickUp rappresenta un workspace sincronizzato con il database locale.
type WorkspaceClickUp struct {
	ID        string         `gorm:"primaryKey" json:"id"`
	Name      string         `gorm:"size:255" json:"name"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	Spaces    []SpaceClickUp `json:"spaces"`
}

// SpaceClickUp rappresenta uno spazio del ClickUp.
type SpaceClickUp struct {
	ID          string          `gorm:"primaryKey" json:"id"`
	Name        string          `gorm:"size:255" json:"name"`
	WorkspaceID string          `gorm:"index" json:"workspace_id"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	Lists       []ListClickUp   `json:"lists"`
	Folders     []FolderClickUp `json:"folders"`
}

// FolderClickUp rappresenta un folder del ClickUp.
type FolderClickUp struct {
	ID        string        `gorm:"primaryKey" json:"id"`
	Name      string        `gorm:"size:255" json:"name"`
	SpaceID   string        `gorm:"index" json:"space_id"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	Lists     []ListClickUp `json:"lists"`
}

// ListClickUp rappresenta una lista del ClickUp.
type ListClickUp struct {
	ID        string        `gorm:"primaryKey" json:"id"`
	Name      string        `gorm:"size:255" json:"name"`
	SpaceID   string        `gorm:"index" json:"space_id"`
	FolderID  *string       `gorm:"index" json:"folder_id"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	Tasks     []TaskClickUp `json:"tasks"`
}

// TaskClickUp rappresenta un task o subtask del ClickUp.
type TaskClickUp struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:255" json:"name"`
	ListID    string    `gorm:"index" json:"list_id"`
	ParentID  *string   `gorm:"index" json:"parent"`
	Status    string    `gorm:"size:100" json:"status"`
	Priority  int       `json:"priority"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName definisce il nome della tabella per WorkspaceClickUp.
func (WorkspaceClickUp) TableName() string { return "clickup_workspaces" }

// TableName definisce il nome della tabella per SpaceClickUp.
func (SpaceClickUp) TableName() string { return "clickup_spaces" }

// TableName definisce il nome della tabella per FolderClickUp.
func (FolderClickUp) TableName() string { return "clickup_folders" }

// TableName definisce il nome della tabella per ListClickUp.
func (ListClickUp) TableName() string { return "clickup_lists" }

// TableName definisce il nome della tabella per TaskClickUp.
func (TaskClickUp) TableName() string { return "clickup_tasks" }
