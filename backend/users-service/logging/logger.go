package logging

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

var Logger = logrus.New()

type CustomFormatter struct {
	SystemName string
}

func (f *CustomFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var b *bytes.Buffer
	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}

	b.WriteString(fmt.Sprintf("Date: %s, Time: %s, ", entry.Time.Format("2006-01-02"), entry.Time.Format("15:04:05")))

	b.WriteString(fmt.Sprintf("Event Source: %s, ", f.SystemName))

	b.WriteString(fmt.Sprintf("Event Type: %s, ", strings.ToUpper(entry.Level.String())))

	eventID := uuid.New().String()
	b.WriteString(fmt.Sprintf("Event ID: %s, ", eventID))

	b.WriteString(fmt.Sprintf("Message: %s, ", entry.Message))

	if entry.HasCaller() {
		b.WriteString(fmt.Sprintf(" Location: %s:%d in %s", entry.Caller.File, entry.Caller.Line, entry.Caller.Function))
	}

	b.WriteByte('\n')

	return b.Bytes(), nil
}

func InitLogger() {
	if _, err := os.Stat("logs"); os.IsNotExist(err) {
		err := os.Mkdir("logs", 0700)
		if err != nil {
			logrus.Fatalf("Failed to create log directory: %v", err)
		}
	}

	logFile := &lumberjack.Logger{
		Filename:   "/app/logs/users.log",
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     28,
		Compress:   true,
	}

	Logger.SetOutput(logFile)

	Logger.SetFormatter(&CustomFormatter{SystemName: "users-service"})

	Logger.SetLevel(logrus.InfoLevel)
	Logger.SetReportCaller(true)
}
