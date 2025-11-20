package clickup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"gpt-clickup/src/model"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

// HTTPClient definisce l'interfaccia minima per eseguire richieste HTTP.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Service espone i metodi necessari per interagire con l'API del ClickUp.
type Service interface {
	GetCurrentUser(ctx context.Context) (*model.User, []model.WorkspaceClickUp, error)
	ListWorkspaces(ctx context.Context) ([]model.WorkspaceClickUp, error)
	ListSpaces(ctx context.Context, teamID string) ([]model.SpaceClickUp, error)
	ListLists(ctx context.Context, spaceID string) ([]model.ListClickUp, error)
	ListFolders(ctx context.Context, spaceID string) ([]model.FolderClickUp, error)
	ListTasks(ctx context.Context, listID string) ([]model.TaskClickUp, error)
	CreateFolder(ctx context.Context, spaceID string, name string, hidden bool) (*model.FolderClickUp, error)
	CreateTask(ctx context.Context, listID string, payload TaskRequest) (*model.TaskClickUp, error)
	DeleteFolder(ctx context.Context, folderID string) error
	DeleteTask(ctx context.Context, taskID string) error
	GetTask(ctx context.Context, taskID string) (*model.TaskClickUp, error)
	UpdateTask(ctx context.Context, taskID string, payload TaskRequest) (*model.TaskClickUp, error)
	ListFolderLists(ctx context.Context, folderID string) ([]model.ListClickUp, error)
}

// Client gestisce le richieste verso l'API del ClickUp.
type Client struct {
	apiKey     string
	baseURL    *url.URL
	httpClient HTTPClient
	logger     *logrus.Entry
}

// TaskRequest rappresenta il payload per la creazione di un task o subtask.
type TaskRequest struct {
	Name         string  `json:"name"`
	Description  string  `json:"description,omitempty"`
	Status       string  `json:"status,omitempty"`
	Priority     *int    `json:"priority,omitempty"`
	TimeEstimate *int64  `json:"time_estimate,omitempty"`
	Parent       *string `json:"parent,omitempty"`
}

// NewClient crea un nuovo client ClickUp utilizzando l'http.DefaultClient.
func NewClient(apiKey string, logger *logrus.Entry) *Client {
	return NewClientWithHTTP(apiKey, http.DefaultClient, logger)
}

// NewClientWithHTTP consente di specificare un HTTPClient personalizzato (utile per i test).
func NewClientWithHTTP(apiKey string, httpClient HTTPClient, logger *logrus.Entry) *Client {
	base, _ := url.Parse("https://api.clickup.com/api/v2")
	return &Client{
		apiKey:     apiKey,
		baseURL:    base,
		httpClient: httpClient,
		logger:     logger,
	}
}

// GetCurrentUser returns the user associated with the API key and their workspaces.
func (c *Client) GetCurrentUser(ctx context.Context) (*model.User, []model.WorkspaceClickUp, error) {
	var response struct {
		User struct {
			ID             any    `json:"id"`
			Username       string `json:"username"`
			Email          string `json:"email"`
			Color          string `json:"color"`
			ProfilePicture string `json:"profilePicture"`
			FirstName      string `json:"first_name"`
			LastName       string `json:"last_name"`
		} `json:"user"`
		Teams []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"teams"`
	}

	if err := c.get(ctx, "user", &response); err != nil {
		return nil, nil, err
	}

	clickUpUserID := fmt.Sprint(response.User.ID)
	username := response.User.Username
	if username == "" {
		username = response.User.Email
	}

	user := &model.User{
		Username:      username,
		Email:         response.User.Email,
		FirstName:     response.User.FirstName,
		LastName:      response.User.LastName,
		AvatarURL:     response.User.ProfilePicture,
		ClickUpUserID: clickUpUserID,
	}

	workspaces := make([]model.WorkspaceClickUp, 0, len(response.Teams))
	for _, team := range response.Teams {
		workspaces = append(workspaces, model.WorkspaceClickUp{ID: team.ID, Name: team.Name})
	}

	return user, workspaces, nil
}

// ListWorkspaces recupera tutti i workspace associati all'utente.
func (c *Client) ListWorkspaces(ctx context.Context) ([]model.WorkspaceClickUp, error) {
	var response struct {
		Teams []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"teams"`
	}

	if err := c.get(ctx, "team", &response); err != nil {
		return nil, err
	}

	workspaces := make([]model.WorkspaceClickUp, 0, len(response.Teams))
	for _, team := range response.Teams {
		workspaces = append(workspaces, model.WorkspaceClickUp{ID: team.ID, Name: team.Name})
	}
	return workspaces, nil
}

// ListSpaces recupera gli spazi di un workspace.
func (c *Client) ListSpaces(ctx context.Context, teamID string) ([]model.SpaceClickUp, error) {
	var response struct {
		Spaces []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"spaces"`
	}

	endpoint := fmt.Sprintf("team/%s/space?archived=false", teamID)
	if err := c.get(ctx, endpoint, &response); err != nil {
		return nil, err
	}

	spaces := make([]model.SpaceClickUp, 0, len(response.Spaces))
	for _, space := range response.Spaces {
		spaces = append(spaces, model.SpaceClickUp{ID: space.ID, Name: space.Name, WorkspaceID: teamID})
	}
	return spaces, nil
}

// ListLists recupera le liste top-level di uno spazio.
func (c *Client) ListLists(ctx context.Context, spaceID string) ([]model.ListClickUp, error) {
	var response struct {
		Lists []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"lists"`
	}

	endpoint := fmt.Sprintf("space/%s/list?archived=false", spaceID)
	if err := c.get(ctx, endpoint, &response); err != nil {
		return nil, err
	}

	lists := make([]model.ListClickUp, 0, len(response.Lists))
	for _, list := range response.Lists {
		lists = append(lists, model.ListClickUp{ID: list.ID, Name: list.Name, SpaceID: spaceID})
	}
	return lists, nil
}

// ListFolders recupera i folder e le relative liste di uno spazio.
func (c *Client) ListFolders(ctx context.Context, spaceID string) ([]model.FolderClickUp, error) {
	var response struct {
		Folders []struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Lists []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"lists"`
		} `json:"folders"`
	}

	endpoint := fmt.Sprintf("space/%s/folder?archived=false", spaceID)
	if err := c.get(ctx, endpoint, &response); err != nil {
		return nil, err
	}

	folders := make([]model.FolderClickUp, 0, len(response.Folders))
	for _, folder := range response.Folders {
		lists := make([]model.ListClickUp, 0, len(folder.Lists))
		folderID := folder.ID
		for _, list := range folder.Lists {
			listID := list.ID
			lists = append(lists, model.ListClickUp{ID: listID, Name: list.Name, SpaceID: spaceID, FolderID: &folderID})
		}
		folders = append(folders, model.FolderClickUp{ID: folder.ID, Name: folder.Name, SpaceID: spaceID, Lists: lists})
	}
	return folders, nil
}

// ListTasks recupera i task di una lista.
func (c *Client) ListTasks(ctx context.Context, listID string) ([]model.TaskClickUp, error) {
	var response struct {
		Tasks []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Status struct {
				Status string `json:"status"`
			} `json:"status"`
			Priority *struct {
				Priority json.RawMessage `json:"priority"`
			} `json:"priority"`
			Parent string `json:"parent"`
		} `json:"tasks"`
	}

	endpoint := fmt.Sprintf("list/%s/task?archived=false", listID)
	if err := c.get(ctx, endpoint, &response); err != nil {
		return nil, err
	}

	tasks := make([]model.TaskClickUp, 0, len(response.Tasks))
	for _, task := range response.Tasks {
		priority := decodePriority(task.Priority)
		var parent *string
		if task.Parent != "" {
			parentID := task.Parent
			parent = &parentID
		}
		tasks = append(tasks, model.TaskClickUp{ID: task.ID, Name: task.Name, ListID: listID, ParentID: parent, Status: task.Status.Status, Priority: priority})
	}
	return tasks, nil
}

// CreateFolder crea un folder in uno spazio.
func (c *Client) CreateFolder(ctx context.Context, spaceID string, name string, hidden bool) (*model.FolderClickUp, error) {
	payload := map[string]interface{}{"name": name, "hidden": hidden}
	endpoint := fmt.Sprintf("space/%s/folder", spaceID)

	var response struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	if err := c.post(ctx, endpoint, payload, &response); err != nil {
		return nil, err
	}

	result := &model.FolderClickUp{ID: response.ID, Name: response.Name, SpaceID: spaceID}
	return result, nil
}

// CreateTask crea un task o subtask in una lista specifica.
func (c *Client) CreateTask(ctx context.Context, listID string, payload TaskRequest) (*model.TaskClickUp, error) {
	endpoint := fmt.Sprintf("list/%s/task", listID)

	var response struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Status struct {
			Status string `json:"status"`
		} `json:"status"`
	}

	if err := c.post(ctx, endpoint, payload, &response); err != nil {
		return nil, err
	}

	var parent *string
	if payload.Parent != nil && *payload.Parent != "" {
		value := *payload.Parent
		parent = &value
	}

	result := &model.TaskClickUp{ID: response.ID, Name: response.Name, ListID: listID, ParentID: parent, Status: response.Status.Status}
	if payload.Priority != nil {
		result.Priority = *payload.Priority
	}
	return result, nil
}

// DeleteFolder elimina un folder esistente.
func (c *Client) DeleteFolder(ctx context.Context, folderID string) error {
	endpoint := fmt.Sprintf("folder/%s", folderID)
	return c.delete(ctx, endpoint)
}

// DeleteTask elimina un task esistente.
func (c *Client) DeleteTask(ctx context.Context, taskID string) error {
	endpoint := fmt.Sprintf("task/%s", taskID)
	return c.delete(ctx, endpoint)
}

// GetTask retrieves detailed information about a specific task.
func (c *Client) GetTask(ctx context.Context, taskID string) (*model.TaskClickUp, error) {
	var response struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Status      struct {
			Status string `json:"status"`
		} `json:"status"`
		Priority *struct {
			Priority json.RawMessage `json:"priority"`
		} `json:"priority"`
		Parent string `json:"parent"`
		List   struct {
			ID string `json:"id"`
		} `json:"list"`
	}

	endpoint := fmt.Sprintf("task/%s", taskID)
	if err := c.get(ctx, endpoint, &response); err != nil {
		return nil, err
	}

	priority := decodePriority(response.Priority)

	var parent *string
	if response.Parent != "" {
		parentID := response.Parent
		parent = &parentID
	}

	return &model.TaskClickUp{
		ID:          response.ID,
		Name:        response.Name,
		Description: response.Description,
		ListID:      response.List.ID,
		ParentID:    parent,
		Status:      response.Status.Status,
		Priority:    priority,
	}, nil
}

// UpdateTask updates an existing task.
func (c *Client) UpdateTask(ctx context.Context, taskID string, payload TaskRequest) (*model.TaskClickUp, error) {
	endpoint := fmt.Sprintf("task/%s", taskID)

	var response struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Status      struct {
			Status string `json:"status"`
		} `json:"status"`
		List struct {
			ID string `json:"id"`
		} `json:"list"`
	}

	if err := c.put(ctx, endpoint, payload, &response); err != nil {
		return nil, err
	}

	result := &model.TaskClickUp{
		ID:          response.ID,
		Name:        response.Name,
		Description: response.Description,
		ListID:      response.List.ID,
		Status:      response.Status.Status,
	}
	if payload.Priority != nil {
		result.Priority = *payload.Priority
	}
	return result, nil
}

// ListFolderLists retrieves all lists within a specific folder.
func (c *Client) ListFolderLists(ctx context.Context, folderID string) ([]model.ListClickUp, error) {
	var response struct {
		Lists []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"lists"`
	}

	endpoint := fmt.Sprintf("folder/%s/list?archived=false", folderID)
	if err := c.get(ctx, endpoint, &response); err != nil {
		return nil, err
	}

	lists := make([]model.ListClickUp, 0, len(response.Lists))
	for _, list := range response.Lists {
		lists = append(lists, model.ListClickUp{
			ID:       list.ID,
			Name:     list.Name,
			FolderID: &folderID,
		})
	}
	return lists, nil
}

func decodePriority(priorityWrapper *struct {
	Priority json.RawMessage `json:"priority"`
}) int {
	if priorityWrapper == nil || priorityWrapper.Priority == nil {
		return 0
	}

	var priorityInt int
	if err := json.Unmarshal(priorityWrapper.Priority, &priorityInt); err == nil {
		return priorityInt
	}

	var priorityStr string
	if err := json.Unmarshal(priorityWrapper.Priority, &priorityStr); err == nil {
		if parsedInt, err := strconv.Atoi(priorityStr); err == nil {
			return parsedInt
		}

		switch strings.ToLower(priorityStr) {
		case "urgent":
			return 1
		case "high":
			return 2
		case "normal":
			return 3
		case "low":
			return 4
		}
	}

	return 0
}

func (c *Client) get(ctx context.Context, endpoint string, out interface{}) error {
	return c.do(ctx, http.MethodGet, endpoint, nil, out)
}

func (c *Client) post(ctx context.Context, endpoint string, body interface{}, out interface{}) error {
	return c.do(ctx, http.MethodPost, endpoint, body, out)
}

func (c *Client) delete(ctx context.Context, endpoint string) error {
	return c.do(ctx, http.MethodDelete, endpoint, nil, nil)
}

// Add this helper method for PUT requests
func (c *Client) put(ctx context.Context, endpoint string, body interface{}, out interface{}) error {
	return c.do(ctx, http.MethodPut, endpoint, body, out)
}

func (c *Client) do(ctx context.Context, method, endpoint string, body interface{}, out interface{}) error {
	u := *c.baseURL
	trimmed := strings.TrimPrefix(endpoint, "/")
	if idx := strings.Index(trimmed, "?"); idx >= 0 {
		u.Path = path.Join(c.baseURL.Path, trimmed[:idx])
		u.RawQuery = trimmed[idx+1:]
	} else {
		u.Path = path.Join(c.baseURL.Path, trimmed)
	}

	var payload io.Reader
	var payloadPreview interface{}
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal payload: %w", err)
		}
		payload = bytes.NewBuffer(data)
		payloadPreview = string(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), payload)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	c.logger.WithFields(logrus.Fields{
		"method":   method,
		"endpoint": u.String(),
		"payload":  payloadPreview,
	}).Info("Sending ClickUp request")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ClickUp request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read ClickUp response: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"method":      method,
		"endpoint":    u.String(),
		"status_code": resp.StatusCode,
		"response":    string(data),
	}).Debug("Received ClickUp response")

	if resp.StatusCode >= 400 {
		return fmt.Errorf("ClickUp API error (%d): %s", resp.StatusCode, string(data))
	}

	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("failed to decode ClickUp response: %w", err)
		}
	}

	return nil
}

// WithBaseURL consente di modificare l'endpoint base (utile per i test con server fittizi).
func (c *Client) WithBaseURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}
	c.baseURL = parsed
	return nil
}

// WithHTTPClient consente di impostare un client HTTP personalizzato.
func (c *Client) WithHTTPClient(httpClient HTTPClient) {
	c.httpClient = httpClient
}

// Timeout imposta un timeout per il client HTTP se possibile.
func (c *Client) Timeout(d time.Duration) {
	if client, ok := c.httpClient.(*http.Client); ok {
		client.Timeout = d
	}
}
