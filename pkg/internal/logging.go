package internal

import (
	"fmt"
	"log"
)

const (
	fatal = iota
	fail
	warn
	info
	debug
	trace
)

var (
	loglevel = warn
)

// SetLogLevel set global log level
func SetLogLevel(level int) {
	loglevel = level
}

// LogInfo log in info level
func LogInfo(format string, v ...interface{}) {
	if loglevel >= info {
		log.Println(fmt.Sprintf("[ INFO ] "+format, v...))
	}
}

// LogDebug log in debug level
func LogDebug(format string, v ...interface{}) {
	if loglevel >= debug {
		log.Println(fmt.Sprintf("[ DEBUG ] "+format, v...))
	}
}

// LogWarn log in warn level
func LogWarn(format string, v ...interface{}) {
	if loglevel >= warn {
		log.Println(fmt.Sprintf("[ WARN ] "+format, v...))
	}
}

// LogTrace log in trace level
func LogTrace(format string, v ...interface{}) {
	if loglevel >= trace {
		log.Println(fmt.Sprintf("[ WARN ] "+format, v...))
	}
}
