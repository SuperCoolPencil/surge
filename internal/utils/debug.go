package utils

import (
	"fmt"
	"os"
	"sync"
	"time"
)

var (
	debugFile *os.File
	debugOnce sync.Once
)

// Debug writes a message to debug.log file
func Debug(format string, args ...any) {
	// add timestamp to each debug message
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	debugOnce.Do(func() {
		debugFile, _ = os.Create("debug.log")
	})
	if debugFile != nil {
		fmt.Fprintf(debugFile, "[%s] %s\n", timestamp, fmt.Sprintf(format, args...))
	}
}
