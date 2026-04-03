package core

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// Log 全局日志实例
var Log *Logger

// LogLevel 日志级别
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

// Logger 简易日志器
type Logger struct {
	level  LogLevel
	logger *log.Logger
}

// InitLogger 初始化全局日志
func InitLogger(level string) {
	l := &Logger{
		level:  parseLevel(level),
		logger: log.New(os.Stdout, "", 0),
	}
	Log = l
}

func parseLevel(s string) LogLevel {
	switch strings.ToLower(s) {
	case "debug":
		return DEBUG
	case "info":
		return INFO
	case "warn":
		return WARN
	case "error":
		return ERROR
	default:
		return INFO
	}
}

func (l *Logger) log(level LogLevel, prefix string, format string, args ...interface{}) {
	if level < l.level {
		return
	}
	ts := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf(format, args...)
	l.logger.Printf("%s [%s] %s", ts, prefix, msg)
}

func (l *Logger) Debug(args ...interface{})                 { l.log(DEBUG, "DEBUG", fmt.Sprint(args...)) }
func (l *Logger) Debugf(format string, args ...interface{}) { l.log(DEBUG, "DEBUG", format, args...) }
func (l *Logger) Info(args ...interface{})                  { l.log(INFO, "INFO", fmt.Sprint(args...)) }
func (l *Logger) Infof(format string, args ...interface{})  { l.log(INFO, "INFO", format, args...) }
func (l *Logger) Warn(args ...interface{})                  { l.log(WARN, "WARN", fmt.Sprint(args...)) }
func (l *Logger) Warnf(format string, args ...interface{})  { l.log(WARN, "WARN", format, args...) }
func (l *Logger) Error(args ...interface{})                 { l.log(ERROR, "ERROR", fmt.Sprint(args...)) }
func (l *Logger) Errorf(format string, args ...interface{}) { l.log(ERROR, "ERROR", format, args...) }
