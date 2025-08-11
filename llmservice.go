package llmservice

type SenderRole string

const (
	SenderRoleSystem    SenderRole = "system"
	SenderRoleUser      SenderRole = "user"
	SenderRoleAssistant SenderRole = "assistant"
)

type RequestMessage interface {
	Role() string
	Content() string
	ToolCalls() []MessageToolCall
	TokenCount() int
}

// ResponseMessage represents a domain concept of a message in an AI conversation
type ResponseMessage interface {
	Role() string
	ReasoningContent() *string
	MessageContent() string
	ToolCalls() []MessageToolCall
	TokenCount() int
}

type MessageToolCall interface {
	Index() int
	ID() string
	ToolName() string
	Args() string
}

type LLMService interface {
	SendMessage([]RequestMessage, ...Option) (chan ResponseMessage, error)
	ServiceTokenLimit() int
}

type LLMServiceMCP interface {
	LLMService
	HandleToolCall([]MessageToolCall) (RequestMessage, error)
}
