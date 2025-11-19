package server

import (
	"gpt-clickup/internal/platform/clickup"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"gpt-clickup/src/model"
)

// ClickUpHandler coordina le operazioni tra il client ClickUp e il database locale.
type ClickUpRepository interface {
	GetWorkspaces() ([]model.WorkspaceClickUp, error)
	GetWorkspaceTree() ([]model.WorkspaceClickUp, error)
	SaveWorkspaces([]model.WorkspaceClickUp) error
	GetSpaces(workspaceID string) ([]model.SpaceClickUp, error)
	SaveSpaces([]model.SpaceClickUp) error
	GetLists(spaceID string) ([]model.ListClickUp, error)
	SaveLists([]model.ListClickUp) error
	GetFolders(spaceID string) ([]model.FolderClickUp, error)
	SaveFolders([]model.FolderClickUp) error
	GetTasks(listID string) ([]model.TaskClickUp, error)
	SaveTasks([]model.TaskClickUp) error
}

type ClickUpHandler struct {
	service clickup.Service
	repo    ClickUpRepository
	logger  *logrus.Entry
}

// NewClickUpHandler crea un nuovo gestore per le rotte ClickUp.
func NewClickUpHandler(service clickup.Service, repo ClickUpRepository, logger *logrus.Entry) *ClickUpHandler {
	return &ClickUpHandler{service: service, repo: repo, logger: logger}
}

// RegisterRoutes registra tutte le rotte HTTP dedicate al ClickUp.
func (h *ClickUpHandler) RegisterRoutes(r *gin.Engine) {
	group := r.Group("/clickup")
	group.GET("/workspaces", h.getWorkspaces)
	group.GET("/workspaces/:id/spaces", h.getSpaces)
	group.GET("/spaces/:id/lists", h.getLists)
	group.GET("/spaces/:id/folders", h.getFolders)
	group.GET("/lists/:id/tasks", h.getTasks)
	group.POST("/spaces/:id/folders", h.createFolder)
	group.POST("/lists/:id/tasks", h.createTask)
	// Add these two new routes:
	group.DELETE("/folders/:id", h.deleteFolder)
	group.DELETE("/tasks/:id", h.deleteTask)
}

func (h *ClickUpHandler) getWorkspaces(c *gin.Context) {
	workspaces, err := h.repo.GetWorkspaces()
	if err != nil {
		h.logger.WithError(err).Error("Failed to load workspaces from repository")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load workspaces"})
		return
	}

	if len(workspaces) == 0 {
		fetched, err := h.service.ListWorkspaces(c.Request.Context())
		if err != nil {
			h.logger.WithError(err).Error("Failed to fetch workspaces from ClickUp")
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch workspaces"})
			return
		}
		if err := h.repo.SaveWorkspaces(fetched); err != nil {
			h.logger.WithError(err).Error("Failed to persist workspaces")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist workspaces"})
			return
		}
		workspaces = fetched
	}

	c.JSON(http.StatusOK, gin.H{"workspaces": workspaces})
}

func (h *ClickUpHandler) getSpaces(c *gin.Context) {
	workspaceID := c.Param("id")

	spaces, err := h.repo.GetSpaces(workspaceID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to load spaces from repository")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load spaces"})
		return
	}

	if len(spaces) == 0 {
		fetched, err := h.service.ListSpaces(c.Request.Context(), workspaceID)
		if err != nil {
			h.logger.WithError(err).Error("Failed to fetch spaces from ClickUp")
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch spaces"})
			return
		}
		if err := h.repo.SaveSpaces(fetched); err != nil {
			h.logger.WithError(err).Error("Failed to persist spaces")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist spaces"})
			return
		}
		spaces = fetched
	}

	c.JSON(http.StatusOK, gin.H{"spaces": spaces})
}

func (h *ClickUpHandler) getLists(c *gin.Context) {
	spaceID := c.Param("id")

	lists, err := h.repo.GetLists(spaceID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to load lists from repository")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load lists"})
		return
	}

	if len(lists) == 0 {
		fetched, err := h.service.ListLists(c.Request.Context(), spaceID)
		if err != nil {
			h.logger.WithError(err).Error("Failed to fetch lists from ClickUp")
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch lists"})
			return
		}
		if err := h.repo.SaveLists(fetched); err != nil {
			h.logger.WithError(err).Error("Failed to persist lists")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist lists"})
			return
		}
		lists = fetched
	}

	c.JSON(http.StatusOK, gin.H{"lists": lists})
}

func (h *ClickUpHandler) getFolders(c *gin.Context) {
	spaceID := c.Param("id")

	folders, err := h.repo.GetFolders(spaceID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to load folders from repository")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load folders"})
		return
	}

	if len(folders) == 0 {
		fetched, err := h.service.ListFolders(c.Request.Context(), spaceID)
		if err != nil {
			h.logger.WithError(err).Error("Failed to fetch folders from ClickUp")
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch folders"})
			return
		}
		if err := h.repo.SaveFolders(fetched); err != nil {
			h.logger.WithError(err).Error("Failed to persist folders")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist folders"})
			return
		}
		folders = fetched
	}

	c.JSON(http.StatusOK, gin.H{"folders": folders})
}

func (h *ClickUpHandler) getTasks(c *gin.Context) {
	listID := c.Param("id")

	tasks, err := h.repo.GetTasks(listID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to load tasks from repository")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load tasks"})
		return
	}

	if len(tasks) == 0 {
		fetched, err := h.service.ListTasks(c.Request.Context(), listID)
		if err != nil {
			h.logger.WithError(err).Error("Failed to fetch tasks from ClickUp")
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch tasks"})
			return
		}
		if err := h.repo.SaveTasks(fetched); err != nil {
			h.logger.WithError(err).Error("Failed to persist tasks")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist tasks"})
			return
		}
		tasks = fetched
	}

	c.JSON(http.StatusOK, gin.H{"tasks": tasks})
}

func (h *ClickUpHandler) createFolder(c *gin.Context) {
	spaceID := c.Param("id")

	var payload struct {
		Name   string `json:"name"`
		Hidden bool   `json:"hidden"`
	}
	if err := c.BindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	folder, err := h.service.CreateFolder(c.Request.Context(), spaceID, payload.Name, payload.Hidden)
	if err != nil {
		h.logger.WithError(err).Error("Failed to create folder on ClickUp")
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to create folder"})
		return
	}

	if err := h.repo.SaveFolders([]model.FolderClickUp{*folder}); err != nil {
		h.logger.WithError(err).Error("Failed to persist folder")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist folder"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"folder": folder})
}

func (h *ClickUpHandler) createTask(c *gin.Context) {
	listID := c.Param("id")

	var payload struct {
		Name         string  `json:"name"`
		Description  string  `json:"description"`
		Status       string  `json:"status"`
		Priority     *int    `json:"priority"`
		TimeEstimate *int64  `json:"time_estimate"`
		Parent       *string `json:"parent"`
	}
	if err := c.BindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	taskRequest := clickup.TaskRequest{
		Name:         payload.Name,
		Description:  payload.Description,
		Status:       payload.Status,
		Priority:     payload.Priority,
		TimeEstimate: payload.TimeEstimate,
		Parent:       payload.Parent,
	}

	task, err := h.service.CreateTask(c.Request.Context(), listID, taskRequest)
	if err != nil {
		h.logger.WithError(err).Error("Failed to create task on ClickUp")
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to create task"})
		return
	}

	if err := h.repo.SaveTasks([]model.TaskClickUp{*task}); err != nil {
		h.logger.WithError(err).Error("Failed to persist task")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist task"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"task": task})
}

// Add these two new handler methods:
func (h *ClickUpHandler) deleteFolder(c *gin.Context) {
	folderID := c.Param("id")

	err := h.service.DeleteFolder(c.Request.Context(), folderID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to delete folder from ClickUp")
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to delete folder"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *ClickUpHandler) deleteTask(c *gin.Context) {
	taskID := c.Param("id")

	err := h.service.DeleteTask(c.Request.Context(), taskID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to delete task from ClickUp")
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to delete task"})
		return
	}

	c.Status(http.StatusNoContent)
}

// storeWorkspaces e le altre funzioni legacy sono state sostituite dal repository.
