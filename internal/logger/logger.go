package logger

import (
	"time"

	"github.com/sirupsen/logrus"
)

// NewLogger returns a custom JSON logger
func NewLogger() logrus.FieldLogger {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	jsonFormatter := logrus.JSONFormatter{
		TimestampFormat: time.RFC3339Nano,
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyMsg:   "message",
			logrus.FieldKeyLevel: "level",
		},
	}
	logger.SetFormatter(&jsonFormatter)

	return logger
}
