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

	if intent := detectTaskListingIntent("listar tarefas abertas do workspace personal", workspaces); intent == nil {
		t.Fatalf("expected intent for workspace '123', got nil")
	} else if intent.workspace.ID != "123" || !intent.openOnly {
		t.Fatalf("expected to match workspace '123' with openOnly, got id=%s openOnly=%v", intent.workspace.ID, intent.openOnly)
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

	result, err := endpoint.listWorkspaceTasks(context.Background(), &workspaceTree[0], nil, nil, nil, false, true)
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
	result, err := endpoint.listWorkspaceTasks(context.Background(), &workspaceTree[0], &space, nil, nil, false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 || result[0].ID != "task-1" || result[0].SpaceName != "Personal" {
		t.Fatalf("expected only tasks from the selected space, got %#v", result)
	}
}

func TestListWorkspaceTasksFiltersByListOrFolder(t *testing.T) {
	repo := NewMemoryClickUpRepository()
	repo.SaveWorkspaces([]model.WorkspaceClickUp{{ID: "ws-1", Name: "Workspace"}})
	repo.SaveSpaces([]model.SpaceClickUp{{ID: "sp-1", Name: "Space", WorkspaceID: "ws-1"}})
	repo.SaveLists([]model.ListClickUp{{ID: "list-1", Name: "Inbox", SpaceID: "sp-1"}, {ID: "list-2", Name: "Backlog", SpaceID: "sp-1"}})
	folderID := "folder-1"
	repo.SaveFolders([]model.FolderClickUp{{ID: "folder-1", Name: "Projects", SpaceID: "sp-1", Lists: []model.ListClickUp{{ID: "list-3", Name: "Sprint", SpaceID: "sp-1", FolderID: &folderID}}}})
	repo.SaveTasks([]model.TaskClickUp{{ID: "task-1", Name: "Space Task", ListID: "list-1", Status: "open"}, {ID: "task-2", Name: "Other Space Task", ListID: "list-2", Status: "open"}, {ID: "task-3", Name: "Folder Task", ListID: "list-3", Status: "open"}})

	workspaceTree, _ := repo.GetWorkspaceTree()
	logger := logrus.New().WithField("component", "test")
	endpoint := NewGPTClickUpEndpoint(nil, &stubClickUpService{}, repo, logger)

	space := workspaceTree[0].Spaces[0]

	targetList := &space.Lists[0]
	result, err := endpoint.listWorkspaceTasks(context.Background(), &workspaceTree[0], nil, nil, targetList, false, true)
	if err != nil {
		t.Fatalf("unexpected error filtering by list: %v", err)
	}

	if len(result) != 1 || result[0].ID != "task-1" {
		t.Fatalf("expected only tasks from the selected list, got %#v", result)
	}

	targetFolder := &space.Folders[0]
	folderResult, err := endpoint.listWorkspaceTasks(context.Background(), &workspaceTree[0], nil, targetFolder, nil, false, true)
	if err != nil {
		t.Fatalf("unexpected error filtering by folder: %v", err)
	}

	if len(folderResult) != 1 || folderResult[0].ID != "task-3" || folderResult[0].FolderName == nil || *folderResult[0].FolderName != "Projects" {
		t.Fatalf("expected only tasks from the selected folder, got %#v", folderResult)
	}
}

func TestListWorkspaceTasksDedupesListsFromFolders(t *testing.T) {
	repo := NewMemoryClickUpRepository()
	repo.SaveWorkspaces([]model.WorkspaceClickUp{{ID: "ws-1", Name: "Workspace"}})

	folderID := "folder-1"
	workspaceTree := []model.WorkspaceClickUp{{
		ID:   "ws-1",
		Name: "Workspace",
		Spaces: []model.SpaceClickUp{{
			ID:          "sp-1",
			Name:        "Space",
			WorkspaceID: "ws-1",
			Lists:       []model.ListClickUp{{ID: "list-1", Name: "Inbox", SpaceID: "sp-1"}},
			Folders: []model.FolderClickUp{{
				ID:      folderID,
				Name:    "Projects",
				SpaceID: "sp-1",
				Lists:   []model.ListClickUp{{ID: "list-1", Name: "Inbox", SpaceID: "sp-1", FolderID: &folderID}},
			}},
		}},
	}}

	repo.SaveTasks([]model.TaskClickUp{{ID: "task-1", Name: "Update LinkedIn profile", ListID: "list-1", Status: "open"}})

	logger := logrus.New().WithField("component", "test")
	endpoint := NewGPTClickUpEndpoint(nil, &stubClickUpService{}, repo, logger)

	tasks, err := endpoint.listWorkspaceTasks(context.Background(), &workspaceTree[0], nil, nil, nil, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) != 1 || tasks[0].FolderName != nil {
		t.Fatalf("expected single task without folder context, got %#v", tasks)
	}
}

func TestDetectTaskListingIntentMatchesListAndFolder(t *testing.T) {
	workspaces := []model.WorkspaceClickUp{
		{
			ID:   "123",
			Name: "Personal",
			Spaces: []model.SpaceClickUp{{
				ID:          "sp-1",
				Name:        "Personal",
				WorkspaceID: "123",
				Lists:       []model.ListClickUp{{ID: "list-1", Name: "Habits"}},
				Folders: []model.FolderClickUp{{
					ID:      "folder-1",
					Name:    "Projects",
					SpaceID: "sp-1",
					Lists:   []model.ListClickUp{{ID: "list-2", Name: "Sprint"}},
				}},
			}},
		},
	}

	intent := detectTaskListingIntent("listar tarefas da lista Habits", workspaces)
	if intent == nil || intent.workspace.ID != "123" || intent.list == nil || intent.list.ID != "list-1" || intent.folder != nil {
		t.Fatalf("expected to match the space list, got %#v", intent)
	}

	intent = detectTaskListingIntent("listar tarefas do folder Projects", workspaces)
	if intent == nil || intent.folder == nil || intent.folder.ID != "folder-1" {
		t.Fatalf("expected to match the folder, got %#v", intent)
	}

	intent = detectTaskListingIntent("listar tarefas da lista Sprint", workspaces)
	if intent == nil || intent.list == nil || intent.list.ID != "list-2" || intent.folder == nil || intent.folder.ID != "folder-1" {
		t.Fatalf("expected to match list inside folder, got %#v", intent)
	}
}

func TestTryCompleteTaskMatchesByName(t *testing.T) {
	repo := NewMemoryClickUpRepository()
	repo.SaveWorkspaces([]model.WorkspaceClickUp{{ID: "ws-1", Name: "Personal"}})
	repo.SaveSpaces([]model.SpaceClickUp{{ID: "sp-1", Name: "Work Tasks", WorkspaceID: "ws-1"}})
	repo.SaveLists([]model.ListClickUp{{ID: "list-1", Name: "To Do", SpaceID: "sp-1"}})
	repo.SaveTasks([]model.TaskClickUp{{ID: "task-1", Name: "Update LinkedIn profile", ListID: "list-1", Status: "to do"}})

	updatedPayload := clickup.TaskRequest{}
	service := &stubClickUpService{
		statuses: map[string][]clickup.Status{"list-1": {{Name: "done", Type: "closed"}}},
		updateFunc: func(ctx context.Context, taskID string, payload clickup.TaskRequest) (*model.TaskClickUp, error) {
			updatedPayload = payload
			return &model.TaskClickUp{ID: taskID, Name: "Update LinkedIn profile", ListID: "list-1", Status: payload.Status}, nil
		},
	}

	workspaceTree, _ := repo.GetWorkspaceTree()
	logger := logrus.New().WithField("component", "test")
	endpoint := NewGPTClickUpEndpoint(nil, service, repo, logger)

	updated, err := endpoint.tryCompleteTask(context.Background(), "Fechar a tarefa Update LinkedIn profile.", workspaceTree, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updated == nil || updated.ID != "task-1" || updated.Status != "done" {
		t.Fatalf("expected task to be closed, got %#v", updated)
	}

	if updatedPayload.Status != "done" {
		t.Fatalf("expected status 'done' to be sent to ClickUp, got %#v", updatedPayload)
	}
}

func TestScoreTaskMatchRequiresReasonableScore(t *testing.T) {
	task := taskWithContext{ID: "task-1", Name: "Update LinkedIn profile"}

	if score := scoreTaskMatch("", task); score != 0 {
		t.Fatalf("expected empty prompt to score 0, got %d", score)
	}

	prompt := "Fechar tarefa de linkedin"
	score := scoreTaskMatch(prompt, task)
	if score <= 0 {
		t.Fatalf("expected positive score for prompt %q", prompt)
	}

	prompt = "Fechar"
	if score := scoreTaskMatch(prompt, task); score != 0 {
		t.Fatalf("expected generic prompt to be ignored, got %d", score)
	}
}

func TestFindTaskSearchMatchesReturnsExactMatch(t *testing.T) {
	repo := NewMemoryClickUpRepository()
	repo.SaveWorkspaces([]model.WorkspaceClickUp{{ID: "ws-1", Name: "Workspace"}})
	repo.SaveSpaces([]model.SpaceClickUp{{ID: "sp-1", Name: "Personal", WorkspaceID: "ws-1"}})
	repo.SaveLists([]model.ListClickUp{{ID: "list-1", Name: "Inbox", SpaceID: "sp-1"}})
	repo.SaveTasks([]model.TaskClickUp{{ID: "task-1", Name: "Update LinkedIn profile", ListID: "list-1", Status: "open"}})

	if tasks, _ := repo.GetTasks("list-1"); len(tasks) == 0 {
		t.Fatalf("expected tasks to be stored in memory repository")
	}

	workspaceTree, _ := repo.GetWorkspaceTree()
	logger := logrus.New().WithField("component", "test")
	endpoint := NewGPTClickUpEndpoint(nil, &stubClickUpService{}, repo, logger)

	tasks, err := endpoint.fetchListTasks(context.Background(), "list-1", false)
	if err != nil || len(tasks) == 0 {
		t.Fatalf("expected tasks to be retrievable, got len=%d err=%v", len(tasks), err)
	}

	matches := endpoint.findTaskSearchMatches(context.Background(), "busque a tarefa Update LinkedIn profile", workspaceTree, false)
	if len(matches) != 1 {
		t.Fatalf("expected a single match, got %#v", matches)
	}
	if matches[0].ID != "task-1" || matches[0].ListName != "Inbox" || matches[0].WorkspaceName != "Workspace" {
		t.Fatalf("unexpected match content: %#v", matches[0])
	}
}

func TestFindTaskSearchMatchesDedupesLists(t *testing.T) {
	repo := NewMemoryClickUpRepository()
	folderID := "folder-1"
	repo.SaveTasks([]model.TaskClickUp{{ID: "task-1", Name: "Update LinkedIn profile", ListID: "list-1", Status: "open"}})

	if tasks, _ := repo.GetTasks("list-1"); len(tasks) == 0 {
		t.Fatalf("expected tasks to be stored in memory repository")
	}

	workspaceTree := []model.WorkspaceClickUp{{
		ID:   "ws-1",
		Name: "Workspace",
		Spaces: []model.SpaceClickUp{{
			ID:          "sp-1",
			Name:        "Personal",
			WorkspaceID: "ws-1",
			Lists:       []model.ListClickUp{{ID: "list-1", Name: "Inbox", SpaceID: "sp-1"}},
			Folders: []model.FolderClickUp{{
				ID:      folderID,
				Name:    "Projects",
				SpaceID: "sp-1",
				Lists:   []model.ListClickUp{{ID: "list-1", Name: "Inbox", SpaceID: "sp-1", FolderID: &folderID}},
			}},
		}},
	}}
	logger := logrus.New().WithField("component", "test")
	endpoint := NewGPTClickUpEndpoint(nil, &stubClickUpService{}, repo, logger)

	tasks, err := endpoint.fetchListTasks(context.Background(), "list-1", false)
	if err != nil || len(tasks) == 0 {
		t.Fatalf("expected tasks to be retrievable, got len=%d err=%v", len(tasks), err)
	}

	matches := endpoint.findTaskSearchMatches(context.Background(), "busque a tarefa Update LinkedIn profile", workspaceTree, false)
	if len(matches) != 1 {
		t.Fatalf("expected a single deduped match, got %#v", matches)
	}
}

func TestFindTaskSearchMatchesIgnoresPromptsWithoutSearchIntent(t *testing.T) {
	repo := NewMemoryClickUpRepository()
	repo.SaveWorkspaces([]model.WorkspaceClickUp{{ID: "ws-1", Name: "Workspace"}})
	repo.SaveSpaces([]model.SpaceClickUp{{ID: "sp-1", Name: "Personal", WorkspaceID: "ws-1"}})
	repo.SaveLists([]model.ListClickUp{{ID: "list-1", Name: "Inbox", SpaceID: "sp-1"}})
	repo.SaveTasks([]model.TaskClickUp{{ID: "task-1", Name: "Update LinkedIn profile", ListID: "list-1", Status: "open"}})

	workspaceTree, _ := repo.GetWorkspaceTree()
	logger := logrus.New().WithField("component", "test")
	endpoint := NewGPTClickUpEndpoint(nil, &stubClickUpService{}, repo, logger)

	matches := endpoint.findTaskSearchMatches(context.Background(), "listar tarefas", workspaceTree, false)
	if matches != nil {
		t.Fatalf("expected nil matches for listing intent, got %#v", matches)
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

type stubClickUpService struct {
	tasksByList map[string][]model.TaskClickUp
	updateFunc  func(ctx context.Context, taskID string, payload clickup.TaskRequest) (*model.TaskClickUp, error)
	statuses    map[string][]clickup.Status
}

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
	if s.tasksByList == nil {
		return nil, nil
	}
	return s.tasksByList[listID], nil
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
	if s.updateFunc != nil {
		return s.updateFunc(ctx, taskID, payload)
	}
	return nil, nil
}

func (s *stubClickUpService) GetListStatuses(ctx context.Context, listID string) ([]clickup.Status, error) {
	return s.statuses[listID], nil
}
func (s *stubClickUpService) ListFolderLists(ctx context.Context, folderID string) ([]model.ListClickUp, error) {
	return nil, nil
}
