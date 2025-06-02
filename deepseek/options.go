package deepseek

import "github.com/TitanLombard/llmservice/internal/helpers"

// MARK: - Concrete options

type DeepSeekOptions struct {
	model          DeepSeekModel
	streamed       *bool
	responseFormat *DeepSeekResponceFormat
	maxTokens      *int
	temperature    *float32
	topP           *float32
	logprobs       *bool
}

// MARK: - Option setters interface

type DeepSeekOption interface {
	Apply(interface{}) error
}

// MARK - Model option setters

type DeepSeekOptionModel struct {
	model DeepSeekModel
}

func (opt DeepSeekOptionModel) applyFunc(dsOpt *DeepSeekOptions) {
	dsOpt.model = opt.model
}

func (o *DeepSeekOptionModel) Apply(option interface{}) error {
	if deepSeekOption, ok := option.(*DeepSeekOptions); ok {
		o.applyFunc(deepSeekOption)
	} else {
		return helpers.NewInvalidOptionError((*DeepSeekOption)(nil), option)
	}

	return nil
}

func WithModel(model DeepSeekModel) *DeepSeekOptionModel {
	return &DeepSeekOptionModel{model: model}
}

// MARK - Streamed option setters

type DeepSeekOptionStreamed struct {
	streamed bool
}

func (opt DeepSeekOptionStreamed) applyFunc(dsOpt *DeepSeekOptions) {
	dsOpt.streamed = &opt.streamed
}

func (o *DeepSeekOptionStreamed) Apply(option interface{}) error {
	if deepSeekOption, ok := option.(*DeepSeekOptions); ok {
		o.applyFunc(deepSeekOption)
	} else {
		return helpers.NewInvalidOptionError((*DeepSeekOptions)(nil), option)
	}

	return nil
}

func WithStreamed(isStreamed bool) *DeepSeekOptionStreamed {
	return &DeepSeekOptionStreamed{streamed: isStreamed}
}

// MARK - ResponseFormat option setters

type DeepSeekResponceFormat string

const (
	ResponceFormatText DeepSeekResponceFormat = "text"
	ResponceFormatJson DeepSeekResponceFormat = "json_object"
)

type DeepSeekOptionResponseFormat struct {
	responseFormat DeepSeekResponceFormat
}

func (opt DeepSeekOptionResponseFormat) applyFunc(dsOpt *DeepSeekOptions) {
	dsOpt.responseFormat = &opt.responseFormat
}

func (o *DeepSeekOptionResponseFormat) Apply(option interface{}) error {

	if option == nil {
		return helpers.NewInvalidOptionError((*DeepSeekOptions)(nil), nil)
	}

	deepSeekOption, ok := option.(*DeepSeekOptions)
	if !ok {
		return helpers.NewInvalidOptionError((*DeepSeekOption)(nil), option)
	}

	o.applyFunc(deepSeekOption)

	return nil
}

func WithResponceFormat(responseFormat DeepSeekResponceFormat) *DeepSeekOptionResponseFormat {
	return &DeepSeekOptionResponseFormat{responseFormat: responseFormat}
}

// MARK - MaxTokens option setters

type DeepSeekOptionMaxTokens struct {
	maxTokens int
}

func (opt DeepSeekOptionMaxTokens) applyFunc(dsOpt *DeepSeekOptions) {
	dsOpt.maxTokens = &opt.maxTokens
}

func (o *DeepSeekOptionMaxTokens) Apply(option interface{}) error {
	if deepSeekOption, ok := option.(*DeepSeekOptions); ok {
		o.applyFunc(deepSeekOption)
	} else {
		return helpers.NewInvalidOptionError((*DeepSeekOption)(nil), option)
	}

	return nil
}

func WithMaxResponceTokens(maxTokens int) *DeepSeekOptionMaxTokens {
	return &DeepSeekOptionMaxTokens{maxTokens: maxTokens}
}

// MARK - Temperature option setters

type DeepSeekOptionTemperature struct {
	temperature float32
}

func (opt DeepSeekOptionTemperature) applyFunc(dsOpt *DeepSeekOptions) {
	dsOpt.temperature = &opt.temperature
}

func (o *DeepSeekOptionTemperature) Apply(option interface{}) error {
	if deepSeekOption, ok := option.(*DeepSeekOptions); ok {
		o.applyFunc(deepSeekOption)
	} else {
		return helpers.NewInvalidOptionError((*DeepSeekOption)(nil), option)
	}

	return nil
}

func WithTemperature(temperature float32) *DeepSeekOptionTemperature {
	return &DeepSeekOptionTemperature{temperature: temperature}
}

// MARK - TopP option setters

type DeepSeekOptionTopP struct {
	topP float32
}

func (opt DeepSeekOptionTopP) applyFunc(dsOpt *DeepSeekOptions) {
	dsOpt.topP = &opt.topP
}

func (o *DeepSeekOptionTopP) Apply(option interface{}) error {
	if deepSeekOption, ok := option.(*DeepSeekOptions); ok {
		o.applyFunc(deepSeekOption)
	} else {
		return helpers.NewInvalidOptionError((*DeepSeekOption)(nil), option)
	}

	return nil
}

func WithTopP(topP float32) *DeepSeekOptionTopP {
	return &DeepSeekOptionTopP{topP: topP}
}

// MARK - Logprobs option setters

type DeepSeekOptionLogprobs struct {
	logprobs bool
}

func (opt DeepSeekOptionLogprobs) applyFunc(dsOpt *DeepSeekOptions) {
	dsOpt.logprobs = &opt.logprobs
}

func (o *DeepSeekOptionLogprobs) Apply(option interface{}) error {
	if deepSeekOption, ok := option.(*DeepSeekOptions); ok {
		o.applyFunc(deepSeekOption)
	} else {
		return helpers.NewInvalidOptionError((*DeepSeekOption)(nil), option)
	}

	return nil
}

func WithLogprobs(logprobs bool) *DeepSeekOptionLogprobs {
	return &DeepSeekOptionLogprobs{logprobs: logprobs}
}

func isValidOptionType(opt DeepSeekOption) bool {
	switch opt.(type) {
	case *DeepSeekOptionModel,
		*DeepSeekOptionStreamed,
		*DeepSeekOptionResponseFormat,
		*DeepSeekOptionMaxTokens,
		*DeepSeekOptionTemperature,
		*DeepSeekOptionTopP,
		*DeepSeekOptionLogprobs:
		return true
	default:
		return false
	}
}
