package gpt

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared/constant"
	"github.com/sirupsen/logrus"
)

type Client struct {
	api    chatCompletionAPI
	logger *logrus.Entry
}

type chatCompletionAPI interface {
	Create(ctx context.Context, params openai.ChatCompletionNewParams) (*openai.ChatCompletion, error)
}

type openAIChatCompletionAPI struct {
	client *openai.Client
}

func (a *openAIChatCompletionAPI) Create(ctx context.Context, params openai.ChatCompletionNewParams) (*openai.ChatCompletion, error) {
	return a.client.Chat.Completions.New(ctx, params)
}

// Message represents a single chat message sent to the GPT models.
type Message struct {
	Role    string
	Content string
}

const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// SystemMessage helper for system role.
func SystemMessage(content string) Message {
	return Message{Role: RoleSystem, Content: content}
}

// UserMessage helper for user role.
func UserMessage(content string) Message {
	return Message{Role: RoleUser, Content: content}
}

func NewClient(apiKey string, logger *logrus.Entry) *Client {
	apiClient := openai.NewClient(option.WithAPIKey(apiKey))
	return &Client{
		api:    &openAIChatCompletionAPI{client: &apiClient},
		logger: logger,
	}
}

// NewClientWithAPI allows injecting a custom API implementation (useful for testing).
func NewClientWithAPI(api chatCompletionAPI, logger *logrus.Entry) *Client {
	return &Client{
		api:    api,
		logger: logger,
	}
}

// Ask keeps backward compatibility by delegating to Chat with a single user message.
func (c *Client) Ask(prompt string) (string, error) {
	return c.Chat([]Message{UserMessage(prompt)})
}

// Chat executes a chat completion with an arbitrary set of messages.
func (c *Client) Chat(messages []Message) (string, error) {
	if len(messages) == 0 {
		return "", errors.New("gpt: at least one message is required")
	}
	c.logger.Infof("🤖 GPT Request with %d message(s)", len(messages))

	payload := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))
	for _, msg := range messages {
		switch msg.Role {
		case RoleSystem:
			payload = append(payload, openai.ChatCompletionMessageParamUnion{
				OfSystem: &openai.ChatCompletionSystemMessageParam{
					Role: constant.ValueOf[constant.System](),
					Content: openai.ChatCompletionSystemMessageParamContentUnion{
						OfString: openai.String(msg.Content),
					},
				},
			})
		case RoleAssistant:
			payload = append(payload, openai.ChatCompletionMessageParamUnion{
				OfAssistant: &openai.ChatCompletionAssistantMessageParam{
					Role: constant.ValueOf[constant.Assistant](),
					Content: openai.ChatCompletionAssistantMessageParamContentUnion{
						OfString: openai.String(msg.Content),
					},
				},
			})
		default:
			payload = append(payload, openai.ChatCompletionMessageParamUnion{
				OfUser: &openai.ChatCompletionUserMessageParam{
					Role: constant.ValueOf[constant.User](),
					Content: openai.ChatCompletionUserMessageParamContentUnion{
						OfString: openai.String(msg.Content),
					},
				},
			})
		}
	}

	resp, err := c.api.Create(context.Background(), openai.ChatCompletionNewParams{
		Model:    openai.ChatModelGPT4oMini,
		Messages: payload,
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

// DebugString returns a shortened version of a message payload, useful for logs.
func DebugString(messages []Message) string {
	parts := make([]string, 0, len(messages))
	for _, msg := range messages {
		parts = append(parts, fmt.Sprintf("%s: %.80s", msg.Role, msg.Content))
	}
	return strings.Join(parts, " | ")
}
