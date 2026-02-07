package logger

import (
	"fmt"
	"sync"
)

var loggerInstance Logger
var loggerInstanceInitialized bool = false
var once = &sync.Once{}
var instalceLock sync.RWMutex

func SetLoggerInstance(logger Logger) error {
	instalceLock.Lock()
	defer instalceLock.Unlock()

	if logger == loggerInstance {
		return nil
	}

	if loggerInstanceInitialized {
		return fmt.Errorf("logger instance already set or initialized")
	}

	loggerInstance = logger
	loggerInstanceInitialized = true

	return nil
}

func GetLoggerInstance() Logger {
	instalceLock.Lock()
	defer instalceLock.Unlock()

	once.Do(func() {
		if loggerInstance == nil {
			loggerInstance = Default(VerbosityInfo, nil)
		}
	})

	return loggerInstance
}
