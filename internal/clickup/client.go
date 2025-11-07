package clickup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
)

type Client struct {
	apiKey string
	listID string
	logger *logrus.Entry
}

func NewClient(apiKey, listID string, logger *logrus.Entry) *Client {
	return &Client{
		apiKey: apiKey,
		listID: listID,
		logger: logger,
	}
}

func (c *Client) CreateTask(taskName string) (string, error) {
	url := fmt.Sprintf("https://api.clickup.com/api/v2/list/%s/task", c.listID)

	body := map[string]string{
		"name":        taskName,
		"description": "Criado automaticamente pelo GPT",
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		c.logger.WithError(err).Error("erro ao serializar payload para ClickUp")
		return "", fmt.Errorf("erro ao preparar requisição do ClickUp: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		c.logger.WithError(err).Error("erro ao criar requisição para ClickUp")
		return "", fmt.Errorf("erro ao criar requisição do ClickUp: %w", err)
	}
	req.Header.Set("Authorization", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	c.logger.Infof("📝 Enviando tarefa ao ClickUp: %s", taskName)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.logger.WithError(err).Error("erro ao enviar requisição ao ClickUp")
		return "", fmt.Errorf("erro ao chamar ClickUp: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logger.WithError(err).Error("erro ao ler resposta do ClickUp")
		return "", fmt.Errorf("erro ao ler resposta do ClickUp: %w", err)
	}
	c.logger.Infof("📦 ClickUp response: %s", string(data))

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("erro do ClickUp: %s", string(data))
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		c.logger.WithError(err).Error("erro ao decodificar resposta do ClickUp")
		return "", fmt.Errorf("erro ao interpretar resposta do ClickUp: %w", err)
	}

	return result.ID, nil
}
