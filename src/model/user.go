package model

import "time"

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
	LastLogin     time.Time `json:"last_login"`
	LastSeen      time.Time `json:"last_seen"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
