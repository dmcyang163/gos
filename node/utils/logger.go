package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings" // Import strings package
	"sync"
	"time"

	"github.com/iancoleman/orderedmap"
	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
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

type ZapLogger struct {
	logger *zap.Logger
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

type OrderedJSONEncoder struct {
	zapcore.Encoder
}

const (
	timeFormat = "2006-01-02 15:04:05" // 定义时间格式常量
)

var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(strings.Builder) // Use strings.Builder
	},
}

func (e *OrderedJSONEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	// 创建一个有序的 map
	data := orderedmap.New()

	data.Set("level", entry.Level.String())
	// Use custom format for timestamp
	data.Set("timestamp", entry.Time.Format(timeFormat))
	data.Set("message", entry.Message)

	// 添加其他字段
	for _, field := range fields {
		data.Set(field.Key, field.Interface)
	}

	// 将有序的 map 转换为 JSON
	serialized, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal fields to JSON: %w", err)
	}

	// Get buffer from pool
	buf := bufferPool.Get().(*strings.Builder)
	buf.Reset() // Reset buffer before use
	defer bufferPool.Put(buf)

	buf.WriteString(string(serialized))
	buf.WriteString("\n")

	b := buffer.NewPool().Get()
	b.AppendString(buf.String())
	return b, nil
}

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
		EncodeTime:     CustomTimeEncoder, // Use custom time encoder
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	core := zapcore.NewCore(
		&OrderedJSONEncoder{Encoder: zapcore.NewJSONEncoder(encoderConfig)},
		zapcore.AddSync(output),
		zapcore.Level(config.getZapLevel()),
	)

	// 使用 zap.New 创建 logger，并添加调用者信息
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	return logger
}

// CustomTimeEncoder 自定义时间编码器
func CustomTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format(timeFormat))
}

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

// NewZapLogger 创建一个新的 ZapLogger 实例
func NewZapLogger(filename string, config *LogConfig) Logger {
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

	// 同时输出到控制台和文件
	output := zapcore.NewMultiWriteSyncer(
		zapcore.AddSync(os.Stdout),
		zapcore.AddSync(logFile),
	)

	logger := configureLogger(output, config)
	return &ZapLogger{logger: logger}
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

	// 输出到聊天日志文件
	output := zapcore.NewMultiWriteSyncer(zapcore.AddSync(chatLogFile))
	logger := configureLogger(output, config)
	return &ZapLogger{logger: logger}
}

func (l *ZapLogger) Debugf(format string, args ...interface{}) {
	l.logger.Sugar().Debugf(format, args...)
}

func (l *ZapLogger) Debug(args ...interface{}) {
	l.logger.Sugar().Debug(args...)
}

func (l *ZapLogger) Infof(format string, args ...interface{}) {
	l.logger.Sugar().Infof(format, args...)
}

func (l *ZapLogger) Info(args ...interface{}) {
	l.logger.Sugar().Info(args...)
}

func (l *ZapLogger) Warnf(format string, args ...interface{}) {
	l.logger.Sugar().Warnf(format, args...)
}

func (l *ZapLogger) Warn(args ...interface{}) {
	l.logger.Sugar().Warn(args...)
}

func (l *ZapLogger) Errorf(format string, args ...interface{}) {
	l.logger.Sugar().Errorf(format, args...)
}

func (l *ZapLogger) Error(args ...interface{}) {
	l.logger.Sugar().Error(args...)
}

func (l *ZapLogger) WithFields(fields map[string]interface{}) Logger {
	zapFields := make([]zap.Field, 0, len(fields))
	for key, value := range fields {
		zapFields = append(zapFields, zap.Any(key, value))
	}
	return &ZapLogger{logger: l.logger.With(zapFields...)}
}

func (l *ZapLogger) WithError(err error) Logger {
	return &ZapLogger{logger: l.logger.With(zap.Error(err))}
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
