package assistant

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/acai-travel/tech-challenge/internal/chat/model"
	"github.com/acai-travel/tech-challenge/internal/chat/tool"
	"github.com/openai/openai-go/v2"
)

type Tool interface {
	Name() string
	Description() string
	Parameters() openai.FunctionParameters
	Execute(ctx context.Context, arguments string) (string, error)
}

type Registry struct {
	tools map[string]Tool
}

type Assistant struct {
	cli   openai.Client
	tools map[string]Tool
}

func New() *Assistant {
	WeatherClient := NewWeatherClient()

	a := &Assistant{
		cli:   openai.NewClient(),
		tools: map[string]Tool{},
	}

	a.registerTool(tool.NewDateTool())
	a.registerTool(tool.NewHolidaysTool())
	a.registerTool(tool.NewWeatherTool(WeatherClient))

	return a
}

func (a *Assistant) registerTool(tool Tool) {
	a.tools[tool.Name()] = tool
}

func (a *Assistant) toolDefinitions() []openai.ChatCompletionToolUnionParam {
	defs := make([]openai.ChatCompletionToolUnionParam, 0, len(a.tools))
	for _, tool := range a.tools {
		defs = append(defs, openai.ChatCompletionFunctionTool(
			openai.FunctionDefinitionParam{
				Name:        tool.Name(),
				Description: openai.String(tool.Description()),
				Parameters:  tool.Parameters(),
			},
		))
	}
	return defs
}

func (a *Assistant) executeTool(ctx context.Context, name, args string) (string, error) {
	tool, ok := a.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	return tool.Execute(ctx, args)
}

func (a *Assistant) Title(ctx context.Context, conv *model.Conversation) (string, error) {
	if len(conv.Messages) == 0 {
		return "An empty conversation", nil
	}

	slog.InfoContext(ctx, "Generating title for conversation", "conversation_id", conv.ID)

	msgs := make([]openai.ChatCompletionMessageParamUnion, 0, len(conv.Messages)+1)

	msgs = append(msgs, openai.AssistantMessage("Generate a concise, descriptive title for the conversation based on the user message. The title should be a single line, no more than 80 characters, and should not include any special characters or emojis."))
	for _, m := range conv.Messages {
		msgs = append(msgs, openai.UserMessage(m.Content))
	}

	resp, err := a.cli.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    openai.ChatModelO1,
		Messages: msgs,
	})

	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 || strings.TrimSpace(resp.Choices[0].Message.Content) == "" {
		return "", errors.New("empty response from OpenAI for title generation")
	}

	title := resp.Choices[0].Message.Content
	title = strings.ReplaceAll(title, "\n", " ")
	title = strings.Trim(title, " \t\r\n-\"'")

	if len(title) > 80 {
		title = title[:80]
	}

	return title, nil
}

func (a *Assistant) Reply(ctx context.Context, conv *model.Conversation) (string, error) {
	if len(conv.Messages) == 0 {
		return "", errors.New("conversation has no messages")
	}

	slog.InfoContext(ctx, "Generating reply for conversation", "conversation_id", conv.ID)

	msgs := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage("You are a helpful, concise AI assistant. Provide accurate, safe, and clear responses."),
	}

	for _, m := range conv.Messages {
		switch m.Role {
		case model.RoleUser:
			msgs = append(msgs, openai.UserMessage(m.Content))
		case model.RoleAssistant:
			msgs = append(msgs, openai.AssistantMessage(m.Content))
		}
	}

	for i := 0; i < 15; i++ {
		resp, err := a.cli.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model:    openai.ChatModelGPT4_1,
			Messages: msgs,
			Tools:    a.toolDefinitions(),
		})

		if err != nil {
			return "", err
		}

		if len(resp.Choices) == 0 {
			return "", errors.New("no choices returned by OpenAI")
		}

		message := resp.Choices[0].Message

		if len(message.ToolCalls) > 0 {
			msgs = append(msgs, message.ToParam())

			for _, call := range message.ToolCalls {
				slog.InfoContext(ctx, "Tool call received",
					"name", call.Function.Name,
					"args", call.Function.Arguments)

				result, err := a.executeTool(ctx, call.Function.Name, call.Function.Arguments)
				if err != nil {
					slog.ErrorContext(ctx, "Tool execution failed",
						"tool", call.Function.Name,
						"error", err)
					result = fmt.Sprintf("Tool execution failed: %v", err)
				}

				msgs = append(msgs, openai.ToolMessage(result, call.ID))
			}
			continue
		}

		return message.Content, nil
	}

	return "", errors.New("too many tool calls, unable to generate reply")
}
