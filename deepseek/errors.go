package deepseek

type deepSeekRequestError struct {
	Code      int
	Message   string
	ErrorBody *networkResponseError
}

type deepSeekNoMessagesError struct{}

func (e deepSeekRequestError) Error() string {
	return e.Message
}

func (deepSeekNoMessagesError) Error() string {
	return "No messages found in the request."
}
