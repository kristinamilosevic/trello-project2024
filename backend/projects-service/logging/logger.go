package logging

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

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

	location := timezoneCEST()
	localTime := entry.Time.In(location)

	b.WriteString(fmt.Sprintf("Date: %s, Time: %s, ", localTime.Format("2006-01-02"), localTime.Format("15:04:05")))

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

func timezoneCEST() *time.Location {
	return time.FixedZone("CEST", 2*60*60) // +2 sata u sekundama
}

func InitLogger() {
	// Create logs directory if it doesn't exist
	if _, err := os.Stat("logs"); os.IsNotExist(err) {
		err := os.Mkdir("logs", 0700) // Owner has read/write/execute permissions
		if err != nil {

			logrus.Fatalf("Failed to create log directory: %v", err)
		}
	}

	logFile := &lumberjack.Logger{
		Filename:   "/app/logs/projects.log",
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     28,
		Compress:   true,
	}

	Logger.SetOutput(logFile)

	Logger.SetFormatter(&CustomFormatter{SystemName: "projects-service"}) // Pass SystemName here

	Logger.SetLevel(logrus.InfoLevel)
	Logger.SetReportCaller(true)

}
