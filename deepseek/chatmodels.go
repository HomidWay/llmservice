package deepseek

// MARK: - Interface

type DeepSeekModel interface {
	Model() string
	ContextTokenSize() int
}

// MARK: - Chat model

type DeepSeekChatModelType struct{}

func NewDeepSeekChatModel() DeepSeekChatModelType {
	return DeepSeekChatModelType{}
}

func (m DeepSeekChatModelType) Model() string {
	return "deepseek-chat"
}

func (m DeepSeekChatModelType) ContextTokenSize() int {
	return 64000
}

// MARK: - Reasoner model

type DeepSeekResonerModelType struct{}

func NewDeepSeekReasonerModel() DeepSeekResonerModelType {
	return DeepSeekResonerModelType{}
}

func (m DeepSeekResonerModelType) Model() string {
	return "deepseek-reasoner"
}

func (m DeepSeekResonerModelType) ContextTokenSize() int {
	return 64000
}
