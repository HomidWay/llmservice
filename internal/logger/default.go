package logger

import (
	"fmt"
	"io"
	"os"
	"time"
)

type DefaultLogger struct {
	verbosity Verbosity
	writers   []io.Writer
}

func Default(verbosity Verbosity, writers []io.Writer) Logger {
	if writers == nil {
		writers = []io.Writer{os.Stdout}
	}

	return &DefaultLogger{verbosity: verbosity, writers: writers}
}

func (l *DefaultLogger) Errorf(format string, args ...interface{}) {
	if l.verbosity >= VerbosityError {
		for _, writer := range l.writers {
			fmt.Fprintf(writer, "[%s] %7s: "+format+"\n", append([]interface{}{time.Now().Format(time.RFC3339), "ERROR"}, args...)...)
		}
	}
}

func (l *DefaultLogger) Infof(format string, args ...interface{}) {
	if l.verbosity >= VerbosityInfo {
		for _, writer := range l.writers {
			fmt.Fprintf(writer, "[%s] %7s: "+format+"\n", append([]interface{}{time.Now().Format(time.RFC3339), "INFO"}, args...)...)
		}
	}
}

func (l *DefaultLogger) Debugf(format string, args ...interface{}) {
	if l.verbosity >= VerbosityDebug {
		for _, writer := range l.writers {
			fmt.Fprintf(writer, "[%s] %7s: "+format+"\n", append([]interface{}{time.Now().Format(time.RFC3339), "DEBUG"}, args...)...)
		}
	}
}
