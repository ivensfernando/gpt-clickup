package clickup

import (
	"context"
	"testing"

	"github.com/openai/openai-go/shared/constant"
	"github.com/sirupsen/logrus"
)

type mockAPI struct {
	response string
	err      error
}

func (m *mockAPI) Create(ctx context.Context, params openai.ChatCompletionNewParams) (*openai.ChatCompletion, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &openai.ChatCompletion{
		ID:      "test-completion",
		Object:  constant.ValueOf[constant.ChatCompletion](),
		Created: 1234567890,
		Model:   string(openai.ChatModelGPT4oMini),
		Usage: openai.CompletionUsage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
		Choices: []openai.ChatCompletionChoice{
			{
				FinishReason: "stop",
				Index:        0,
				Logprobs:     openai.ChatCompletionChoiceLogprobs{},
				Message: openai.ChatCompletionMessage{
					Role:    constant.ValueOf[constant.Assistant](),
					Content: m.response,
				},
			},
		},
	}, nil
}

func TestClientAsk(t *testing.T) {
	tests := []struct {
		name     string
		mock     mockAPI
		prompt   string
		wantResp string
		wantErr  bool
	}{
		{
			name: "successful response",
			mock: mockAPI{
				response: "Test response",
			},
			prompt:   "Test prompt",
			wantResp: "Test response",
			wantErr:  false,
		},
		{
			name: "api error",
			mock: mockAPI{
				err: &openai.Error{
					Type:    "invalid_request_error",
					Message: "Test error",
				},
			},
			prompt:   "Test prompt",
			wantResp: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logrus.New().WithField("test", true)
			client := NewClientWithAPI(&tt.mock, logger)

			resp, err := client.Ask(tt.prompt)
			if (err != nil) != tt.wantErr {
				t.Errorf("Ask() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if resp != tt.wantResp {
				t.Errorf("Ask() = %v, want %v", resp, tt.wantResp)
			}
		})
	}
}
