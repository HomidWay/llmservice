package logger

type Verbosity int

const (
	VerbosityNone Verbosity = iota
	VerbosityError
	VerbosityInfo
	VerbosityDebug
)
