package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

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
