package server

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"

	"gpt-clickup/internal/platform/clickup"
	"gpt-clickup/src/model"
)

func TestIsWorkspaceListingPrompt(t *testing.T) {
	cases := []struct {
		prompt string
		expect bool
	}{
		{"Listar workspaces", true},
		{"Preciso listar workspace agora", true},
		{"list workspaces please", true},
		{"create task", false},
	}

	for _, tc := range cases {
		if got := isWorkspaceListingPrompt(tc.prompt); got != tc.expect {
			t.Fatalf("prompt %q expected %v got %v", tc.prompt, tc.expect, got)
		}
	}
}

func TestFindListAnywhereMatchesSingleToken(t *testing.T) {
	workspaces := []model.WorkspaceClickUp{
		{
			ID:   "ws-1",
			Name: "Workspace",
			Spaces: []model.SpaceClickUp{{
				ID:          "sp-1",
				Name:        "Space",
				WorkspaceID: "ws-1",
				Lists:       []model.ListClickUp{{ID: "list-123", Name: "Habits"}},
			}},
		},
	}

	if result := findListAnywhere(workspaces, []string{"list-123"}); result == nil || result.ID != "list-123" {
		t.Fatalf("expected to find list by ID, got %#v", result)
	}

	if result := findListAnywhere(workspaces, nil); result != nil {
		t.Fatalf("expected nil for empty path, got %#v", result)
	}
}

func TestDetectTaskListingIntentMatchesWorkspaceByName(t *testing.T) {
	workspaces := []model.WorkspaceClickUp{
		{
			ID:   "123",
			Name: "Personal",
			Spaces: []model.SpaceClickUp{{
				ID:          "sp-1",
				Name:        "Personal",
				WorkspaceID: "123",
			}},
		},
		{ID: "456", Name: "Work"},
	}

	if intent := detectTaskListingIntent("listar tarefas abertas do workspace personal", workspaces); intent == nil || intent.workspace.ID != "123" || !intent.openOnly {
		t.Fatalf("expected to match workspace '123' with openOnly, got %#v", intent)
	}

	if intent := detectTaskListingIntent("listar tasks do workspace 456", workspaces); intent == nil || intent.workspace.ID != "456" || intent.openOnly {
		t.Fatalf("expected to match workspace '456' without openOnly, got %#v", intent)
	}

	if intent := detectTaskListingIntent("listar tarefas abertas do space personal", workspaces); intent == nil || intent.workspace.ID != "123" || intent.space == nil || intent.space.ID != "sp-1" {
		t.Fatalf("expected to match space 'sp-1' within workspace '123', got %#v", intent)
	}

	if intent := detectTaskListingIntent("create task", workspaces); intent != nil {
		t.Fatalf("expected nil intent for unrelated prompt, got %#v", intent)
	}
}

func TestListWorkspaceTasksFiltersClosedStatuses(t *testing.T) {
	repo := NewMemoryClickUpRepository()
	repo.SaveWorkspaces([]model.WorkspaceClickUp{{ID: "ws-1", Name: "Personal"}})
	repo.SaveSpaces([]model.SpaceClickUp{{ID: "sp-1", Name: "Space", WorkspaceID: "ws-1"}})
	repo.SaveLists([]model.ListClickUp{{ID: "list-1", Name: "Inbox", SpaceID: "sp-1"}})
	repo.SaveTasks([]model.TaskClickUp{{ID: "task-1", Name: "Open task", ListID: "list-1", Status: "open"}, {ID: "task-2", Name: "Done task", ListID: "list-1", Status: "closed"}})

	workspaceTree, _ := repo.GetWorkspaceTree()
	logger := logrus.New().WithField("component", "test")
	endpoint := NewGPTClickUpEndpoint(nil, &stubClickUpService{}, repo, logger)

	result, err := endpoint.listWorkspaceTasks(context.Background(), &workspaceTree[0], nil, false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 || result[0].ID != "task-1" {
		t.Fatalf("expected only open tasks, got %#v", result)
	}
}

func TestListWorkspaceTasksFiltersBySpaceWhenProvided(t *testing.T) {
	repo := NewMemoryClickUpRepository()
	repo.SaveWorkspaces([]model.WorkspaceClickUp{{ID: "ws-1", Name: "Workspace"}})
	repo.SaveSpaces([]model.SpaceClickUp{{
		ID:          "sp-1",
		Name:        "Personal",
		WorkspaceID: "ws-1",
	}, {
		ID:          "sp-2",
		Name:        "Trading",
		WorkspaceID: "ws-1",
	}})
	repo.SaveLists([]model.ListClickUp{{ID: "list-1", Name: "Inbox", SpaceID: "sp-1"}, {ID: "list-2", Name: "Backlog", SpaceID: "sp-2"}})
	repo.SaveTasks([]model.TaskClickUp{{ID: "task-1", Name: "Task A", ListID: "list-1", Status: "open"}, {ID: "task-2", Name: "Task B", ListID: "list-2", Status: "open"}})

	workspaceTree, _ := repo.GetWorkspaceTree()
	logger := logrus.New().WithField("component", "test")
	endpoint := NewGPTClickUpEndpoint(nil, &stubClickUpService{}, repo, logger)

	space := workspaceTree[0].Spaces[0]
	result, err := endpoint.listWorkspaceTasks(context.Background(), &workspaceTree[0], &space, false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 || result[0].ID != "task-1" || result[0].SpaceName != "Personal" {
		t.Fatalf("expected only tasks from the selected space, got %#v", result)
	}
}

func TestWorkspaceTreeIncompleteIgnoresTasks(t *testing.T) {
	workspaces := []model.WorkspaceClickUp{
		{
			ID:   "ws-1",
			Name: "Workspace",
			Spaces: []model.SpaceClickUp{{
				ID:          "sp-1",
				Name:        "Space",
				WorkspaceID: "ws-1",
				Lists:       []model.ListClickUp{{ID: "list-1", Name: "List"}},
				Folders:     []model.FolderClickUp{},
			}},
		},
	}

	if workspaceTreeIncomplete(workspaces) {
		t.Fatalf("workspace tree should be considered complete without tasks")
	}
}

type stubClickUpService struct{}

func (s *stubClickUpService) GetCurrentUser(ctx context.Context) (*model.User, []model.WorkspaceClickUp, error) {
	return nil, nil, nil
}

func (s *stubClickUpService) ListWorkspaces(ctx context.Context) ([]model.WorkspaceClickUp, error) {
	return nil, nil
}
func (s *stubClickUpService) ListSpaces(ctx context.Context, teamID string) ([]model.SpaceClickUp, error) {
	return nil, nil
}
func (s *stubClickUpService) ListLists(ctx context.Context, spaceID string) ([]model.ListClickUp, error) {
	return nil, nil
}
func (s *stubClickUpService) ListFolders(ctx context.Context, spaceID string) ([]model.FolderClickUp, error) {
	return nil, nil
}
func (s *stubClickUpService) ListTasks(ctx context.Context, listID string) ([]model.TaskClickUp, error) {
	return nil, nil
}
func (s *stubClickUpService) CreateFolder(ctx context.Context, spaceID string, name string, hidden bool) (*model.FolderClickUp, error) {
	return nil, nil
}
func (s *stubClickUpService) CreateTask(ctx context.Context, listID string, payload clickup.TaskRequest) (*model.TaskClickUp, error) {
	return nil, nil
}
func (s *stubClickUpService) DeleteFolder(ctx context.Context, folderID string) error { return nil }
func (s *stubClickUpService) DeleteTask(ctx context.Context, taskID string) error     { return nil }
func (s *stubClickUpService) GetTask(ctx context.Context, taskID string) (*model.TaskClickUp, error) {
	return nil, nil
}
func (s *stubClickUpService) UpdateTask(ctx context.Context, taskID string, payload clickup.TaskRequest) (*model.TaskClickUp, error) {
	return nil, nil
}
func (s *stubClickUpService) ListFolderLists(ctx context.Context, folderID string) ([]model.ListClickUp, error) {
	return nil, nil
}
