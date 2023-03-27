package config

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config defines the node config.
type Config struct {
	Hostname      string
	Port          int
	NodeAddresses []string
	Logger
}

// New initialises a new Config from flags and env vars.
func New() Config {
	debug := flag.Bool("debug", false, "enable debug logs")
	port := flag.Int("port", 7000, "the port this server should serve from")
	var nodesAddresses multiFlag
	flag.Var(&nodesAddresses, "node_address", "list of cluster node addresses (NODE_ADDRESSES env var takes priority)")
	flag.Parse()

	if addrs := os.Getenv("NODE_ADDRESSES"); addrs != "" {
		nodesAddresses = strings.Split(addrs, ",")
	}

	logLevel := zapcore.InfoLevel
	if *debug == true {
		logLevel = zapcore.DebugLevel
	}

	return Config{
		Port:          *port,
		NodeAddresses: nodesAddresses,
		Logger:        newLogger(logLevel),
	}
}

// Logger defines the required logger functionality.
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

	logger = logger.With(zap.Int("pid", os.Getpid()))
	return logger
}

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
