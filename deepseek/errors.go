package deepseek

import "errors"

var DeepSeekNoMessagesErr = errors.New("No messages found in the request.")
var DeepSeekResponseErr = errors.New("DeepSeek returned error instead of response")
