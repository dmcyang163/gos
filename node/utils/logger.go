package utils

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

// 默认日志配置
var defaultLogConfig = LogConfig{
	Level:      "info",
	MaxSize:    100,
	MaxBackups: 3,
	MaxAge:     28,
	Compress:   true,
	APIPort:    "8081",
}

type LogConfig struct {
	Level      string `json:"level"`
	MaxSize    int    `json:"max_size"`
	MaxBackups int    `json:"max_backups"`
	MaxAge     int    `json:"max_age"`
	Compress   bool   `json:"compress"`
	APIPort    string `json:"api_port"`
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

// AsyncWriter 是一个异步的 io.Writer，用于异步处理日志输出
type AsyncWriter struct {
	logChan chan *logrus.Entry // 用于传递日志数据的 Channel
	wg      sync.WaitGroup     // 用于等待 Goroutine 结束
	output  io.Writer          // 实际输出目标（如文件或控制台）
	logger  *logrus.Logger     // 保存 logger 实例
}

// NewAsyncWriter 创建一个新的 AsyncWriter
func NewAsyncWriter(bufferSize int, output io.Writer, logger *logrus.Logger) *AsyncWriter {
	writer := &AsyncWriter{
		logChan: make(chan *logrus.Entry, bufferSize),
		output:  output,
		logger:  logger, // 保存 logger
	}
	writer.wg.Add(1)
	go writer.processLogs() // 启动 Goroutine 处理日志
	return writer
}

// Write 实现 io.Writer 接口，将日志数据发送到 Channel
func (w *AsyncWriter) Write(p []byte) (int, error) {
	// 将 []byte 转换为 logrus.Entry
	var entry logrus.Entry
	err := json.Unmarshal(p, &entry)
	if err != nil {
		fmt.Printf("Failed to unmarshal log entry: %v\n", err)
		return 0, err
	}

	// 创建一个新的 Entry，并使用原始数据
	newEntry := logrus.NewEntry(w.logger) // 使用保存的 logger
	newEntry.Level = entry.Level
	newEntry.Time = entry.Time
	newEntry.Message = entry.Message
	newEntry.Data = entry.Data

	w.logChan <- newEntry
	return len(p), nil
}

// processLogs 处理日志，异步写入
func (w *AsyncWriter) processLogs() {
	defer w.wg.Done()
	for entry := range w.logChan {
		// 将日志写入到实际输出目标
		data, err := entry.Logger.Formatter.Format(entry)
		if err != nil {
			fmt.Printf("Failed to format log entry: %v\n", err)
			continue
		}

		if _, err := w.output.Write(data); err != nil {
			fmt.Printf("Failed to write log entry: %v\n", err)
		}

		// 检查日志级别，发送告警
		if entry.Level == logrus.ErrorLevel {
			sendAlert(entry.Message) // 发送告警
		}
	}
}

// Close 关闭 AsyncWriter，等待所有日志处理完成
func (w *AsyncWriter) Close() {
	close(w.logChan) // 关闭 Channel
	w.wg.Wait()      // 等待 Goroutine 结束
}

func sendAlert(message string) {
	// 实现告警逻辑，例如发送邮件或调用 Webhook
	fmt.Printf("ALERT: %s\n", message)
}

func configureLogger(output io.Writer, config *LogConfig) *logrus.Logger {
	logger := logrus.New()

	formatter := &OrderedJSONFormatter{
		JSONFormatter: logrus.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
		},
	}
	logger.SetFormatter(formatter)

	// 使用 AsyncWriter 包装实际的输出目标
	asyncWriter := NewAsyncWriter(1000, output, logger) // 传递 logger
	logger.SetOutput(asyncWriter)

	switch config.Level {
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

	// 在程序退出时关闭 AsyncWriter
	go func() {
		<-make(chan struct{}) // 阻塞，直到程序退出
		asyncWriter.Close()
	}()

	return logger
}

// NewLogrusLogger 创建一个新的 LogrusLogger 实例
func NewLogrusLogger(filename string, config *LogConfig) Logger {
	if config == nil {
		config = &defaultLogConfig // 使用默认配置
	}

	logFile := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    config.MaxSize,
		MaxBackups: config.MaxBackups,
		MaxAge:     config.MaxAge,
		Compress:   config.Compress,
	}

	// 使用 AsyncWriter 包装日志输出目标
	logger := configureLogger(
		io.MultiWriter(os.Stdout, logFile), // 同时输出到控制台和文件
		config,
	)
	entry := logrus.NewEntry(logger)
	return &LogrusLogger{entry: entry}
}

// NewChatLogger 创建一个新的 ChatLogger 实例
func NewChatLogger(filename string, config *LogConfig) Logger {
	if config == nil {
		config = &defaultLogConfig // 使用默认配置
	}

	chatLogFile := &lumberjack.Logger{
		Filename:   filename, // 聊天日志文件名
		MaxSize:    config.MaxSize,
		MaxBackups: config.MaxBackups,
		MaxAge:     config.MaxAge,
		Compress:   config.Compress,
	}

	// 使用 AsyncWriter 包装日志输出目标
	logger := configureLogger(
		chatLogFile,
		config,
	)
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
func StartLogLevelAPI(logger Logger) {
	http.HandleFunc("/loglevel", func(w http.ResponseWriter, r *http.Request) {
		level := r.URL.Query().Get("level")
		if level == "" {
			http.Error(w, "Missing level parameter", http.StatusBadRequest)
			return
		}
		SetLogLevel(logger, level)
		w.Write([]byte("Log level updated to " + level))
	})

	go http.ListenAndServe(":"+defaultLogConfig.APIPort, nil)
	logger.WithFields(map[string]interface{}{"port": defaultLogConfig.APIPort}).Info("Log level API started")
}
