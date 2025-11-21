package model

import "time"

// User rappresenta il proprietario delle credenziali ClickUp utilizzate dal sistema.
type User struct {
	ID            uint               `gorm:"primaryKey" json:"id"`
	Username      string             `gorm:"uniqueIndex;not null" json:"username"`
	Password      string             `json:"-"`
	Email         string             `gorm:"size:255" json:"email"`
	FirstName     string             `gorm:"size:100" json:"first_name"`
	LastName      string             `gorm:"size:100" json:"last_name"`
	Bio           string             `gorm:"size:1024" json:"bio"`
	AvatarURL     string             `gorm:"size:512" json:"avatar_url"`
	ClickUpUserID string             `gorm:"unique" json:"clickup_user_id"`
	ApiKey        string             `gorm:"-" json:"-"`
	LastLogin     time.Time          `json:"last_login"`
	LastSeen      time.Time          `json:"last_seen"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
	Workspaces    []WorkspaceClickUp `gorm:"foreignKey:UserID" json:"workspaces"`
}
