package gpt

import (
	"context"
	"github.com/openai/openai-go"
	"github.com/sirupsen/logrus"
)

type Client struct {
	api    *openai.Client
	logger *logrus.Logger
}

func NewClient(apiKey string, logger *logrus.Logger) *Client {
	return &Client{
		api:    openai.NewClient(apiKey),
		logger: logger,
	}
}

func (c *Client) Ask(prompt string) (string, error) {
	c.logger.Infof("🤖 GPT Request: %s", prompt)

	resp, err := c.api.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
		Model: openai.F(openai.ChatModelGPT4oMini),
		Messages: openai.F([]openai.ChatCompletionMessageParam{
			{Role: openai.F("user"), Content: openai.F(prompt)},
		}),
	})
	if err != nil {
		c.logger.Error("GPT error: ", err)
		return "", err
	}

	answer := *resp.Choices[0].Message.Content
	c.logger.Infof("✅ GPT Response: %s", answer)
	return answer, nil
}
