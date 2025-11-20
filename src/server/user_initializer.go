package server

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gpt-clickup/internal/platform/clickup"
	"gpt-clickup/src/db"
	"gpt-clickup/src/model"
)

// ensurePrimaryUser makes sure there is at least one user in the database, creating it from ClickUp if necessary.
func ensurePrimaryUser(ctx context.Context, service clickup.Service, logger *logrus.Entry) error {
	var count int64
	if err := db.DB.Model(&model.User{}).Count(&count).Error; err != nil {
		return fmt.Errorf("count existing users: %w", err)
	}

	if count > 0 {
		return nil
	}

	user, workspaces, err := service.GetCurrentUser(ctx)
	if err != nil {
		return fmt.Errorf("fetch ClickUp user: %w", err)
	}

	return db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return fmt.Errorf("create user: %w", err)
		}

		userID := new(uint)
		*userID = user.ID

		for i := range workspaces {
			workspaces[i].UserID = userID
		}

		if len(workspaces) > 0 {
			if err := tx.Create(&workspaces).Error; err != nil {
				return fmt.Errorf("create workspaces: %w", err)
			}
		}

		logger.WithFields(logrus.Fields{
			"operation":       "bootstrap_user",
			"user_id":         user.ID,
			"workspace_count": len(workspaces),
		}).Info("created primary user from ClickUp credentials")

		return nil
	})
}
