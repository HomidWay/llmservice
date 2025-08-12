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
	"github.com/mark3labs/mcp-go/mcp"
)

const baseURL = "https://api.deepseek.com/"

type DeepSeekAIService struct {
	apikey         string
	ctx            context.Context
	log            logger.Logger
	mcpConnections []MCPConnection
}

func NewDeepSeekService(apikey string, ctx context.Context, log logger.Logger) *DeepSeekAIService {

	if log == nil {
		log = logger.Default(logger.VerbosityInfo, nil)
	}

	return &DeepSeekAIService{apikey: apikey, ctx: ctx, log: log}
}

func NewDeepSeekServiceWithMCP(apikey string, ctx context.Context, log logger.Logger, mcpConnections ...MCPConnection) (*DeepSeekAIService, error) {

	if log == nil {
		log = logger.Default(logger.VerbosityInfo, nil)
	}

	var initiatedConnections []MCPConnection

	for _, connection := range mcpConnections {
		err := connection.client.Start(ctx)
		if err != nil {
			return nil, err
		}

		_, err = connection.client.Initialize(ctx, mcp.InitializeRequest{})
		if err != nil {
			return nil, err
		}

		connection.resources, err = extractResources(connection)
		if err != nil {
			return nil, err
		}

		initiatedConnections = append(initiatedConnections, connection)
	}

	return &DeepSeekAIService{apikey: apikey, ctx: ctx, log: log, mcpConnections: initiatedConnections}, nil
}

func (ds *DeepSeekAIService) ServiceTokenLimit() int {
	return 64000
}

func (ds *DeepSeekAIService) SendMessage(
	messages []llmservice.LLMMessage,
	options ...llmservice.Option,
) (chan llmservice.LLMMessage, error) {

	const requestURL string = baseURL + "/chat/completions"

	returnChan := make(chan llmservice.LLMMessage)
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

	ds.debugPrint(dsOptions)

	if len(messages) == 0 {
		return returnChan, deepSeekNoMessagesError{}
	}

	mcpResources := ""

	for i, conn := range ds.mcpConnections {

		if i > 0 {
			mcpResources += "\n"
		}

		mcpResources += fmt.Sprintf("MCP[%d] Resources: %s", i, conn.resources)
	}

	requestMessages := make([]DeepSeekMessage, len(messages))

	for i := range messages {

		switch messages[i].Role() {
		case string(llmservice.SenderRoleSystem):
			messageContent := messages[i].MessageContent()

			if messages[i].Role() == string(llmservice.SenderRoleSystem) && mcpResources != "" {
				messageContent += fmt.Sprintf("\n\n Available MCP resources: %s", mcpResources)
			}

			requestMessages[i] = *NewMessage(messages[i].Role(), messageContent, messages[i].ToolCalls())
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

	request := DeepSeekCompletionCall{
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
		return returnChan, fmt.Errorf("error marshalling request: %s", err)
	}

	ds.log.Infof("DeepSeek request created")
	ds.log.Debugf("%s", string(requestBody))

	req, err := http.NewRequestWithContext(ds.ctx, "POST", requestURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return returnChan, fmt.Errorf("Error creating HTTP request: %s", err)
	}

	authHeadervalue := "Bearer " + ds.apikey

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeadervalue)

	client := &http.Client{}

	ds.log.Debugf("DeepSeek request sent")
	resp, err := client.Do(req)
	if err != nil {
		return returnChan, fmt.Errorf("Error sending HTTP request: %s", err)
	}

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

		ds.log.Debugf("HTML Body:\n%s", string(rawBody))
		return returnChan, fmt.Errorf("Error sending HTTP request: %s", dsErr)
	}

	ds.log.Debugf("DeepSeek response Status: %s\n", resp.Status)

	go func(ds *DeepSeekAIService, readCloser io.ReadCloser, outputChan chan<- llmservice.LLMMessage) {
		err := ds.handleResponse(readCloser, outputChan)
		if err != nil {
			ds.log.Errorf("Error handling DeepSeek response: %s", err.Error())
		}
	}(ds, resp.Body, returnChan)

	return returnChan, nil
}

func (ds *DeepSeekAIService) HandleToolCall(toolCalls []llmservice.MessageToolCall) ([]llmservice.LLMMessage, error) {

	messages := make([]llmservice.LLMMessage, len(toolCalls))

	for i, toolCall := range toolCalls {
		if toolCall.ToolName() == ResourceCallTool().Function.Name {

			var resourceCall ResourceCall
			err := json.Unmarshal([]byte(toolCall.Args()), &resourceCall)
			if err != nil {
				messages[i] = NewToolCallResponse(
					fmt.Sprintf("Incorrect argument json. Expected {mcp_id: <Int>, uri: <String>}; Got: %s, err: %s", toolCall.Args(), err.Error()),
					toolCall.ID(),
				)
				continue
			}

			if len(ds.mcpConnections)-1 < resourceCall.McpIndex {
				return nil, fmt.Errorf("invalid mcp_index. MCP connections count is %d; Got: %d", len(ds.mcpConnections), resourceCall.McpIndex)
			}

			resourceReadReq := mcp.ReadResourceRequest{
				Params: mcp.ReadResourceParams{
					URI: resourceCall.URI,
				},
			}

			result, err := ds.mcpConnections[resourceCall.McpIndex].client.ReadResource(ds.ctx, resourceReadReq)
			if err != nil {
				messages[i] = NewToolCallResponse(
					fmt.Sprintf("MCP Resource read failed with error: %s", err.Error()),
					toolCall.ID(),
				)
				continue
			}

			textData := ""

			for _, chunk := range result.Contents {
				switch v := chunk.(type) {
				case mcp.TextResourceContents:
					textData += v.Text
				case mcp.BlobResourceContents:
					textData += v.Blob
				}
			}

			messages[i] = NewToolCallResponse(
				textData,
				toolCall.ID(),
			)

			continue
		} else {
			messages[i] = NewToolCallResponse(fmt.Sprintf("Unsupported tool: %s", toolCall.ToolName()), toolCall.ID())
		}
	}

	return messages, nil
}

func (ds *DeepSeekAIService) handleResponse(readCloser io.ReadCloser, outputChan chan<- llmservice.LLMMessage) error {
	defer close(outputChan)
	scanner := bufio.NewScanner(readCloser)

	for scanner.Scan() {

		bodyText := scanner.Text()

		ds.log.Debugf("DeepSeek Response:\n%s", bodyText)

		var chunk networkResponse

		data := strings.TrimPrefix(bodyText, "data: ")

		if data == "[DONE]" {
			break
		}

		decoder := json.NewDecoder(strings.NewReader(data))

		err := decoder.Decode(&chunk)
		if err != nil && err != io.EOF {
			ds.log.Debugf("DeepSeek response chunk data: %s\n", data)
			return fmt.Errorf("deepseek response chunk decode error: %s", err.Error())
		}

		for _, choice := range chunk.Choices {

			if choice.Delta == nil && choice.Message == nil {
				return fmt.Errorf("no messages found in the request")
			}

			if choice.Delta != nil {
				if choice.FinishReason != nil {
					fmt.Printf("Delta finish Reason: %s\n", *choice.FinishReason)
				}

				delta := choice.Delta

				delta.StopReasonString = choice.FinishReason

				outputChan <- delta
			}

			if choice.Message != nil {
				if choice.FinishReason != nil {
					fmt.Printf("Message finish Reason: %s\n", *choice.FinishReason)
				}

				message := choice.Message
				message.StopReasonString = choice.FinishReason

				outputChan <- message
			}
		}
	}

	return nil
}

func (ds DeepSeekAIService) debugPrint(dsOptions DeepSeekOptions) {
	ds.log.Debugf("Options Details:")
	ds.log.Debugf("- Model: %v", dsOptions.model)
	if dsOptions.responseFormat != nil {
		ds.log.Debugf("- ResponseFormat: %v", *dsOptions.responseFormat)
	} else {
		ds.log.Debugf("- ResponseFormat: nil")
	}
	ds.log.Debugf("- Streamed: %v", dsOptions.streamed)
	ds.log.Debugf("- MaxTokens: %v", dsOptions.maxTokens)
	ds.log.Debugf("- Temperature: %v", dsOptions.temperature)
	ds.log.Debugf("- TopP: %v", dsOptions.topP)
	ds.log.Debugf("- Logprobs: %v", dsOptions.logprobs)
	if dsOptions.tools != nil {
		toolsJson, _ := json.Marshal(dsOptions.tools)
		ds.log.Debugf("- Tools: %s", string(toolsJson))
	} else {
		ds.log.Debugf("- Tools: nil")
	}
	ds.log.Debugf("- ToolChoice: %v", dsOptions.toolChoice)
}

func extractResources(mcpConnection MCPConnection) (string, error) {

	stringBuilder := strings.Builder{}

	resources, err := mcpConnection.client.ListResources(context.Background(), mcp.ListResourcesRequest{})
	if err != nil {
		return "", fmt.Errorf("failed to list resources: %s", err.Error())
	}

	resourceTemplates, err := mcpConnection.client.ListResourceTemplates(context.Background(), mcp.ListResourceTemplatesRequest{})
	if err != nil {
		return "", fmt.Errorf("failed to list resource templates: %s", err.Error())
	}

	if len(resources.Resources) > 0 {
		stringBuilder.WriteString("Resources:\n")
		for i, resource := range resources.Resources {
			if i > 0 {
				stringBuilder.WriteString("\n")
			}
			jsonString, err := json.Marshal(resource)
			if err != nil {
				return "", fmt.Errorf("failed to marshal resource: %s", err.Error())
			}

			stringBuilder.WriteString(string(jsonString))
		}
	}

	if len(resourceTemplates.ResourceTemplates) > 0 {
		stringBuilder.WriteString("ResourceTemplates:\n")
		for i, networkResponse := range resourceTemplates.ResourceTemplates {
			if i > 0 {
				stringBuilder.WriteString("\n")
			}
			jsonString, err := json.Marshal(networkResponse)
			if err != nil {
				return "", fmt.Errorf("failed to marshal resource template: %s", err.Error())
			}

			stringBuilder.WriteString(string(jsonString))
		}
	}

	return stringBuilder.String(), nil
}
