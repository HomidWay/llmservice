package llmservice

type Option interface {
	Apply(interface{}) error
}
