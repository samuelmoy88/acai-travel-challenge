package chat

import (
	"context"
	"errors"
	"testing"

	"github.com/acai-travel/tech-challenge/internal/chat/model"
	. "github.com/acai-travel/tech-challenge/internal/chat/testing"
	"github.com/acai-travel/tech-challenge/internal/pb"
	"github.com/google/go-cmp/cmp"
	"github.com/twitchtv/twirp"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestServer_DescribeConversation(t *testing.T) {
	ctx := context.Background()
	srv := NewServer(model.New(ConnectMongo()), nil)

	t.Run("describe existing conversation", WithFixture(func(t *testing.T, f *Fixture) {
		c := f.CreateConversation()

		out, err := srv.DescribeConversation(ctx, &pb.DescribeConversationRequest{ConversationId: c.ID.Hex()})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, want := out.GetConversation(), c.Proto()
		if !cmp.Equal(got, want, protocmp.Transform()) {
			t.Errorf("DescribeConversation() mismatch (-got +want):\n%s", cmp.Diff(got, want, protocmp.Transform()))
		}
	}))

	t.Run("describe non existing conversation should return 404", WithFixture(func(t *testing.T, f *Fixture) {
		_, err := srv.DescribeConversation(ctx, &pb.DescribeConversationRequest{ConversationId: "08a59244257c872c5943e2a2"})
		if err == nil {
			t.Fatal("expected error for non-existing conversation, got nil")
		}

		if te, ok := err.(twirp.Error); !ok || te.Code() != twirp.NotFound {
			t.Fatalf("expected twirp.NotFound error, got %v", err)
		}
	}))
}

func TestServer_StartConversation(t *testing.T) {
	ctx := context.Background()

	t.Run("create a new conversation with title and reply", WithFixture(func(t *testing.T, f *Fixture) {
		mockAssist := &MockAssistant{
			TitleFunc: func(ctx context.Context, conv *model.Conversation) (string, error) {
				return "Weather in Barcelona", nil
			},
			ReplyFunc: func(ctx context.Context, conv *model.Conversation) (string, error) {
				return "The weather in Barcelona is sunny with a temperature of 22°C.", nil
			},
		}

		srv := NewServer(model.New(ConnectMongo()), mockAssist)

		resp, err := srv.StartConversation(ctx, &pb.StartConversationRequest{
			Message: "What's the weather in Barcelona?",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.GetTitle() != "Weather in Barcelona" {
			t.Errorf("expected title 'Weather in Barcelona', got %q", resp.GetTitle())
		}

		if resp.GetReply() != "The weather in Barcelona is sunny with a temperature of 22°C." {
			t.Errorf("unexpected reply: %q", resp.GetReply())
		}

		if resp.GetConversationId() == "" {
			t.Error("expected conversation ID to be set")
		}

		saved, err := f.Repository.DescribeConversation(ctx, resp.GetConversationId())
		if err != nil {
			t.Fatalf("failed to fetch saved conversation: %v", err)
		}

		if saved.Title != "Weather in Barcelona" {
			t.Errorf("saved conversation has wrong title: %q", saved.Title)
		}

		if len(saved.Messages) != 2 {
			t.Fatalf("expected 2 messages, got %d", len(saved.Messages))
		}

		if saved.Messages[0].Role != model.RoleUser {
			t.Errorf("first message should be user, got %v", saved.Messages[0].Role)
		}
		if saved.Messages[0].Content != "What's the weather in Barcelona?" {
			t.Errorf("unexpected user message: %q", saved.Messages[0].Content)
		}

		if saved.Messages[1].Role != model.RoleAssistant {
			t.Errorf("second message should be assistant, got %v", saved.Messages[1].Role)
		}
		if saved.Messages[1].Content != "The weather in Barcelona is sunny with a temperature of 22°C." {
			t.Errorf("unexpected assistant message: %q", saved.Messages[1].Content)
		}
	}))

	t.Run("returns error when message is empty", WithFixture(func(t *testing.T, f *Fixture) {
		mockAssist := &MockAssistant{}
		srv := NewServer(model.New(ConnectMongo()), mockAssist)

		_, err := srv.StartConversation(ctx, &pb.StartConversationRequest{
			Message: "   ",
		})

		if err == nil {
			t.Fatal("expected error for empty message, got nil")
		}

		twerr, ok := err.(twirp.Error)
		if !ok {
			t.Fatalf("expected twirp.Error, got %T", err)
		}

		if twerr.Code() != twirp.InvalidArgument {
			t.Errorf("expected InvalidArgument error, got %v", twerr.Code())
		}
	}))

	t.Run("continues when title generation fails", WithFixture(func(t *testing.T, f *Fixture) {
		mockAssist := &MockAssistant{
			TitleFunc: func(ctx context.Context, conv *model.Conversation) (string, error) {
				return "", errors.New("OpenAI quota exceeded")
			},
			ReplyFunc: func(ctx context.Context, conv *model.Conversation) (string, error) {
				return "This is the reply", nil
			},
		}

		srv := NewServer(model.New(ConnectMongo()), mockAssist)

		resp, err := srv.StartConversation(ctx, &pb.StartConversationRequest{
			Message: "Hello!",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.GetTitle() != "Untitled conversation" {
			t.Errorf("expected default title, got %q", resp.GetTitle())
		}

		if resp.GetReply() != "This is the reply" {
			t.Errorf("unexpected reply: %q", resp.GetReply())
		}
	}))

	t.Run("returns error when reply generation fails", WithFixture(func(t *testing.T, f *Fixture) {
		mockAssist := &MockAssistant{
			TitleFunc: func(ctx context.Context, conv *model.Conversation) (string, error) {
				return "Some Title", nil
			},
			ReplyFunc: func(ctx context.Context, conv *model.Conversation) (string, error) {
				return "", errors.New("OpenAI service unavailable")
			},
		}

		srv := NewServer(model.New(ConnectMongo()), mockAssist)

		_, err := srv.StartConversation(ctx, &pb.StartConversationRequest{
			Message: "Hello!",
		})

		if err == nil {
			t.Fatal("expected error when reply fails, got nil")
		}
	}))
}
