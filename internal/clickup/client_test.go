package clickup

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestClientListWorkspaces(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/team" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "test-token" {
			t.Fatalf("unexpected auth header: %s", auth)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"teams": []map[string]any{{"id": "1", "name": "Workspace"}}})
	}))
	defer server.Close()

	client := NewClient("test-token", logrus.New().WithField("test", true))
	if err := client.WithBaseURL(server.URL + "/api/v2"); err != nil {
		t.Fatalf("failed to set base url: %v", err)
	}
	client.WithHTTPClient(server.Client())

	workspaces, err := client.ListWorkspaces(context.Background())
	if err != nil {
		t.Fatalf("ListWorkspaces error: %v", err)
	}
	if len(workspaces) != 1 || workspaces[0].ID != "1" {
		t.Fatalf("unexpected workspaces: %#v", workspaces)
	}
}

func TestClientListFoldersAndLists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "folder") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"folders": []map[string]any{{
			"id":    "10",
			"name":  "Folder",
			"lists": []map[string]any{{"id": "100", "name": "List"}},
		}}})
	}))
	defer server.Close()

	client := NewClient("token", logrus.New().WithField("test", true))
	if err := client.WithBaseURL(server.URL + "/api/v2"); err != nil {
		t.Fatalf("failed to set base url: %v", err)
	}
	client.WithHTTPClient(server.Client())

	folders, err := client.ListFolders(context.Background(), "space")
	if err != nil {
		t.Fatalf("ListFolders error: %v", err)
	}
	if len(folders) != 1 || len(folders[0].Lists) != 1 {
		t.Fatalf("unexpected folders: %#v", folders)
	}
	if folders[0].Lists[0].FolderID == nil || *folders[0].Lists[0].FolderID != "10" {
		t.Fatalf("folder id not set correctly: %#v", folders[0].Lists[0])
	}
}

func TestClientCreateAndDeleteTask(t *testing.T) {
	var created bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/task"):
			created = true
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "task-1", "name": "Task", "status": map[string]any{"status": "to do"}})
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "task/task-1"):
			if !created {
				t.Fatalf("delete called before create")
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient("token", logrus.New().WithField("test", true))
	if err := client.WithBaseURL(server.URL + "/api/v2"); err != nil {
		t.Fatalf("failed to set base url: %v", err)
	}
	client.WithHTTPClient(server.Client())

	task, err := client.CreateTask(context.Background(), "123", TaskRequest{Name: "Task"})
	if err != nil {
		t.Fatalf("CreateTask error: %v", err)
	}
	if task.ID != "task-1" {
		t.Fatalf("unexpected task: %#v", task)
	}

	if err := client.DeleteTask(context.Background(), task.ID); err != nil {
		t.Fatalf("DeleteTask error: %v", err)
	}
}

func TestClientListTasksParsesPriority(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"tasks": []map[string]any{{
			"id":   "task-2",
			"name": "Task",
			"status": map[string]any{
				"status": "to do",
			},
			"priority": map[string]any{
				"priority": 3,
			},
			"parent": "parent-1",
		}}})
	}))
	defer server.Close()

	client := NewClient("token", logrus.New().WithField("test", true))
	if err := client.WithBaseURL(server.URL + "/api/v2"); err != nil {
		t.Fatalf("failed to set base url: %v", err)
	}
	client.WithHTTPClient(server.Client())

	tasks, err := client.ListTasks(context.Background(), "list-1")
	if err != nil {
		t.Fatalf("ListTasks error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("unexpected tasks: %#v", tasks)
	}
	if tasks[0].Priority != 3 {
		t.Fatalf("priority not parsed: %#v", tasks[0])
	}
	if tasks[0].ParentID == nil || *tasks[0].ParentID != "parent-1" {
		t.Fatalf("parent not parsed: %#v", tasks[0])
	}
}
