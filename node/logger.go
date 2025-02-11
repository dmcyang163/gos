package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

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
func NewLogrusLogger(config *Config) Logger {
	logger := logrus.New()

	// 配置日志轮转
	logFile.MaxSize = config.Log.MaxSize
	logFile.MaxBackups = config.Log.MaxBackups
	logFile.MaxAge = config.Log.MaxAge
	logFile.Compress = config.Log.Compress

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

	switch level {
	case "debug":
		logrusLogger.entry.Logger.SetLevel(logrus.DebugLevel)
	case "info":
		logrusLogger.entry.Logger.SetLevel(logrus.InfoLevel)
	case "warn":
		logrusLogger.entry.Logger.SetLevel(logrus.WarnLevel)
	case "error":
		logrusLogger.entry.Logger.SetLevel(logrus.ErrorLevel)
	default:
		logrusLogger.entry.Logger.SetLevel(logrus.InfoLevel)
	}
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
