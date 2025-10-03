package testing

import (
	"context"

	"github.com/acai-travel/tech-challenge/internal/chat/model"
)

// MockAssistant is a test double for the Assistant interface
type MockAssistant struct {
	TitleFunc func(ctx context.Context, conv *model.Conversation) (string, error)
	ReplyFunc func(ctx context.Context, conv *model.Conversation) (string, error)
}

func (m *MockAssistant) Title(ctx context.Context, conv *model.Conversation) (string, error) {
	if m.TitleFunc != nil {
		return m.TitleFunc(ctx, conv)
	}
	return "Mock Title", nil
}

func (m *MockAssistant) Reply(ctx context.Context, conv *model.Conversation) (string, error) {
	if m.ReplyFunc != nil {
		return m.ReplyFunc(ctx, conv)
	}
	return "Mock Reply", nil
}
