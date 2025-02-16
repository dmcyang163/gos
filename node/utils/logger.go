package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/iancoleman/orderedmap"
	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger 接口
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

// ZapLogger 结构体
type ZapLogger struct {
	logger *zap.Logger
}

// LogConfig 结构体
type LogConfig struct {
	Level      string `json:"level"`
	MaxSize    int    `json:"max_size"`
	MaxBackups int    `json:"max_backups"`
	MaxAge     int    `json:"max_age"`
	Compress   bool   `json:"compress"`
	APIPort    string `json:"api_port"`
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

// NewLogger 创建一个新的 Logger 实例 (使用可变参数)
func NewLogger(filename string, options ...interface{}) Logger {
	config := defaultLogConfig
	consoleOutput := false

	// 解析可变参数
	for _, option := range options {
		switch v := option.(type) {
		case *LogConfig:
			config = *v // 复制 config 的值
		case bool:
			consoleOutput = v
		default:
			fmt.Printf("Invalid option type: %T\n", option)
		}
	}

	logFile := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    config.MaxSize,
		MaxBackups: config.MaxBackups,
		MaxAge:     config.MaxAge,
		Compress:   config.Compress,
	}

	var output io.Writer
	if consoleOutput {
		// 同时输出到控制台和文件
		output = zapcore.NewMultiWriteSyncer(
			zapcore.AddSync(os.Stdout),
			zapcore.AddSync(logFile),
		)
	} else {
		// 只输出到文件
		output = zapcore.NewMultiWriteSyncer(zapcore.AddSync(logFile))
	}

	logger := configureLogger(output, &config)
	return &ZapLogger{logger: logger}
}

// OrderedJSONEncoder 结构体
type OrderedJSONEncoder struct {
	zapcore.Encoder
}

const (
	timeFormat = "2006-01-02 15:04:05" // 定义时间格式常量
)

var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(strings.Builder) // 使用 strings.Builder
	},
}

// EncodeEntry 方法
func (e *OrderedJSONEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	// 创建一个有序的 map
	data := orderedmap.New()

	data.Set("level", entry.Level.String())
	// 使用自定义格式的时间戳
	data.Set("timestamp", entry.Time.Format(timeFormat))
	data.Set("message", entry.Message)

	// 添加 caller 信息
	if entry.Caller.Defined {
		data.Set("caller", entry.Caller.String())
	}

	// 添加其他字段
	for _, field := range fields {
		data.Set(field.Key, field.Interface)
	}

	// 将有序的 map 转换为 JSON
	serialized, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal fields to JSON: %w", err)
	}

	// 从 pool 中获取 buffer
	buf := bufferPool.Get().(*strings.Builder)
	buf.Reset() // 重置 buffer
	defer bufferPool.Put(buf)

	buf.WriteString(string(serialized))
	buf.WriteString("\n")

	b := buffer.NewPool().Get()
	b.AppendString(buf.String())
	return b, nil
}

// CustomTimeEncoder 自定义时间编码器
func CustomTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format(timeFormat))
}

// NewOrderedJSONEncoder creates a new OrderedJSONEncoder.
func NewOrderedJSONEncoder(cfg zapcore.EncoderConfig) zapcore.Encoder {
	return &OrderedJSONEncoder{Encoder: zapcore.NewJSONEncoder(cfg)}
}

// configureLogger 函数
func configureLogger(output io.Writer, config *LogConfig) *zap.Logger {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     CustomTimeEncoder, // 使用自定义时间编码器
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   nil, // 关闭默认的 CallerEncoder
	}

	core := zapcore.NewCore(
		NewOrderedJSONEncoder(encoderConfig),
		zapcore.AddSync(output),
		zapcore.Level(config.getZapLevel()),
	)

	// 使用 zap.New 创建 logger，并添加调用者信息
	logger := zap.New(core, zap.AddCallerSkip(2), zap.AddStacktrace(zapcore.ErrorLevel))
	return logger
}

// Debugf 方法
func (l *ZapLogger) Debugf(format string, args ...interface{}) {
	l.logger.Sugar().Debugf(format, args...)
}

// Debug 方法
func (l *ZapLogger) Debug(args ...interface{}) {
	l.logger.Sugar().Debug(args...)
}

// Infof 方法
func (l *ZapLogger) Infof(format string, args ...interface{}) {
	l.logger.Sugar().Infof(format, args...)
}

// Info 方法
func (l *ZapLogger) Info(args ...interface{}) {
	l.logger.Sugar().Info(args...)
}

// Warnf 方法
func (l *ZapLogger) Warnf(format string, args ...interface{}) {
	l.logger.Sugar().Warnf(format, args...)
}

// Warn 方法
func (l *ZapLogger) Warn(args ...interface{}) {
	l.logger.Sugar().Warn(args...)
}

// Errorf 方法
func (l *ZapLogger) Errorf(format string, args ...interface{}) {
	l.logger.Sugar().Errorf(format, args...)
}

// Error 方法
func (l *ZapLogger) Error(args ...interface{}) {
	l.logger.Sugar().Error(args...)
}

// WithFields 方法
func (l *ZapLogger) WithFields(fields map[string]interface{}) Logger {
	zapFields := make([]zap.Field, 0, len(fields))
	for key, value := range fields {
		zapFields = append(zapFields, zap.Any(key, value))
	}
	return &ZapLogger{logger: l.logger.With(zapFields...)}
}

// WithError 方法
func (l *ZapLogger) WithError(err error) Logger {
	return &ZapLogger{logger: l.logger.With(zap.Error(err))}
}

// getZapLevel 方法
func (c *LogConfig) getZapLevel() zapcore.Level {
	switch c.Level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// SetLogLevel dynamically sets the log level.
func SetLogLevel(logger Logger, level string) {
	zapLogger, ok := logger.(*ZapLogger)
	if !ok {
		return
	}

	var logLevel zapcore.Level
	switch level {
	case "debug":
		logLevel = zapcore.DebugLevel
	case "info":
		logLevel = zapcore.InfoLevel
	case "warn":
		logLevel = zapcore.WarnLevel
	case "error":
		logLevel = zapcore.ErrorLevel
	default:
		zapLogger.logger.Sugar().Warnw("Invalid log level", "level", level)
		return
	}

	// 动态调整日志级别
	zapLogger.logger.Core().Enabled(logLevel)
	zapLogger.logger.Sugar().Infow("Log level changed", "level", level)
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
