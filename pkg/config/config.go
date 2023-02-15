package config

import (
	"flag"
	"fmt"
	"log"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// multiFlag satisfies the Value interface in order to parse multiple command line arguments of the same name into a
// slice.
type multiFlag []string

func (m *multiFlag) String() string {
	return fmt.Sprintf("%+v", *m)
}

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

type Config struct {
	HTTPPort      int
	NodeAddresses []string
	Logger
}

func New() Config {
	port := flag.Int("port", 6000, "the port this server should serve from")
	debug := flag.Bool("debug", false, "enable debug logs")
	var nodesAddresses multiFlag
	flag.Var(&nodesAddresses, "node_address", "list of node addresses that this node should attempt to synchronise with")
	flag.Parse()

	logLevel := zapcore.InfoLevel
	if *debug == true {
		logLevel = zapcore.DebugLevel
	}

	return Config{
		HTTPPort:      *port,
		NodeAddresses: nodesAddresses,
		Logger:        newLogger(logLevel),
	}
}

// Logger defines the required logger methods.
type Logger interface {
	Debug(msg string, fields ...zapcore.Field)
	Info(msg string, fields ...zapcore.Field)
	Warn(msg string, fields ...zapcore.Field)
	Error(msg string, fields ...zapcore.Field)
	Fatal(msg string, fields ...zapcore.Field)
	With(fields ...zap.Field) *zap.Logger
}

func newLogger(level zapcore.Level) Logger {
	loggerConfig := zap.NewProductionConfig()
	loggerConfig.EncoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
	loggerConfig.EncoderConfig.TimeKey = "ts"
	loggerConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	loggerConfig.Level.SetLevel(level)

	logger, err := loggerConfig.Build()
	if err != nil {
		log.Printf("failed to initialise logger: %s", err)
		os.Exit(1)
	}

	return logger
}