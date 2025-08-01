package deepseek

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/TitanLombard/llmservice"
	"github.com/TitanLombard/llmservice/internal/helpers"
	"github.com/TitanLombard/logger"
)

const baseURL = "https://api.deepseek.com/"

type DeepSeekAIService struct {
	apikey string
	ctx    context.Context
	log    logger.Logger
}

func NewDeepSeekService(apikey string, ctx context.Context, log logger.Logger) *DeepSeekAIService {

	if log == nil {
		log = logger.Default(logger.VerbosityInfo, nil)
	}

	return &DeepSeekAIService{apikey: apikey, ctx: ctx, log: log}
}

func (ds *DeepSeekAIService) ServiceTokenLimit() int {
	return 64000
}

func (ds *DeepSeekAIService) SendMessage(
	messages []llmservice.RequestMessage,
	options ...llmservice.Option,
) (chan string, error) {

	const requestURL string = baseURL + "chat/completions"

	returnChan := make(chan string)
	dsOptions := DeepSeekOptions{NewDeepSeekChatModel(), nil, nil, nil, nil, nil, nil, nil, nil}

	for _, option := range options {
		dsOption, ok := option.(DeepSeekOption)
		if ok && isValidOptionType(dsOption) {
			if err := dsOption.Apply(&dsOptions); err != nil {
				return returnChan, err
			}
		} else {
			return returnChan, helpers.NewInvalidOptionError((*DeepSeekOption)(nil), option)
		}
	}

	ds.log.Debugf("Options: \n%v", dsOptions)

	if len(messages) == 0 {
		return returnChan, deepSeekNoMessagesError{}
	}

	requestMessages := make([]networkRequestMessage, len(messages))

	for i := range messages {
		requestMessages[i] = networkRequestMessage{
			Role:    string(messages[i].Role()),
			Content: messages[i].Content(),
		}
	}

	var responseFormat *networkResponseFormat
	if dsOptions.responseFormat != nil {
		responseFormat = &networkResponseFormat{string(*dsOptions.responseFormat)}
	}

	request := networkRequest{
		Model:          string(dsOptions.model.Model()),
		Messages:       requestMessages,
		Streamed:       dsOptions.streamed,
		ResponseFormat: responseFormat,
		MaxTokens:      dsOptions.maxTokens,
		Temperature:    dsOptions.temperature,
		TopP:           dsOptions.topP,
		Logprobs:       dsOptions.logprobs,
		Tools:          dsOptions.tools,
		ToolChoice:     dsOptions.toolChoice,
	}

	requestBody, err := json.Marshal(request)

	if err != nil {
		return returnChan, err
	}

	ds.log.Debugf("DeepSeek request created")

	req, err := http.NewRequestWithContext(ds.ctx, "POST", requestURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return returnChan, err
	}

	authHeadervalue := "Bearer " + ds.apikey

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeadervalue)

	client := &http.Client{}

	ds.log.Debugf("DeepSeek request sent")
	resp, err := client.Do(req)
	if err != nil {
		return returnChan, err
	}

	go func(resp *http.Response, returnChan chan string) {
		ds.handleResponse(resp, returnChan)
	}(resp, returnChan)

	ds.log.Debugf("DeepSeek responce recieved")

	if resp.StatusCode != http.StatusOK {
		var dsErr deepSeekRequestError

		dsErr.Code = resp.StatusCode
		dsErr.Message = fmt.Sprintf("DeepSeek returned error: %s", resp.Status)

		var responseErr networkResponseError
		rawBody, _ := io.ReadAll(resp.Body)
		err := json.Unmarshal(rawBody, &responseErr)
		if err == nil {
			dsErr.ErrorBody = &responseErr
		}

		return returnChan, dsErr
	}

	ds.log.Debugf("DeepSeek rsponse Status: %s\n", resp.Status)
	return returnChan, nil
}

func (ds *DeepSeekAIService) handleResponse(resp *http.Response, outputChan chan<- string) {
	defer close(outputChan)
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {

		var chunk networkResponse

		data := strings.TrimPrefix(scanner.Text(), "data: ")

		if data == "[DONE]" {
			break
		}

		decoder := json.NewDecoder(strings.NewReader(data))

		err := decoder.Decode(&chunk)
		if err != nil && err != io.EOF {
			ds.log.Errorf("DeepSeek response chunk decode error: %s\n", err.Error())
			continue
		}

		for _, choice := range chunk.Choices {

			if choice.Delta != nil {
				outputChan <- choice.Delta.Content
			}

			if choice.Message != nil {
				outputChan <- choice.Message.Content
			}
		}
	}
}
