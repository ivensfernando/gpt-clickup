package clickup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
)

type Client struct {
	apiKey string
	logger *logrus.Logger
}

func NewClient(apiKey string, logger *logrus.Logger) *Client {
	return &Client{
		apiKey: apiKey,
		logger: logger,
	}
}

func (c *Client) CreateTask(taskName string) (string, error) {
	listID := os.Getenv("CLICKUP_LIST_ID")

	url := fmt.Sprintf("https://api.clickup.com/api/v2/list/%s/task", listID)

	body := map[string]string{
		"name":        taskName,
		"description": "Criado automaticamente pelo GPT",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	req.Header.Set("Authorization", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	c.logger.Infof("📝 Enviando tarefa ao ClickUp: %s", taskName)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.logger.Error("Erro ao criar tarefa: ", err)
		return "", err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	c.logger.Infof("📦 ClickUp response: %s", string(data))

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("erro do ClickUp: %s", string(data))
	}

	var result struct {
		ID string `json:"id"`
	}
	json.Unmarshal(data, &result)

	return result.ID, nil
}
