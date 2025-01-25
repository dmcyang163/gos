// logger.go
package main

import (
	"io"
	"net/http"
	"os"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger 是日志模块的接口
type Logger interface {
	Debugf(format string, args ...interface{})
	Debug(args ...interface{}) // 新增 Debug 方法
	Infof(format string, args ...interface{})
	Info(args ...interface{})
	Warnf(format string, args ...interface{})
	Warn(args ...interface{})
	Errorf(format string, args ...interface{})
	Error(args ...interface{})
	WithFields(fields map[string]interface{}) Logger
	WithError(err error) Logger
}

// LogrusLogger 是 logrus.Logger 的封装，实现 Logger 接口
type LogrusLogger struct {
	logger *logrus.Logger
}

// NewLogrusLogger 创建一个新的 LogrusLogger
func NewLogrusLogger(config *Config) Logger {
	logger := logrus.New()

	// 配置日志轮转
	logger.SetOutput(&lumberjack.Logger{
		Filename:   "node.log",
		MaxSize:    config.Log.MaxSize,    // 从 LogConfig 中读取
		MaxBackups: config.Log.MaxBackups, // 从 LogConfig 中读取
		MaxAge:     config.Log.MaxAge,     // 从 LogConfig 中读取
		Compress:   config.Log.Compress,   // 从 LogConfig 中读取
	})

	// 同时输出到控制台
	logger.SetOutput(io.MultiWriter(os.Stdout, logger.Out))

	// 设置日志格式为 JSON
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02 15:04:05",
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "level",
			logrus.FieldKeyMsg:   "message",
		},
	})

	// 设置日志级别
	switch config.Log.Level { // 从 LogConfig 中读取
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

	return &LogrusLogger{logger: logger}
}

func (l *LogrusLogger) Debugf(format string, args ...interface{}) {
	l.logger.Debugf(format, args...)
}

func (l *LogrusLogger) Debug(args ...interface{}) {
	l.logger.Debug(args...)
}

func (l *LogrusLogger) Infof(format string, args ...interface{}) {
	l.logger.Infof(format, args...)
}

func (l *LogrusLogger) Info(args ...interface{}) {
	l.logger.Info(args...)
}

func (l *LogrusLogger) Warnf(format string, args ...interface{}) {
	l.logger.Warnf(format, args...)
}
func (l *LogrusLogger) Warn(args ...interface{}) {
	l.logger.Warn(args...)
}
func (l *LogrusLogger) Errorf(format string, args ...interface{}) {
	l.logger.Errorf(format, args...)
}

func (l *LogrusLogger) Error(args ...interface{}) {
	l.logger.Error(args...)
}

func (l *LogrusLogger) WithFields(fields map[string]interface{}) Logger {
	return &LogrusLogger{logger: l.logger.WithFields(fields).Logger}
}

func (l *LogrusLogger) WithError(err error) Logger {
	return &LogrusLogger{logger: l.logger.WithError(err).Logger}
}

// SetLogLevel dynamically sets the log level.
func SetLogLevel(logger Logger, level string) {
	switch level {
	case "debug":
		logger.(*LogrusLogger).logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.(*LogrusLogger).logger.SetLevel(logrus.InfoLevel)
	case "warn":
		logger.(*LogrusLogger).logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.(*LogrusLogger).logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.(*LogrusLogger).logger.SetLevel(logrus.InfoLevel)
	}
	logger.WithFields(map[string]interface{}{"level": level}).Info("Log level changed")
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

	go http.ListenAndServe(":"+config.Log.APIPort, nil) // 从 LogConfig 中读取
	logger.WithFields(map[string]interface{}{"port": config.Log.APIPort}).Info("Log level API started")
}
