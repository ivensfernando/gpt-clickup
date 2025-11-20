package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gpt-clickup/internal/platform/clickup"
	"gpt-clickup/src/model"
)

type fakeService struct {
	workspaces           []model.WorkspaceClickUp
	spaces               map[string][]model.SpaceClickUp
	lists                map[string][]model.ListClickUp
	folders              map[string][]model.FolderClickUp
	tasks                map[string][]model.TaskClickUp
	createdTasks         []model.TaskClickUp
	createdFolders       []model.FolderClickUp
	forbidWorkspaceFetch bool
}

func newFakeService() *fakeService {
	return &fakeService{
		spaces:  make(map[string][]model.SpaceClickUp),
		lists:   make(map[string][]model.ListClickUp),
		folders: make(map[string][]model.FolderClickUp),
		tasks:   make(map[string][]model.TaskClickUp),
	}
}

func (f *fakeService) GetCurrentUser(ctx context.Context) (*model.User, []model.WorkspaceClickUp, error) {
	user := &model.User{ID: 1, Username: "fake", Email: "fake@example.com", ClickUpUserID: "cu-1"}
	return user, nil, nil
}

func (f *fakeService) ListWorkspaces(ctx context.Context) ([]model.WorkspaceClickUp, error) {
	if f.forbidWorkspaceFetch {
		return nil, fmt.Errorf("unexpected fetch")
	}
	return f.workspaces, nil
}

func (f *fakeService) ListSpaces(ctx context.Context, teamID string) ([]model.SpaceClickUp, error) {
	return f.spaces[teamID], nil
}

func (f *fakeService) ListLists(ctx context.Context, spaceID string) ([]model.ListClickUp, error) {
	return f.lists[spaceID], nil
}

func (f *fakeService) ListFolders(ctx context.Context, spaceID string) ([]model.FolderClickUp, error) {
	return f.folders[spaceID], nil
}

func (f *fakeService) ListTasks(ctx context.Context, listID string) ([]model.TaskClickUp, error) {
	return f.tasks[listID], nil
}

func (f *fakeService) CreateFolder(ctx context.Context, spaceID string, name string, hidden bool) (*model.FolderClickUp, error) {
	folder := model.FolderClickUp{ID: "f-" + name, Name: name, SpaceID: spaceID}
	f.createdFolders = append(f.createdFolders, folder)
	return &folder, nil
}

func (f *fakeService) CreateTask(ctx context.Context, listID string, payload clickup.TaskRequest) (*model.TaskClickUp, error) {
	task := model.TaskClickUp{ID: "t-" + payload.Name, Name: payload.Name, ListID: listID, ParentID: payload.Parent}
	f.createdTasks = append(f.createdTasks, task)
	return &task, nil
}

func (f *fakeService) DeleteFolder(ctx context.Context, folderID string) error {
	for i, folder := range f.createdFolders {
		if folder.ID == folderID {
			f.createdFolders = append(f.createdFolders[:i], f.createdFolders[i+1:]...)
			break
		}
	}
	return nil
}

func (f *fakeService) DeleteTask(ctx context.Context, taskID string) error {
	for i, task := range f.createdTasks {
		if task.ID == taskID {
			f.createdTasks = append(f.createdTasks[:i], f.createdTasks[i+1:]...)
			break
		}
	}
	return nil
}

func (f *fakeService) GetTask(ctx context.Context, taskID string) (*model.TaskClickUp, error) {
	for _, task := range f.createdTasks {
		if task.ID == taskID {
			return &task, nil
		}
	}
	return nil, fmt.Errorf("task not found")
}

func (f *fakeService) UpdateTask(ctx context.Context, taskID string, payload clickup.TaskRequest) (*model.TaskClickUp, error) {
	for i, task := range f.createdTasks {
		if task.ID == taskID {
			if payload.Name != "" {
				task.Name = payload.Name
			}
			if payload.Status != "" {
				task.Status = payload.Status
			}
			f.createdTasks[i] = task
			return &task, nil
		}
	}
	return nil, fmt.Errorf("task not found")
}

func (f *fakeService) ListFolderLists(ctx context.Context, folderID string) ([]model.ListClickUp, error) {
	for _, folder := range f.folders[folderID] {
		if folder.ID == folderID {
			return folder.Lists, nil
		}
	}
	return nil, nil
}

func (f *fakeService) GetListStatuses(ctx context.Context, listID string) ([]clickup.Status, error) {
	return nil, nil
}

func setupTestRouter(service clickup.Service, repo ClickUpRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	logger := logrus.New().WithField("test", true)
	handler := NewClickUpHandler(service, repo, logger)
	router := gin.New()
	handler.RegisterRoutes(router)
	return router
}

func TestGetWorkspacesCachesResults(t *testing.T) {
	service := newFakeService()
	service.workspaces = []model.WorkspaceClickUp{{ID: "1", Name: "Workspace"}}
	repo := NewMemoryClickUpRepository()

	router := setupTestRouter(service, repo)

	req, _ := http.NewRequest(http.MethodGet, "/clickup/workspaces", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.Code)
	}

	service.forbidWorkspaceFetch = true
	req2, _ := http.NewRequest(http.MethodGet, "/clickup/workspaces", nil)
	resp2 := httptest.NewRecorder()
	router.ServeHTTP(resp2, req2)
	if resp2.Code != http.StatusOK {
		t.Fatalf("unexpected status on cached call: %d", resp2.Code)
	}
}

func TestCreateFolderPersistsData(t *testing.T) {
	service := newFakeService()
	repo := NewMemoryClickUpRepository()
	router := setupTestRouter(service, repo)

	payload := map[string]any{"name": "Work", "hidden": false}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, "/clickup/spaces/space-1/folders", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("unexpected status: %d", resp.Code)
	}

	stored, err := repo.GetFolders("space-1")
	if err != nil {
		t.Fatalf("repo query failed: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("expected folder persisted, got %d", len(stored))
	}

	t.Cleanup(func() {
		_ = service.DeleteFolder(context.Background(), stored[0].ID)
		repo.Reset()
	})
}

func TestCreateTaskPersistsData(t *testing.T) {
	service := newFakeService()
	repo := NewMemoryClickUpRepository()
	router := setupTestRouter(service, repo)

	payload := map[string]any{"name": "Subtask", "description": "desc", "status": "to do", "parent": "task-parent"}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, "/clickup/lists/list-1/tasks", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("unexpected status: %d", resp.Code)
	}

	stored, err := repo.GetTasks("list-1")
	if err != nil {
		t.Fatalf("repo query failed: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("expected task persisted, got %d", len(stored))
	}
	if stored[0].ParentID == nil || *stored[0].ParentID != "task-parent" {
		t.Fatalf("parent not stored: %#v", stored[0])
	}

	t.Cleanup(func() {
		_ = service.DeleteTask(context.Background(), stored[0].ID)
		repo.Reset()
	})
}
