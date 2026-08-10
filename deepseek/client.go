package deepseek

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/HomidWay/llmservice"
	"github.com/HomidWay/llmservice/internal/helpers"
)

var _ llmservice.LLMService = (*DeepSeekAIService)(nil)

const completionsURL = "https://api.deepseek.com/chat/completions"

type DeepSeekAIService struct {
	apikey string
}

func NewDeepSeekService(apikey string) *DeepSeekAIService {
	return &DeepSeekAIService{apikey: apikey}
}

func (ds *DeepSeekAIService) SendMessage(
	ctx context.Context,
	messages []llmservice.LLMMessage,
	options ...llmservice.Option,
) (<-chan llmservice.LLMMessage, error) {

	dsOptions := DeepSeekOptions{NewDeepSeekV4FlashModel(), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil}

	for _, option := range options {
		dsOption, ok := option.(DeepSeekOption)
		if ok && isValidOptionType(dsOption) {
			if err := dsOption.Apply(&dsOptions); err != nil {
				return nil, err
			}
		} else {
			return nil, helpers.NewInvalidOptionError((*DeepSeekOption)(nil), option)
		}
	}

	if len(messages) == 0 {
		return nil, DeepSeekNoMessagesErr
	}

	requestMessages := make([]DeepSeekMessage, len(messages))

	for i := range messages {
		switch messages[i].Role() {
		case string(llmservice.SenderRoleSystem):
			requestMessages[i] = *NewMessage(messages[i].Role(), messages[i].MessageContent(), messages[i].ToolCalls())
		case string(SenderRoleTool):
			requestMessages[i] = *NewToolCallResponse(messages[i].MessageContent(), messages[i].ToolCallID())
		default:
			requestMessages[i] = *NewMessage(messages[i].Role(), messages[i].MessageContent(), messages[i].ToolCalls())
		}
	}

	var responseFormat *DeepSeekResponseFormat
	if dsOptions.responseFormat != nil {
		responseFormat = &DeepSeekResponseFormat{string(*dsOptions.responseFormat)}
	}

	// Build MCP tool definitions
	var allTools []DeepSeekToolDefinition
	if dsOptions.tools != nil {
		allTools = append(allTools, *dsOptions.tools...)
	}

	var toolDefinitions *[]DeepSeekToolDefinition
	if len(allTools) > 0 {
		toolDefinitions = &allTools
	}

	request := DeepSeekCompletionCall{
		Model:           string(dsOptions.model.Model()),
		Messages:        requestMessages,
		Streamed:        dsOptions.streamed,
		ResponseFormat:  responseFormat,
		MaxTokens:       dsOptions.maxTokens,
		Temperature:     dsOptions.temperature,
		TopP:            dsOptions.topP,
		Logprobs:        dsOptions.logprobs,
		Tools:           toolDefinitions,
		ToolChoice:      dsOptions.toolChoice,
		Thinking:        dsOptions.thinking,
		ReasoningEffort: dsOptions.reasoningEffort,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("error marshalling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", completionsURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP request: %w", err)
	}

	authHeadervalue := "Bearer " + ds.apikey

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeadervalue)

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending HTTP request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {

		errDescription := "failed to get description"

		var responseErr networkResponseError
		rawBody, _ := io.ReadAll(resp.Body)
		err := json.Unmarshal(rawBody, &responseErr)
		if err == nil {
			errDescription = responseErr.Error.Message
		}

		return nil, errors.Join(DeepSeekResponseErr, fmt.Errorf("status code: %d. description: %s\n\nBody: %s", resp.StatusCode, errDescription, string(rawBody)))
	}

	returnChan := ds.handleResponse(ctx, resp.Body)

	return returnChan, nil
}

func (ds *DeepSeekAIService) handleResponse(ctx context.Context, body io.ReadCloser) <-chan llmservice.LLMMessage {

	outputChan := make(chan llmservice.LLMMessage)

	scanner := bufio.NewScanner(body)

	go func() {

		defer close(outputChan)

		for scanner.Scan() {

			select {
			case <-ctx.Done():
				return
			default:
			}

			data := strings.TrimPrefix(scanner.Text(), "data: ")

			if data == "[DONE]" {
				break
			}

			var chunk networkResponse
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				return
			}

			for _, choice := range chunk.Choices {
				if choice.Delta == nil && choice.Message == nil {
					return
				}

				if choice.Delta != nil {
					delta := choice.Delta
					delta.StopReasonString = choice.FinishReason
					outputChan <- delta
				}

				if choice.Message != nil {
					message := choice.Message
					message.StopReasonString = choice.FinishReason
					outputChan <- message
				}
			}
		}
	}()

	return outputChan
}
