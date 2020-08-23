package internal

import (
	"fmt"
	"log"
)

// LogInfo log in info level
func LogInfo(format string, v ...interface{}) {
	log.Println(fmt.Sprintf("[ INFO ] "+format, v...))
}

// LogDebug log in debug level
func LogDebug(format string, v ...interface{}) {
	log.Println(fmt.Sprintf("[ DEBUG ] "+format, v...))
}

// LogWarn log in warn level
func LogWarn(format string, v ...interface{}) {
	log.Println(fmt.Sprintf("[ WARN ] "+format, v...))
}
