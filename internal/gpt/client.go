package gpt

import (
	"context"
	"errors"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared/constant"
	"github.com/sirupsen/logrus"
)

type Client struct {
	api    *openai.Client
	logger *logrus.Entry
}

func NewClient(apiKey string, logger *logrus.Entry) *Client {
	apiClient := openai.NewClient(option.WithAPIKey(apiKey))
	return &Client{
		api:    apiClient,
		logger: logger,
	}
}

func (c *Client) Ask(prompt string) (string, error) {
	c.logger.Infof("🤖 GPT Request: %s", prompt)

	resp, err := c.api.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
		Model: openai.ChatModelGPT4oMini,
		Messages: []openai.ChatCompletionMessageParamUnion{
			{
				OfUser: &openai.ChatCompletionUserMessageParam{
					Role: constant.ValueOf[constant.User](),
					Content: openai.ChatCompletionUserMessageParamContentUnion{
						OfString: openai.String(prompt),
					},
				},
			},
		},
	})
	if err != nil {
		c.logger.Error("GPT error: ", err)
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", errors.New("gpt: empty response choices")
	}

	answer := resp.Choices[0].Message.Content
	c.logger.Infof("✅ GPT Response: %s", answer)
	return answer, nil
}
