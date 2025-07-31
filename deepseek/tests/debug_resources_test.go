package deepseek_test

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"testing"

	"github.com/TitanLombard/llmservice"
	"github.com/TitanLombard/llmservice/deepseek"
	"github.com/TitanLombard/llmservice/internal/helpers"
	"github.com/TitanLombard/logger"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

var apiKeyFlag = flag.String("apikey", "", "DeepSeekApi key")

const (
	sysMessageContent  = "This is a test for MCP client."
	userMessageContent = "Напиши короткую сводку данных о сотруднике с ID 5560 с его показателями за неделю за неделю"
)

func TestDebugMCPServerEndpoints(t *testing.T) {

	client, err := client.NewSSEMCPClient("http://127.0.0.1:8770/sse")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	t.Log("Client connected")

	err = client.Start(context.Background())
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	t.Log("Client started")

	initial, err := client.Initialize(context.Background(), mcp.InitializeRequest{})
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	t.Logf("%v", initial)

	resourceTemplates, err := client.ListResourceTemplates(context.Background(), mcp.ListResourceTemplatesRequest{})
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	t.Logf("%v", resourceTemplates)

	resources, err := client.ListResources(context.Background(), mcp.ListResourcesRequest{})
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	t.Logf("%v", resources)
}

func TestSendMessage_streamed(t *testing.T) {

	mcpConnection, err := deepseek.WithSSEMCP("http://127.0.0.1:8770/sse")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	t.Log("Streamed response test started")

	flag.Parse()

	if *apiKeyFlag == "" {
		t.Fatal("Skipping test - no API key provided")
		t.FailNow()
	}

	apikey := *apiKeyFlag

	log := logger.Default(logger.VerbosityDebug, nil)
	ctx := context.Background()
	deepSeek := deepseek.NewDeepSeekServiceWithMCP(apikey, ctx, log, mcpConnection)

	sysMessage, _ := deepseek.NewMessage(llmservice.SenderRoleSystem, sysMessageContent)
	usrMessage, _ := deepseek.NewMessage(llmservice.SenderRoleUser, userMessageContent)

	messages := []llmservice.RequestMessage{
		*sysMessage,
		*usrMessage,
	}

	json, _ := json.Marshal([]deepseek.ToolDefinition{deepseek.ResourceCallTool()})
	t.Log(string(json))

	returnChan, err := llmInterfaceCall(
		deepSeek,
		messages,
		deepseek.WithModel(deepseek.NewDeepSeekChatModel()),
		deepseek.WithStreamed(true),
		deepseek.WithTools([]deepseek.ToolDefinition{deepseek.ResourceCallTool()}),
	)

	var invalidOption *helpers.InvalidOptionError
	if errors.As(err, &invalidOption) {
		t.Fatalf("Invalid option type: %s", invalidOption.Error())
		t.FailNow()
	}
	if err != nil {
		t.Errorf("Test failed due to error in deep seek request: %s", err.Error())
		t.FailNow()
	}

	fmt.Println("============ RESPONSE ==================")
	for line := range returnChan {
		fmt.Print(line)
	}
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println()
}

func llmInterfaceCall(llmservice llmservice.LLMService, messages []llmservice.RequestMessage, args ...llmservice.Option) (chan string, error) {
	return llmservice.SendMessage(messages, args...)
}
