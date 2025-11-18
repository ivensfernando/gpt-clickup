package model

import "time"

// User represents a user in the system
type User struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	Username      string    `gorm:"uniqueIndex;not null" json:"username"`
	Password      string    `json:"-"` // Hashed
	Email         string    `gorm:"size:255" json:"email"`
	FirstName     string    `gorm:"size:100" json:"first_name"`
	LastName      string    `gorm:"size:100" json:"last_name"`
	Bio           string    `gorm:"size:1024" json:"bio"`
	AvatarURL     string    `gorm:"size:512" json:"avatar_url"`
	ClickUpUserID string    `gorm:"unique" json:"clickup_user_id"`
	ApiKey        string    `gorm:"-" json:"-"` // Sensitive data, not stored
	LastLogin     time.Time `json:"last_login"`
	LastSeen      time.Time `json:"last_seen"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Workspaces    []WorkspaceClickUp `gorm:"foreignKey:UserID" json:"workspaces"`
}

type WorkspaceClickUp struct {
	ID        string         `gorm:"primaryKey" json:"id"`
	Name      string         `gorm:"size:255" json:"name"`
	UserID    uint           `gorm:"index" json:"user_id"`
	User      *User          `gorm:"foreignKey:UserID" json:"-"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	Spaces    []SpaceClickUp `gorm:"foreignKey:WorkspaceID;references:ID" json:"spaces"`
}

type SpaceClickUp struct {
	ID          string            `gorm:"primaryKey" json:"id"`
	Name        string            `gorm:"size:255" json:"name"`
	WorkspaceID string            `gorm:"index" json:"workspace_id"`
	Workspace   *WorkspaceClickUp `gorm:"foreignKey:WorkspaceID" json:"-"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Lists       []ListClickUp     `gorm:"foreignKey:SpaceID;references:ID" json:"lists"`
	Folders     []FolderClickUp   `gorm:"foreignKey:SpaceID;references:ID" json:"folders"`
}

type FolderClickUp struct {
	ID        string        `gorm:"primaryKey" json:"id"`
	Name      string        `gorm:"size:255" json:"name"`
	SpaceID   string        `gorm:"index" json:"space_id"`
	Space     *SpaceClickUp `gorm:"foreignKey:SpaceID" json:"-"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	Lists     []ListClickUp `gorm:"foreignKey:FolderID;references:ID" json:"lists"`
}

type ListClickUp struct {
	ID        string         `gorm:"primaryKey" json:"id"`
	Name      string         `gorm:"size:255" json:"name"`
	SpaceID   string         `gorm:"index" json:"space_id"`
	Space     *SpaceClickUp  `gorm:"foreignKey:SpaceID" json:"-"`
	FolderID  *string        `gorm:"index" json:"folder_id"`
	Folder    *FolderClickUp `gorm:"foreignKey:FolderID" json:"-"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	Tasks     []TaskClickUp  `gorm:"foreignKey:ListID;references:ID" json:"tasks"`
}

type TaskClickUp struct {
	ID         string       `gorm:"primaryKey" json:"id"`
	Name       string       `gorm:"size:255" json:"name"`
	ListID     string       `gorm:"index" json:"list_id"`
	List       *ListClickUp `gorm:"foreignKey:ListID" json:"-"`
	ParentID   *string      `gorm:"index" json:"parent"`
	ParentTask *TaskClickUp `gorm:"foreignKey:ParentID" json:"-"`
	Status     string       `gorm:"size:100" json:"status"`
	Priority   int          `json:"priority"`
	StartDate  *time.Time   `json:"start_date"`
	DueDate    *time.Time   `json:"due_date"`
	CreatedAt  time.Time    `json:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at"`
}
