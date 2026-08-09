package deepseek

// MARK: - Interface

type DeepSeekModel interface {
	Model() string
	ContextTokenSize() int
}

// MARK: - V4 Flash model (DeepSeek-V4-Flash-0731)

type DeepSeekV4FlashModel struct{}

func NewDeepSeekV4FlashModel() DeepSeekV4FlashModel {
	return DeepSeekV4FlashModel{}
}

func (m DeepSeekV4FlashModel) Model() string {
	return "deepseek-v4-flash"
}

func (m DeepSeekV4FlashModel) ContextTokenSize() int {
	return 1_000_000
}

// MARK: - V4 Pro model (deepseek-v4-pro)

type DeepSeekV4ProModel struct{}

func NewDeepSeekV4ProModel() DeepSeekV4ProModel {
	return DeepSeekV4ProModel{}
}

func (m DeepSeekV4ProModel) Model() string {
	return "deepseek-v4-pro"
}

func (m DeepSeekV4ProModel) ContextTokenSize() int {
	return 1_000_000
}
