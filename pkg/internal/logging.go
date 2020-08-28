package internal

import (
	"fmt"
	"log"
)

const (
	FATAL = iota
	ERROR
	WARN
	INFO
	DEBUG
)

var (
	loglevel = WARN
)

func SetLogLevel(level int) {
	loglevel = level
}

// LogInfo log in info level
func LogInfo(format string, v ...interface{}) {
	if loglevel >= INFO {
		log.Println(fmt.Sprintf("[ INFO ] "+format, v...))
	}
}

// LogDebug log in debug level
func LogDebug(format string, v ...interface{}) {
	if loglevel >= DEBUG {
		log.Println(fmt.Sprintf("[ DEBUG ] "+format, v...))
	}
}

// LogWarn log in warn level
func LogWarn(format string, v ...interface{}) {
	if loglevel >= WARN {
		log.Println(fmt.Sprintf("[ WARN ] "+format, v...))
	}
}
