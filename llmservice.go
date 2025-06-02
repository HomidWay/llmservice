package llmservice

type SenderRole string

const (
	SenderRoleSystem    SenderRole = "system"
	SenderRoleUser      SenderRole = "user"
	SenderRoleAssistant SenderRole = "assistant"
)

// RequestMessage represents a domain concept of a message in an AI conversation
type RequestMessage interface {
	Role() SenderRole
	Content() string
	TokenCount() int
}

type LLMService interface {
	SendMessage([]RequestMessage, ...Option) (chan string, error)
	ServiceTokenLimit() int
}
