package llmservice

import "context"

type SenderRole string

const (
	SenderRoleSystem    SenderRole = "system"
	SenderRoleUser      SenderRole = "user"
	SenderRoleAssistant SenderRole = "assistant"
)

// ResponseMessage represents a domain concept of a message in an AI conversation
type LLMMessage interface {
	Role() string
	ReasoningContent() *string
	MessageContent() string
	ToolCallID() string
	ToolCalls() []MessageToolCall
	StopReason() string
	TokenCount() int
}

type MessageToolCall interface {
	Index() int
	ID() string
	ToolName() string
	Args() string
}

type LLMService interface {
	SendMessage(context.Context, []LLMMessage, ...Option) (<-chan LLMMessage, error)
}
