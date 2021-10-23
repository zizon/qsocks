package internal

import (
	"fmt"
	"log"
)

const (
	trace = iota
	debug
	info
	warn
)

var (
	loglevel = info
)

func init() {
	log.SetFlags(log.LstdFlags | log.Llongfile | log.Lmsgprefix)
}

// SetLogLevel set global log level
func SetLogLevel(level int) {
	loglevel = level
}

func LogWarn(format string, v ...interface{}) {
	if warn >= loglevel {
		format = fmt.Sprintf("[warn] %s", format)
		msg := fmt.Sprintf(format, v...)
		log.Output(2, msg)
	}
}

func LogInfo(format string, v ...interface{}) {
	if info >= loglevel {
		format = fmt.Sprintf("[info] %s", format)
		msg := fmt.Sprintf(format, v...)
		log.Output(2, msg)
	}
}

func LogDebug(format string, v ...interface{}) {
	if debug >= loglevel {
		format = fmt.Sprintf("[debug] %s", format)
		msg := fmt.Sprintf(format, v...)
		log.Output(2, msg)
	}
}

func LogTrace(format string, v ...interface{}) {
	if trace >= loglevel {
		format = fmt.Sprintf("[trace] %s", format)
		msg := fmt.Sprintf(format, v...)
		log.Output(2, msg)
	}
}
