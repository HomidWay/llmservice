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

	const requestURL string = baseURL + "/chat/completions"

	returnChan := make(chan string)
	dsOptions := DeepSeekOptions{NewDeepSeekChatModel(), nil, nil, nil, nil, nil, nil}

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
		responseFormat = newNetworkResponseFormat(string(*dsOptions.responseFormat))
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

	go func(isStreamed *bool, resp *http.Response, returnChan chan string) {
		if isStreamed == nil {
			ds.handleMessageJSONResponse(resp, returnChan)
			return
		}

		if !*isStreamed {
			ds.handleMessageJSONResponse(resp, returnChan)
		} else {
			ds.handleStreamedJSONResponse(resp, returnChan)
		}
	}(dsOptions.streamed, resp, returnChan)

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

func (c *DeepSeekAIService) handleStreamedJSONResponse(resp *http.Response, outputChan chan<- string) {
	defer close(outputChan)
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	for {
		select {
		case <-c.ctx.Done():
			c.log.Debugf("Stream canceled")
			return
		default:
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					c.log.Errorf("Stream read error", "error", err)
				}
				return
			}

			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				if data == "[DONE]" {
					return
				}

				var chunk networkResponse
				err := json.Unmarshal([]byte(data), &chunk)
				if err != nil {
					c.log.Debugf("Failed to unmarshal delta chunk: %s", err.Error())
					break
				}

				for _, choice := range chunk.Choices {
					outputChan <- choice.Delta.Content
				}
			}
		}
	}
}

func (c *DeepSeekAIService) handleMessageJSONResponse(resp *http.Response, outputChan chan<- string) {
	defer close(outputChan)
	defer resp.Body.Close()

	var responce networkResponse

	jsonDecoder := json.NewDecoder(resp.Body)
	jsonDecoder.Decode(&responce)

	select {
	case <-c.ctx.Done():
		return
	default:
		outputChan <- responce.Choices[0].Message.Content
	}
}
