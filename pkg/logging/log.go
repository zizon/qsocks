package logging

import (
	"fmt"
	"log"
)

const (
	trace = iota
	debug
	info
	warn
	error
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

func Error(format string, v ...any) {
	if error >= loglevel {
		format = fmt.Sprintf("[error] %s", format)
		msg := fmt.Sprintf(format, v...)
		log.Output(2, msg)
	}
}

func Warn(format string, v ...any) {
	if warn >= loglevel {
		format = fmt.Sprintf("[warn] %s", format)
		msg := fmt.Sprintf(format, v...)
		log.Output(2, msg)
	}
}

func Info(format string, v ...any) {
	if info >= loglevel {
		format = fmt.Sprintf("[info] %s", format)
		msg := fmt.Sprintf(format, v...)
		log.Output(2, msg)
	}
}

func Debug(format string, v ...any) {
	if debug >= loglevel {
		format = fmt.Sprintf("[debug] %s", format)
		msg := fmt.Sprintf(format, v...)
		log.Output(2, msg)
	}
}

func Trace(format string, v ...any) {
	if trace >= loglevel {
		format = fmt.Sprintf("[trace] %s", format)
		msg := fmt.Sprintf(format, v...)
		log.Output(2, msg)
	}
}
