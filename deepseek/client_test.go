package deepseek_test

import (
	"context"
	"errors"
	"flag"
	"testing"
	"time"

	"github.com/TitanLombard/llmservice"
	"github.com/TitanLombard/llmservice/deepseek"
	"github.com/TitanLombard/llmservice/internal/helpers"
	"github.com/TitanLombard/logger"
)

var apiKeyFlag = flag.String("apikey", "", "DeepSeekApi key")

const (
	sysMessageContent  = "This is a test, make answer with 3 sentences."
	userMessageContent = "Say hi"
)

func TestSendMessage_streamed(t *testing.T) {

	t.Log("Streamed response test started")

	flag.Parse()

	if *apiKeyFlag == "" {
		t.Fatal("Skipping test - no API key provided")
		t.FailNow()
	}

	apikey := *apiKeyFlag

	log := logger.Default(logger.VerbosityDebug, nil)
	ctx := context.Background()
	deepSeek := deepseek.NewDeepSeekService(apikey, ctx, log)

	sysMessage, _ := deepseek.NewMessage(llmservice.SenderRoleSystem, sysMessageContent)
	usrMessage, _ := deepseek.NewMessage(llmservice.SenderRoleUser, userMessageContent)

	messages := []llmservice.RequestMessage{
		*sysMessage,
		*usrMessage,
	}

	returnChan, err := llmInterfaceCall(deepSeek, messages, deepseek.WithModel(deepseek.NewDeepSeekChatModel()), deepseek.WithStreamed(true))
	var invalidOption *helpers.InvalidOptionError
	if errors.As(err, &invalidOption) {
		t.Fatalf("Invalid option type: %s", invalidOption.Error())
		t.FailNow()
	}
	if err != nil {
		t.Errorf("Test failed due to error in deep seek request: %s", err.Error())
		t.FailNow()
	}

	for line := range returnChan {
		t.Log(line)
	}
}

func TestSendMessage_notstreamed(t *testing.T) {

	t.Log("Non-streamed response test started")

	flag.Parse()

	if *apiKeyFlag == "" {
		t.Fatal("Skipping test - no API key provided")
		t.FailNow()
	}

	apikey := *apiKeyFlag

	log := logger.Default(logger.VerbosityDebug, nil)
	ctx := context.Background()
	deepSeek := deepseek.NewDeepSeekService(apikey, ctx, log)

	sysMessage, _ := deepseek.NewMessage(llmservice.SenderRoleSystem, sysMessageContent)
	usrMessage, _ := deepseek.NewMessage(llmservice.SenderRoleUser, userMessageContent)

	messages := []llmservice.RequestMessage{
		*sysMessage,
		*usrMessage,
	}

	returnChan, err := llmInterfaceCall(deepSeek, messages, deepseek.WithModel(deepseek.NewDeepSeekChatModel()))
	var invalidOption *helpers.InvalidOptionError
	if errors.As(err, &invalidOption) {
		t.Fatalf("Invalid option type: %s", invalidOption.Error())
		t.FailNow()
	}
	if err != nil {
		t.Errorf("Test failed due to error in deep seek request: %s", err.Error())
		t.FailNow()
	}

	for line := range returnChan {
		t.Log(line)
	}
}

type randomOption struct{}

func (o randomOption) Apply(interface{}) error {
	return nil
}
func TestSendMessage_incorrectoption(t *testing.T) {

	t.Log("Invalid option type test started")

	apikey := ""

	ctx := context.Background()
	deepSeek := deepseek.NewDeepSeekService(apikey, ctx, nil)

	sysMessage, _ := deepseek.NewMessage(llmservice.SenderRoleSystem, sysMessageContent)
	usrMessage, _ := deepseek.NewMessage(llmservice.SenderRoleUser, userMessageContent)

	messages := []llmservice.RequestMessage{
		*sysMessage,
		*usrMessage,
	}

	_, err := llmInterfaceCall(deepSeek, messages, deepseek.WithModel(deepseek.NewDeepSeekChatModel()), randomOption{})
	var invalidOption *helpers.InvalidOptionError
	if errors.As(err, &invalidOption) {
		t.Errorf("Expected Invalid option type: %s", invalidOption.Error())
		return
	}

	t.FailNow()
}

func TestSendMessage_streamcancel(t *testing.T) {
	t.Log("Cancel of streamed response test started")

	flag.Parse()

	if *apiKeyFlag == "" {
		t.Fatal("Skipping test - no API key provided")
		t.FailNow()
	}

	apikey := *apiKeyFlag

	log := logger.Default(logger.VerbosityDebug, nil)
	ctx, cancel := context.WithCancel(context.Background())
	deepSeek := deepseek.NewDeepSeekService(apikey, ctx, log)

	sysMessage, _ := deepseek.NewMessage(llmservice.SenderRoleSystem, sysMessageContent)
	usrMessage, _ := deepseek.NewMessage(llmservice.SenderRoleUser, userMessageContent)

	messages := []llmservice.RequestMessage{
		*sysMessage,
		*usrMessage,
	}

	returnChan, err := llmInterfaceCall(deepSeek, messages, deepseek.WithModel(deepseek.NewDeepSeekChatModel()), deepseek.WithStreamed(true))
	var invalidOption *helpers.InvalidOptionError
	if errors.As(err, &invalidOption) {
		t.Fatalf("Invalid option type: %s", invalidOption.Error())
		t.FailNow()
	}
	if err != nil {
		t.Errorf("Test failed due to error in deep seek request: %s", err.Error())
		t.FailNow()
	}

	var cancelationTime time.Time

	go func() {
		time.Sleep(1 * time.Second)
		cancel()
		cancelationTime = time.Now()
	}()

	for line := range returnChan {
		t.Log(line)
	}

	if time.Since(cancelationTime) > (1 * time.Second) {
		t.Errorf("Failed to cancel task or cancelation took longer than 1 second. Canceled within: %v", time.Since(cancelationTime))
		t.FailNow()
	}
}

func TestSendMessage_nonstreamcancel(t *testing.T) {
	t.Log("Cancel of non-streamed response test started")

	flag.Parse()

	if *apiKeyFlag == "" {
		t.Fatal("Skipping test - no API key provided")
		t.FailNow()
	}

	apikey := *apiKeyFlag

	log := logger.Default(logger.VerbosityDebug, nil)
	ctx, cancel := context.WithCancel(context.Background())
	deepSeek := deepseek.NewDeepSeekService(apikey, ctx, log)

	sysMessage, _ := deepseek.NewMessage(llmservice.SenderRoleSystem, sysMessageContent)
	usrMessage, _ := deepseek.NewMessage(llmservice.SenderRoleUser, userMessageContent)

	messages := []llmservice.RequestMessage{
		*sysMessage,
		*usrMessage,
	}

	returnChan, err := llmInterfaceCall(deepSeek, messages, deepseek.WithModel(deepseek.NewDeepSeekChatModel()))
	var invalidOption *helpers.InvalidOptionError
	if errors.As(err, &invalidOption) {
		t.Fatalf("Invalid option type: %s", invalidOption.Error())
		t.FailNow()
	}
	if err != nil {
		t.Errorf("Test failed due to error in deep seek request: %s", err.Error())
		t.FailNow()
	}

	var cancelationTime time.Time

	go func() {
		time.Sleep(1 * time.Second)
		cancel()
		cancelationTime = time.Now()
	}()

	for line := range returnChan {
		t.Log(line)
	}

	if time.Since(cancelationTime) > (1 * time.Second) {
		t.Errorf("Failed to cancel task or cancelation took longer than 1 second. Canceled within: %v", time.Since(cancelationTime))
		t.FailNow()
	}
}

func llmInterfaceCall(llmservice llmservice.LLMService, messages []llmservice.RequestMessage, args ...llmservice.Option) (chan string, error) {
	return llmservice.SendMessage(messages, args...)
}
