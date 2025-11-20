package server

import (
	"testing"

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
			}},
		},
	}

	if workspaceTreeIncomplete(workspaces) {
		t.Fatalf("workspace tree should be considered complete without tasks")
	}
}
