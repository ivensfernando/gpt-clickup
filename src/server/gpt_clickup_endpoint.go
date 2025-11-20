package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gpt-clickup/internal/gpt"
	"gpt-clickup/internal/platform/clickup"
	"gpt-clickup/src/model"
)

const (
	plannerSystemManual = `Você é um planejador especializado em ClickUp.\n` +
		`Sua função é receber o pedido do usuário, analisar o mapa atual de workspaces/spaces/listas ` +
		`e devolver UM JSON com a task principal que deve ser criada.\n` +
		`Sempre responda no seguinte formato:\n` +
		`{"task":{"name":"...","description":"...","status":"backlog","priority":1},"explanation":"..."}` +
		`\n- "name" deve ser curto (até 80 caracteres).\n` +
		`- "description" pode conter detalhes adicionais.\n` +
		`- "status" precisa ser um dos status que já existem na lista alvo (se não souber use "backlog").\n` +
		`- "priority" é um inteiro de 1 (alta) a 4 (baixa).\n` +
		`Se não houver informações suficientes para atender ao pedido explique o motivo em "explanation".`
)

// GPTClickUpEndpoint encapsula o endpoint /gpt-clickup.
type GPTClickUpEndpoint struct {
	gpt     *gpt.Client
	service clickup.Service
	repo    ClickUpRepository
	logger  *logrus.Entry
}

// GPTClickUpRequest representa o payload aceito pelo endpoint.
type GPTClickUpRequest struct {
	Prompt    string   `json:"prompt" binding:"required"`
	ListID    string   `json:"list_id"`
	ListPath  []string `json:"list_path"`
	ForceSync bool     `json:"force_sync"`
}

type gptPlannerTask struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Priority    *int   `json:"priority"`
}

type gptPlannerResponse struct {
	Task        gptPlannerTask `json:"task"`
	Explanation string         `json:"explanation"`
}

type taskWithContext struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Status        string  `json:"status"`
	Priority      int     `json:"priority"`
	ListID        string  `json:"list_id"`
	ListName      string  `json:"list_name"`
	SpaceName     string  `json:"space_name"`
	WorkspaceName string  `json:"workspace_name"`
	FolderName    *string `json:"folder_name,omitempty"`
}

type taskListingIntent struct {
	workspace *model.WorkspaceClickUp
	space     *model.SpaceClickUp
	folder    *model.FolderClickUp
	list      *model.ListClickUp
	openOnly  bool
}

// NewGPTClickUpEndpoint cria um novo handler especializado para o endpoint inteligente.
func NewGPTClickUpEndpoint(gptClient *gpt.Client, service clickup.Service, repo ClickUpRepository, logger *logrus.Entry) *GPTClickUpEndpoint {
	return &GPTClickUpEndpoint{
		gpt:     gptClient,
		service: service,
		repo:    repo,
		logger:  logger,
	}
}

// Handle executa o fluxo planner -> execução -> resposta.
func (h *GPTClickUpEndpoint) Handle(c *gin.Context) {
	h.logger.WithFields(logrus.Fields{
		"operation": "handle_request",
		"path":      c.FullPath(),
		"method":    c.Request.Method,
	}).Info("incoming request")

	var req GPTClickUpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.WithError(err).WithFields(logrus.Fields{
			"operation": "bind_request",
			"path":      c.FullPath(),
		}).Warn("failed to bind request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	workspaces, err := h.workspaceSnapshot(c.Request.Context(), req.ForceSync)
	if err != nil {
		h.logger.WithError(err).WithField("operation", "workspace_snapshot").Error("failed to build workspace snapshot")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build workspace snapshot"})
		return
	}

	if isWorkspaceListingPrompt(req.Prompt) {
		c.JSON(http.StatusOK, gin.H{"workspace_map": workspaces})
		return
	}

	if updated, err := h.tryCompleteTask(c.Request.Context(), req.Prompt, workspaces, req.ForceSync); err != nil {
		h.logger.WithError(err).WithField("operation", "complete_task").Error("failed to complete task")
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to complete task"})
		return
	} else if updated != nil {
		c.JSON(http.StatusOK, gin.H{"updated_task": updated})
		return
	}

	if matches := h.findTaskSearchMatches(c.Request.Context(), req.Prompt, workspaces, req.ForceSync); len(matches) > 0 {
		c.JSON(http.StatusOK, gin.H{"matched_tasks": matches, "workspace_map": workspaces})
		return
	}

	if intent := detectTaskListingIntent(req.Prompt, workspaces); intent != nil {
		tasks, err := h.listWorkspaceTasks(c.Request.Context(), intent.workspace, intent.space, intent.folder, intent.list, req.ForceSync, intent.openOnly)
		if err != nil {
			h.logger.WithError(err).WithField("operation", "list_workspace_tasks").Error("failed to list workspace tasks")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list workspace tasks"})
			return
		}

		response := gin.H{
			"workspace": gin.H{"id": intent.workspace.ID, "name": intent.workspace.Name},
			"tasks":     tasks,
			"open_only": intent.openOnly,
		}

		if intent.space != nil {
			response["space"] = gin.H{"id": intent.space.ID, "name": intent.space.Name}
		}

		if intent.folder != nil {
			response["folder"] = gin.H{"id": intent.folder.ID, "name": intent.folder.Name}
		}

		if intent.list != nil {
			response["list"] = gin.H{"id": intent.list.ID, "name": intent.list.Name}
		}

		c.JSON(http.StatusOK, response)
		return
	}

	if task := h.findTaskCompletionTarget(c.Request.Context(), req.Prompt, workspaces, req.ForceSync); task != nil {
		status, err := h.resolveClosedStatus(c.Request.Context(), task.ListID)
		if err != nil {
			h.logger.WithError(err).WithField("operation", "resolve_closed_status").Error("failed to determine closed status")
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to determine closed status", "details": err.Error()})
			return
		}

		updatePayload := clickup.TaskRequest{Status: status}
		updated, err := h.service.UpdateTask(c.Request.Context(), task.ID, updatePayload)
		if err != nil {
			h.logger.WithError(err).WithFields(logrus.Fields{"operation": "complete_task", "task_id": task.ID}).Error("failed to complete task")
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to complete task", "details": err.Error()})
			return
		}

		if err := h.repo.SaveTasks([]model.TaskClickUp{*updated}); err != nil {
			h.logger.WithError(err).WithFields(logrus.Fields{"operation": "persist_task", "task_id": updated.ID}).Warn("failed to persist updated task")
		}

		task.Status = status
		c.JSON(http.StatusOK, gin.H{"completed_task": task, "applied_status": status, "workspace_map": workspaces})
		return
	}

	listID, list := h.resolveList(workspaces, req)
	if listID == "" {
		h.logger.WithFields(logrus.Fields{
			"operation": "resolve_list",
			"list_id":   req.ListID,
			"list_path": req.ListPath,
		}).Warn("unable to resolve list")
		c.JSON(http.StatusBadRequest, gin.H{"error": "list_id or a valid list_path must be provided"})
		return
	}

	messages, err := h.buildPlannerMessages(req.Prompt, listID, workspaces, req.ListPath)
	if err != nil {
		h.logger.WithError(err).WithField("operation", "build_planner_messages").Error("failed to build GPT planner prompt")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build GPT planner prompt"})
		return
	}

	planRaw, err := h.gpt.Chat(messages)
	if err != nil {
		h.logger.WithError(err).WithField("operation", "gpt_planner").Error("failed to contact GPT planner")
		c.JSON(http.StatusBadGateway, gin.H{"error": "planner failed", "details": err.Error()})
		return
	}

	plan, err := parsePlannerResponse(planRaw)
	if err != nil {
		h.logger.WithError(err).WithField("operation", "parse_planner_response").Warn("GPT planner returned invalid JSON, falling back to prompt text")
		plan = gptPlannerResponse{
			Task: gptPlannerTask{Name: truncateString(req.Prompt, 80), Description: planRaw, Status: "backlog"},
		}
	}

	payload := clickup.TaskRequest{
		Name:        fallback(plan.Task.Name, truncateString(req.Prompt, 80)),
		Description: fallback(plan.Task.Description, req.Prompt),
		Status:      fallback(plan.Task.Status, "backlog"),
	}
	if plan.Task.Priority != nil {
		payload.Priority = plan.Task.Priority
	}

	task, err := h.service.CreateTask(c.Request.Context(), listID, payload)
	if err != nil {
		h.logger.WithError(err).WithFields(logrus.Fields{
			"operation": "create_task",
			"list_id":   listID,
		}).Error("failed to create task")
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to create task", "details": err.Error(), "planner": planRaw})
		return
	}

	if err := h.repo.SaveTasks([]model.TaskClickUp{*task}); err != nil {
		h.logger.WithError(err).WithFields(logrus.Fields{
			"operation": "persist_task",
			"list_id":   listID,
			"task_id":   task.ID,
		}).Warn("failed to persist task created by GPT endpoint")
	}

	if list != nil {
		list.Tasks = append(list.Tasks, *task)
	}

	c.JSON(http.StatusOK, gin.H{
		"planner_messages": messages,
		"planner_raw":      planRaw,
		"planner":          plan,
		"list_id":          listID,
		"created_task":     task,
		"workspace_map":    workspaces,
	})
}

func (h *GPTClickUpEndpoint) workspaceSnapshot(ctx context.Context, force bool) ([]model.WorkspaceClickUp, error) {
	workspaces, err := h.repo.GetWorkspaceTree()
	if err != nil {
		return nil, err
	}

	if force || workspaceTreeIncomplete(workspaces) {
		if err := h.refreshWorkspaceSnapshot(ctx); err != nil {
			return nil, err
		}
		return h.repo.GetWorkspaceTree()
	}

	return workspaces, nil
}

func (h *GPTClickUpEndpoint) refreshWorkspaceSnapshot(ctx context.Context) error {
	workspaces, err := h.service.ListWorkspaces(ctx)
	if err != nil {
		return err
	}
	if err := h.repo.SaveWorkspaces(workspaces); err != nil {
		return err
	}

	for _, workspace := range workspaces {
		spaces, err := h.service.ListSpaces(ctx, workspace.ID)
		if err != nil {
			return err
		}
		if err := h.repo.SaveSpaces(spaces); err != nil {
			return err
		}

		for _, space := range spaces {
			if err := h.syncSpaceChildren(ctx, space.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *GPTClickUpEndpoint) syncSpaceChildren(ctx context.Context, spaceID string) error {
	lists, err := h.service.ListLists(ctx, spaceID)
	if err != nil {
		return err
	}
	if err := h.repo.SaveLists(lists); err != nil {
		return err
	}

	folders, err := h.service.ListFolders(ctx, spaceID)
	if err != nil {
		return err
	}
	if err := h.repo.SaveFolders(folders); err != nil {
		return err
	}
	return nil
}

func (h *GPTClickUpEndpoint) resolveList(workspaces []model.WorkspaceClickUp, req GPTClickUpRequest) (string, *model.ListClickUp) {
	if req.ListID != "" {
		for i := range workspaces {
			for j := range workspaces[i].Spaces {
				space := &workspaces[i].Spaces[j]
				for k := range space.Lists {
					if space.Lists[k].ID == req.ListID {
						return req.ListID, &space.Lists[k]
					}
				}
				for k := range space.Folders {
					folder := &space.Folders[k]
					for l := range folder.Lists {
						if folder.Lists[l].ID == req.ListID {
							return req.ListID, &folder.Lists[l]
						}
					}
				}
			}
		}
		return req.ListID, nil
	}

	list := findListByPath(workspaces, req.ListPath)
	if list == nil {
		list = findListAnywhere(workspaces, req.ListPath)
	}
	if list == nil {
		return "", nil
	}
	return list.ID, list
}

func (h *GPTClickUpEndpoint) buildPlannerMessages(prompt, listID string, workspaces []model.WorkspaceClickUp, path []string) ([]gpt.Message, error) {
	payload := struct {
		Workspaces []model.WorkspaceClickUp `json:"workspaces"`
		ListID     string                   `json:"target_list_id"`
		ListPath   []string                 `json:"list_path"`
	}{Workspaces: workspaces, ListID: listID, ListPath: path}

	ctxBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	userPrompt := fmt.Sprintf("Pergunta original: %s\nContexto conhecido (JSON): %s", prompt, string(ctxBytes))
	return []gpt.Message{
		gpt.SystemMessage(plannerSystemManual),
		gpt.UserMessage(userPrompt),
	}, nil
}

func (h *GPTClickUpEndpoint) listWorkspaceTasks(ctx context.Context, workspace *model.WorkspaceClickUp, targetSpace *model.SpaceClickUp, targetFolder *model.FolderClickUp, targetList *model.ListClickUp, forceSync, openOnly bool) ([]taskWithContext, error) {
	tasks := make([]taskWithContext, 0)

	addTasks := func(list model.ListClickUp, space model.SpaceClickUp, folderName *string) error {
		listTasks, err := h.fetchListTasks(ctx, list.ID, forceSync)
		if err != nil {
			return err
		}

		for _, task := range listTasks {
			if openOnly && isClosedStatus(task.Status) {
				continue
			}
			tasks = append(tasks, taskWithContext{
				ID:            task.ID,
				Name:          task.Name,
				Status:        task.Status,
				Priority:      task.Priority,
				ListID:        list.ID,
				ListName:      list.Name,
				SpaceName:     space.Name,
				WorkspaceName: workspace.Name,
				FolderName:    folderName,
			})
		}
		return nil
	}

	spaces := workspace.Spaces
	if targetSpace != nil {
		spaces = []model.SpaceClickUp{*targetSpace}
	}

	for _, space := range spaces {
		if targetFolder == nil {
			for _, list := range space.Lists {
				if intentList := list.ID; intentList != "" && targetList != nil && intentList != targetList.ID {
					continue
				}
				if err := addTasks(list, space, nil); err != nil {
					return nil, err
				}
			}
		}

		for _, folder := range space.Folders {
			if intentFolder := folder.ID; intentFolder != "" && targetFolder != nil && intentFolder != targetFolder.ID {
				continue
			}

			folderName := folder.Name
			for _, list := range folder.Lists {
				if intentList := list.ID; intentList != "" && targetList != nil && intentList != targetList.ID {
					continue
				}

				if err := addTasks(list, space, &folderName); err != nil {
					return nil, err
				}
			}
		}
	}

	return tasks, nil
}

func (h *GPTClickUpEndpoint) tryCompleteTask(ctx context.Context, prompt string, workspaces []model.WorkspaceClickUp, forceSync bool) (*taskWithContext, error) {
	if !hasCompletionIntent(prompt) {
		return nil, nil
	}

	allTasks := make([]taskWithContext, 0)
	for idx := range workspaces {
		if idx >= len(workspaces) {
			return nil, fmt.Errorf("workspace index out of bounds: %d", idx)
		}
		taskList, err := h.listWorkspaceTasks(ctx, &workspaces[idx], nil, nil, nil, forceSync, false)
		if err != nil {
			return nil, err
		}
		allTasks = append(allTasks, taskList...)
	}

	candidate := findBestMatchingTask(prompt, allTasks)
	if candidate == nil {
		return nil, nil
	}

	updated, err := h.service.UpdateTask(ctx, candidate.ID, clickup.TaskRequest{Status: "closed"})
	if err != nil {
		return nil, err
	}

	if updated != nil {
		candidate.Name = updated.Name
		candidate.Status = updated.Status
		candidate.Priority = updated.Priority

		save := model.TaskClickUp{ID: updated.ID, Name: updated.Name, ListID: updated.ListID, Status: updated.Status, Priority: updated.Priority}
		if err := h.repo.SaveTasks([]model.TaskClickUp{save}); err != nil {
			h.logger.WithError(err).WithFields(logrus.Fields{"operation": "persist_task", "task_id": updated.ID}).Warn("failed to persist completed task")
		}
	}

	return candidate, nil
}

func findBestMatchingTask(prompt string, tasks []taskWithContext) *taskWithContext {
	normalized := strings.ToLower(prompt)
	bestScore := 0
	var best *taskWithContext

	for idx := range tasks {
		score := scoreTaskMatch(normalized, tasks[idx])
		if score > bestScore {
			bestScore = score
			best = &tasks[idx]
		}
	}

	if bestScore < 20 {
		return nil
	}

	return best
}

func scoreTaskMatch(prompt string, task taskWithContext) int {
	score := 0
	name := strings.ToLower(strings.TrimSpace(task.Name))

	if name == "" || prompt == "" {
		return score
	}

	if strings.Contains(prompt, strings.ToLower(task.ID)) {
		score += 120
	}
	if strings.Contains(prompt, name) {
		score += 100
	}
	if strings.Contains(name, prompt) {
		score += 80
	}

	for _, token := range strings.Fields(name) {
		token = strings.TrimSpace(token)
		if len(token) < 3 {
			continue
		}
		if strings.Contains(prompt, token) {
			score += 10
		}
	}

	return score
}

func hasCompletionIntent(prompt string) bool {
	value := strings.ToLower(strings.TrimSpace(prompt))
	if value == "" {
		return false
	}

	keywords := []string{"fechar", "concluir", "finalizar", "completar", "complete", "close", "done", "finalize"}
	for _, kw := range keywords {
		if strings.Contains(value, kw) {
			return true
		}
	}
	return false
}

func (h *GPTClickUpEndpoint) fetchListTasks(ctx context.Context, listID string, forceSync bool) ([]model.TaskClickUp, error) {
	tasks, err := h.repo.GetTasks(listID)
	if err != nil {
		return nil, err
	}

	if forceSync || len(tasks) == 0 {
		fetched, err := h.service.ListTasks(ctx, listID)
		if err != nil {
			return nil, err
		}
		tasks = fetched
		if err := h.repo.SaveTasks(fetched); err != nil {
			h.logger.WithError(err).WithFields(logrus.Fields{"operation": "persist_tasks", "list_id": listID}).Warn("failed to persist tasks")
		}
	}

	return tasks, nil
}

func findListByPath(workspaces []model.WorkspaceClickUp, path []string) *model.ListClickUp {
	if len(path) < 3 {
		return nil
	}

	for wi := range workspaces {
		ws := &workspaces[wi]
		if !matchesToken(ws.Name, ws.ID, path[0]) {
			continue
		}
		for si := range ws.Spaces {
			space := &ws.Spaces[si]
			if !matchesToken(space.Name, space.ID, path[1]) {
				continue
			}
			if len(path) == 3 {
				for li := range space.Lists {
					if matchesToken(space.Lists[li].Name, space.Lists[li].ID, path[2]) {
						return &space.Lists[li]
					}
				}
			}
			if len(path) >= 4 {
				for fi := range space.Folders {
					folder := &space.Folders[fi]
					if !matchesToken(folder.Name, folder.ID, path[2]) {
						continue
					}
					for li := range folder.Lists {
						if matchesToken(folder.Lists[li].Name, folder.Lists[li].ID, path[3]) {
							return &folder.Lists[li]
						}
					}
				}
			}
		}
	}
	return nil
}

func matchesToken(name, id, token string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}
	return strings.EqualFold(name, token) || strings.EqualFold(id, token)
}

func workspaceTreeIncomplete(workspaces []model.WorkspaceClickUp) bool {
	if len(workspaces) == 0 {
		return true
	}
	for _, workspace := range workspaces {
		if len(workspace.Spaces) == 0 {
			return true
		}
		for _, space := range workspace.Spaces {
			if space.Lists == nil || space.Folders == nil {
				return true
			}
			if len(space.Lists) == 0 && len(space.Folders) == 0 {
				return true
			}
			for _, folder := range space.Folders {
				if folder.Lists == nil {
					return true
				}
			}
		}
	}
	return false
}

func findListAnywhere(workspaces []model.WorkspaceClickUp, path []string) *model.ListClickUp {
	if len(path) == 0 {
		return nil
	}

	for wi := range workspaces {
		ws := &workspaces[wi]
		for si := range ws.Spaces {
			space := &ws.Spaces[si]
			for li := range space.Lists {
				if matchesToken(space.Lists[li].Name, space.Lists[li].ID, path[0]) {
					return &space.Lists[li]
				}
			}
			for fi := range space.Folders {
				folder := &space.Folders[fi]
				for li := range folder.Lists {
					if matchesToken(folder.Lists[li].Name, folder.Lists[li].ID, path[0]) {
						return &folder.Lists[li]
					}
				}
			}
		}
	}

	return nil
}

func mentionedInPrompt(normalized, name, id string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	id = strings.ToLower(strings.TrimSpace(id))

	return tokenMentioned(normalized, name) || tokenMentioned(normalized, id)
}

func tokenMentioned(normalized, token string) bool {
	token = strings.ToLower(strings.TrimSpace(token))
	if token == "" {
		return false
	}

	remaining := normalized
	offset := 0

	for {
		idx := strings.Index(remaining, token)
		if idx == -1 {
			return false
		}

		start := offset + idx
		end := start + len(token)

		if boundaryOK(normalized, start, end) {
			return true
		}

		offset = end
		if offset >= len(normalized) {
			return false
		}
		remaining = normalized[offset:]
	}
}

func boundaryOK(normalized string, start, end int) bool {
	if start > 0 {
		r, _ := utf8.DecodeLastRuneInString(normalized[:start])
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
	}

	if end < len(normalized) {
		r, _ := utf8.DecodeRuneInString(normalized[end:])
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
	}

	return true
}

func detectTaskListingIntent(prompt string, workspaces []model.WorkspaceClickUp) *taskListingIntent {
	normalized := strings.ToLower(strings.TrimSpace(prompt))
	if normalized == "" {
		return nil
	}

	if !(strings.Contains(normalized, "task") || strings.Contains(normalized, "taref")) {
		return nil
	}

	var chosen *model.WorkspaceClickUp
	var chosenSpace *model.SpaceClickUp
	var chosenFolder *model.FolderClickUp
	var chosenList *model.ListClickUp

	for idx := range workspaces {
		ws := &workspaces[idx]

		if mentionedInPrompt(normalized, ws.Name, ws.ID) {
			chosen = ws
		}

		for si := range ws.Spaces {
			sp := &ws.Spaces[si]

			if mentionedInPrompt(normalized, sp.Name, sp.ID) {
				chosen = ws
				chosenSpace = sp
			}

			for li := range sp.Lists {
				list := &sp.Lists[li]
				if mentionedInPrompt(normalized, list.Name, list.ID) {
					chosen = ws
					chosenSpace = sp
					chosenFolder = nil
					chosenList = list
					break
				}
			}

			if chosenList != nil {
				break
			}

			for fi := range sp.Folders {
				folder := &sp.Folders[fi]
				if mentionedInPrompt(normalized, folder.Name, folder.ID) {
					chosen = ws
					chosenSpace = sp
					chosenFolder = folder
				}

				for li := range folder.Lists {
					list := &folder.Lists[li]
					if mentionedInPrompt(normalized, list.Name, list.ID) {
						chosen = ws
						chosenSpace = sp
						chosenFolder = folder
						chosenList = list
						break
					}
				}

				if chosenList != nil {
					break
				}
			}

			if chosenList != nil {
				break
			}
		}

		if chosenList != nil {
			break
		}
	}

	if chosen == nil && len(workspaces) == 1 {
		chosen = &workspaces[0]
	}

	if chosen == nil {
		return nil
	}

	return &taskListingIntent{workspace: chosen, space: chosenSpace, folder: chosenFolder, list: chosenList, openOnly: wantsOnlyOpenTasks(normalized)}
}

func wantsOnlyOpenTasks(prompt string) bool {
	keywords := []string{"abert", "open", "pendente", "fazendo", "doing", "em andamento"}
	for _, kw := range keywords {
		if strings.Contains(prompt, kw) {
			return true
		}
	}
	return false
}

func isClosedStatus(status string) bool {
	if status == "" {
		return false
	}
	normalized := strings.ToLower(status)
	closed := []string{"closed", "done", "completed", "complete", "archiv", "cancel"}
	for _, token := range closed {
		if strings.Contains(normalized, token) {
			return true
		}
	}
	return false
}

func isWorkspaceListingPrompt(prompt string) bool {
	value := strings.ToLower(strings.TrimSpace(prompt))
	return strings.Contains(value, "listar workspaces") || strings.Contains(value, "list workspaces") || strings.Contains(value, "listar workspace")
}

func parsePlannerResponse(raw string) (gptPlannerResponse, error) {
	var resp gptPlannerResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return resp, err
	}
	if resp.Task.Name == "" {
		return resp, errors.New("planner response missing task.name")
	}
	return resp, nil
}

func fallback(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}

func truncateString(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	if limit <= 3 {
		return value[:limit]
	}
	return value[:limit-3] + "..."
}

func (h *GPTClickUpEndpoint) findTaskCompletionTarget(ctx context.Context, prompt string, workspaces []model.WorkspaceClickUp, forceSync bool) *taskWithContext {
	normalized := strings.ToLower(prompt)
	if normalized == "" {
		return nil
	}

	keywords := []string{"fechar", "feche", "close", "encerrar", "concluir", "finalizar"}
	matchesIntent := false
	for _, kw := range keywords {
		if strings.Contains(normalized, kw) {
			matchesIntent = true
			break
		}
	}
	if !matchesIntent {
		return nil
	}

	var best *taskWithContext

	for wi := range workspaces {
		workspace := &workspaces[wi]
		for si := range workspace.Spaces {
			space := &workspace.Spaces[si]

			scanList := func(list model.ListClickUp, folderName *string) bool {
				listTasks, err := h.fetchListTasks(ctx, list.ID, forceSync)
				if err != nil {
					h.logger.WithError(err).WithFields(logrus.Fields{"operation": "load_tasks", "list_id": list.ID}).Warn("failed to load tasks for completion")
					return false
				}

				for _, task := range listTasks {
					name := strings.ToLower(strings.TrimSpace(task.Name))
					if name == "" || !strings.Contains(normalized, name) {
						continue
					}

					candidate := taskWithContext{
						ID:            task.ID,
						Name:          task.Name,
						Status:        task.Status,
						Priority:      task.Priority,
						ListID:        list.ID,
						ListName:      list.Name,
						SpaceName:     space.Name,
						WorkspaceName: workspace.Name,
						FolderName:    folderName,
					}

					if best == nil || len(candidate.Name) > len(best.Name) {
						copy := candidate
						best = &copy
					}
				}

				return false
			}

			for _, list := range space.Lists {
				scanList(list, nil)
			}
			for _, folder := range space.Folders {
				folderName := folder.Name
				for _, list := range folder.Lists {
					scanList(list, &folderName)
				}
			}
		}
	}

	return best
}

func (h *GPTClickUpEndpoint) findTaskSearchMatches(ctx context.Context, prompt string, workspaces []model.WorkspaceClickUp, forceSync bool) []taskWithContext {
	normalized := strings.ToLower(strings.TrimSpace(prompt))
	if normalized == "" {
		return nil
	}

	keywords := []string{"busc", "procur", "encontr", "ach", "localiz", "verifique"}
	matchesIntent := false
	for _, kw := range keywords {
		if strings.Contains(normalized, kw) {
			matchesIntent = true
			break
		}
	}
	if !matchesIntent {
		return nil
	}

	results := make([]taskWithContext, 0)
	for wi := range workspaces {
		workspace := &workspaces[wi]
		for si := range workspace.Spaces {
			space := &workspace.Spaces[si]

			scanList := func(list model.ListClickUp, folderName *string) {
				listTasks, err := h.fetchListTasks(ctx, list.ID, forceSync)
				if err != nil {
					h.logger.WithError(err).WithFields(logrus.Fields{"operation": "load_tasks", "list_id": list.ID}).Warn("failed to load tasks for search")
					return
				}

				for _, task := range listTasks {
					name := strings.ToLower(strings.TrimSpace(task.Name))
					if name == "" || !strings.Contains(normalized, name) {
						continue
					}

					results = append(results, taskWithContext{
						ID:            task.ID,
						Name:          task.Name,
						Status:        task.Status,
						Priority:      task.Priority,
						ListID:        list.ID,
						ListName:      list.Name,
						SpaceName:     space.Name,
						WorkspaceName: workspace.Name,
						FolderName:    folderName,
					})
				}
			}

			for _, list := range space.Lists {
				scanList(list, nil)
			}
			for _, folder := range space.Folders {
				folderName := folder.Name
				for _, list := range folder.Lists {
					scanList(list, &folderName)
				}
			}
		}
	}

	if len(results) == 0 {
		return nil
	}

	return results
}

func (h *GPTClickUpEndpoint) resolveClosedStatus(ctx context.Context, listID string) (string, error) {
	statuses, err := h.service.GetListStatuses(ctx, listID)
	if err != nil {
		return "", err
	}

	for _, status := range statuses {
		if status.Type == "closed" || isClosedStatus(status.Name) {
			return status.Name, nil
		}
	}

	if len(statuses) > 0 {
		return statuses[len(statuses)-1].Name, nil
	}

	return "closed", nil
}
