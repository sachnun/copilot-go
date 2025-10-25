package logger

import (
	"fmt"
	"log"
	"os"
	"sync/atomic"
)

type Level int32

const (
	LevelError Level = iota
	LevelWarn
	LevelInfo
	LevelDebug
	LevelTrace
)

var currentLevel atomic.Int32

func init() {
	currentLevel.Store(int32(LevelInfo))
	log.SetOutput(os.Stdout)
	log.SetFlags(0)
}

func SetLevel(level Level) {
	currentLevel.Store(int32(level))
}

func Debug(msg string, args ...any) {
	logWithLevel(LevelDebug, "DEBUG", msg, args...)
}

func Trace(msg string, args ...any) {
	logWithLevel(LevelTrace, "TRACE", msg, args...)
}

func Info(msg string, args ...any) {
	logWithLevel(LevelInfo, "INFO", msg, args...)
}

func Warn(msg string, args ...any) {
	logWithLevel(LevelWarn, "WARN", msg, args...)
}

func Error(msg string, args ...any) {
	logWithLevel(LevelError, "ERROR", msg, args...)
}

func logWithLevel(level Level, prefix, msg string, args ...any) {
	if Level(currentLevel.Load()) < level {
		return
	}
	output := msg
	if len(args) > 0 {
		output = fmt.Sprintf(msg, args...)
	}
	log.Printf("[%s] %s", prefix, output)
}
