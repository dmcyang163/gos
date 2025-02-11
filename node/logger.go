package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

	"github.com/iancoleman/orderedmap"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Logger interface {
	Debugf(format string, args ...interface{})
	Debug(args ...interface{})
	Infof(format string, args ...interface{})
	Info(args ...interface{})
	Warnf(format string, args ...interface{})
	Warn(args ...interface{})
	Errorf(format string, args ...interface{})
	Error(args ...interface{})
	WithFields(fields map[string]interface{}) Logger
	WithError(err error) Logger
}

type LogrusLogger struct {
	entry *logrus.Entry
}

var logFile = &lumberjack.Logger{
	Filename:   "log/node.log",
	MaxSize:    100, // Default values, will be overridden by config
	MaxBackups: 3,
	MaxAge:     28,
	Compress:   true,
}

type OrderedJSONFormatter struct {
	logrus.JSONFormatter
}

func (f *OrderedJSONFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	// 创建一个有序的 map
	data := orderedmap.New()

	data.Set("level", entry.Level.String())
	data.Set("timestamp", entry.Time.Format(f.TimestampFormat))
	data.Set("message", entry.Message)

	// 添加其他字段
	for key, value := range entry.Data {
		data.Set(key, value)
	}

	// 将有序的 map 转换为 JSON
	serialized, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal fields to JSON: %w", err)
	}

	return append(serialized, '\n'), nil
}

// AsyncHook 是一个自定义的 logrus Hook，用于异步处理日志
type AsyncHook struct {
	logChan chan *logrus.Entry // 用于传递日志的 Channel
	wg      sync.WaitGroup     // 用于等待 Goroutine 结束
}

// NewAsyncHook 创建一个新的 AsyncHook
func NewAsyncHook(bufferSize int) *AsyncHook {
	hook := &AsyncHook{
		logChan: make(chan *logrus.Entry, bufferSize),
	}
	hook.wg.Add(1)
	go hook.processLogs() // 启动 Goroutine 处理日志
	return hook
}

// Fire 实现 logrus.Hook 接口，将日志发送到 Channel
func (hook *AsyncHook) Fire(entry *logrus.Entry) error {
	hook.logChan <- entry
	return nil
}

// Levels 实现 logrus.Hook 接口，指定需要处理的日志级别
func (hook *AsyncHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// processLogs 处理日志，异步写入
func (hook *AsyncHook) processLogs() {
	defer hook.wg.Done()
	for entry := range hook.logChan {
		// 在这里实现日志的异步写入逻辑
		// 例如：写入文件、发送到远程服务器等
		data, err := entry.Logger.Formatter.Format(entry)
		if err != nil {
			fmt.Printf("Failed to format log entry: %v\n", err)
			continue
		}
		if _, err := entry.Logger.Out.Write(data); err != nil {
			fmt.Printf("Failed to write log entry: %v\n", err)
		}
	}
}

// Close 关闭 Hook，等待所有日志处理完成
func (hook *AsyncHook) Close() {
	close(hook.logChan) // 关闭 Channel
	hook.wg.Wait()      // 等待 Goroutine 结束
}

func NewLogrusLogger(config *Config) Logger {
	logger := logrus.New()

	// 配置日志轮转
	logFile.MaxSize = config.Log.MaxSize
	logFile.MaxBackups = config.Log.MaxBackups
	logFile.MaxAge = config.Log.MaxAge
	logFile.Compress = config.Log.Compress

	// 设置日志输出
	logger.SetOutput(io.MultiWriter(os.Stdout, logFile))

	// 使用自定义的 OrderedJSONFormatter
	logger.SetFormatter(&OrderedJSONFormatter{
		JSONFormatter: logrus.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
		},
	})

	// 设置日志级别
	switch config.Log.Level {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
	}

	// 添加异步 Hook
	asyncHook := NewAsyncHook(1000) // 缓冲区大小为 1000
	logger.AddHook(asyncHook)

	// 在程序退出时关闭 Hook
	go func() {
		<-make(chan struct{}) // 阻塞，直到程序退出
		asyncHook.Close()
	}()

	// 将 *logrus.Logger 转换为 *logrus.Entry
	entry := logrus.NewEntry(logger)
	return &LogrusLogger{entry: entry}
}

func (l *LogrusLogger) Debugf(format string, args ...interface{}) {
	l.entry.Debugf(format, args...)
}

func (l *LogrusLogger) Debug(args ...interface{}) {
	l.entry.Debug(args...)
}

func (l *LogrusLogger) Infof(format string, args ...interface{}) {
	l.entry.Infof(format, args...)
}

func (l *LogrusLogger) Info(args ...interface{}) {
	l.entry.Info(args...)
}

func (l *LogrusLogger) Warnf(format string, args ...interface{}) {
	l.entry.Warnf(format, args...)
}

func (l *LogrusLogger) Warn(args ...interface{}) {
	l.entry.Warn(args...)
}

func (l *LogrusLogger) Errorf(format string, args ...interface{}) {
	l.entry.Errorf(format, args...)
}

func (l *LogrusLogger) Error(args ...interface{}) {
	l.entry.Error(args...)
}

func (l *LogrusLogger) WithFields(fields map[string]interface{}) Logger {
	return &LogrusLogger{entry: l.entry.WithFields(fields)}
}

func (l *LogrusLogger) WithError(err error) Logger {
	return &LogrusLogger{entry: l.entry.WithError(err)}
}

// SetLogLevel dynamically sets the log level.
func SetLogLevel(logger Logger, level string) {
	logrusLogger, ok := logger.(*LogrusLogger)
	if !ok {
		return
	}

	var logLevel logrus.Level
	switch level {
	case "debug":
		logLevel = logrus.DebugLevel
	case "info":
		logLevel = logrus.InfoLevel
	case "warn":
		logLevel = logrus.WarnLevel
	case "error":
		logLevel = logrus.ErrorLevel
	default:
		logrusLogger.entry.WithFields(map[string]interface{}{"level": level}).Warn("Invalid log level")
		return
	}

	logrusLogger.entry.Logger.SetLevel(logLevel)
	logrusLogger.entry.WithFields(map[string]interface{}{"level": level}).Info("Log level changed")
}

// StartLogLevelAPI starts an HTTP server to dynamically adjust the log level.
func StartLogLevelAPI(logger Logger, config *Config) {
	http.HandleFunc("/loglevel", func(w http.ResponseWriter, r *http.Request) {
		level := r.URL.Query().Get("level")
		if level == "" {
			http.Error(w, "Missing level parameter", http.StatusBadRequest)
			return
		}
		SetLogLevel(logger, level)
		w.Write([]byte("Log level updated to " + level))
	})

	go http.ListenAndServe(":"+config.Log.APIPort, nil)
	logger.WithFields(map[string]interface{}{"port": config.Log.APIPort}).Info("Log level API started")
}
