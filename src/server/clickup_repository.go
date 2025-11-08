package server

import (
	"github.com/ivensfernando/gpt-clickup/src/db"
	"github.com/ivensfernando/gpt-clickup/src/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GormClickUpRepository implementa ClickUpRepository utilizzando GORM.
type GormClickUpRepository struct {
	db *gorm.DB
}

// NewGormClickUpRepository crea un repository basato su GORM.
func NewGormClickUpRepository(database *gorm.DB) *GormClickUpRepository {
	return &GormClickUpRepository{db: database}
}

func (r *GormClickUpRepository) GetWorkspaces() ([]model.WorkspaceClickUp, error) {
	var workspaces []model.WorkspaceClickUp
	err := r.db.Preload("Spaces").Find(&workspaces).Error
	return workspaces, err
}

func (r *GormClickUpRepository) SaveWorkspaces(workspaces []model.WorkspaceClickUp) error {
	if len(workspaces) == 0 {
		return nil
	}
	return r.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&workspaces).Error
}

func (r *GormClickUpRepository) GetSpaces(workspaceID string) ([]model.SpaceClickUp, error) {
	var spaces []model.SpaceClickUp
	err := r.db.Where("workspace_id = ?", workspaceID).Find(&spaces).Error
	return spaces, err
}

func (r *GormClickUpRepository) SaveSpaces(spaces []model.SpaceClickUp) error {
	if len(spaces) == 0 {
		return nil
	}
	return r.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&spaces).Error
}

func (r *GormClickUpRepository) GetLists(spaceID string) ([]model.ListClickUp, error) {
	var lists []model.ListClickUp
	err := r.db.Where("space_id = ? AND folder_id IS NULL", spaceID).Find(&lists).Error
	return lists, err
}

func (r *GormClickUpRepository) SaveLists(lists []model.ListClickUp) error {
	if len(lists) == 0 {
		return nil
	}
	return r.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&lists).Error
}

func (r *GormClickUpRepository) GetFolders(spaceID string) ([]model.FolderClickUp, error) {
	var folders []model.FolderClickUp
	err := r.db.Preload("Lists").Where("space_id = ?", spaceID).Find(&folders).Error
	return folders, err
}

func (r *GormClickUpRepository) SaveFolders(folders []model.FolderClickUp) error {
	if len(folders) == 0 {
		return nil
	}
	lists := make([]model.ListClickUp, 0)
	for _, folder := range folders {
		if len(folder.Lists) > 0 {
			lists = append(lists, folder.Lists...)
		}
	}
	if err := r.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&folders).Error; err != nil {
		return err
	}
	return r.SaveLists(lists)
}

func (r *GormClickUpRepository) GetTasks(listID string) ([]model.TaskClickUp, error) {
	var tasks []model.TaskClickUp
	err := r.db.Where("list_id = ?", listID).Find(&tasks).Error
	return tasks, err
}

func (r *GormClickUpRepository) SaveTasks(tasks []model.TaskClickUp) error {
	if len(tasks) == 0 {
		return nil
	}
	return r.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&tasks).Error
}

// DefaultRepository restituisce il repository condiviso basato sul database globale.
func DefaultRepository() ClickUpRepository {
	return NewGormClickUpRepository(db.DB)
}
