package logging

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"sync" // Dodato za once.Do
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger je globalna instanca Logrusa.
// U ovom slučaju, inicijalizovan je direktno kao u projects-service primeru.
var Logger = logrus.New()
var once sync.Once // once je zadržan zbog InitLogger implementacije

// CustomFormatter implementira logrus.Formatter interfejs za prilagođeni format logova.
// Sada ima SystemName polje kao u projects-service.
type CustomFormatter struct {
	SystemName string
}

// Format generiše izlazni bajt niz za log zapis.
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

	// Koristi SystemName iz CustomFormatter instance, kao u projects-service
	b.WriteString(fmt.Sprintf("Event Source: %s, ", f.SystemName))

	b.WriteString(fmt.Sprintf("Event Type: %s, ", strings.ToUpper(entry.Level.String())))

	// Generisanje novog UUID-a za Event ID, kao u projects-service
	eventID := uuid.New().String()
	b.WriteString(fmt.Sprintf("Event ID: %s, ", eventID))

	// Poruka loga, kao u projects-service
	b.WriteString(fmt.Sprintf("Message: %s, ", entry.Message))

	// Informacije o pozivaocu (fajl, linija, funkcija), kao u projects-service
	if entry.HasCaller() {
		// Uklanjamo filepath.Base() i skraćivanje funkcije jer to nije bilo u projects-service primeru
		b.WriteString(fmt.Sprintf(" Location: %s:%d in %s", entry.Caller.File, entry.Caller.Line, entry.Caller.Function))
	}

	b.WriteByte('\n') // Novi red na kraju

	return b.Bytes(), nil
}

func timezoneCEST() *time.Location {
	return time.FixedZone("CEST", 2*60*60) // +2 sata u sekundama
}

// InitLogger inicijalizuje globalni logger.
func InitLogger() {
	once.Do(func() { // once.Do je i dalje tu radi sigurne inicijalizacije

		// Kreiranje 'logs' direktorijuma ako ne postoji
		if _, err := os.Stat("logs"); os.IsNotExist(err) {
			err := os.Mkdir("logs", 0700) // Owner has read/write/execute permissions
			if err != nil {
				// Fatal error ako se direktorijum ne može kreirati, koristeći logrus.Fatalf
				logrus.Fatalf("Event ID: LOG_DIR_CREATE_FAILED, Description: Failed to create log directory: %v", err)
			}
		}

		logFile := &lumberjack.Logger{
			Filename:   "/app/logs/tasks.log", // Promenjena putanja loga za tasks-service
			MaxSize:    10,                    // megabytes
			MaxBackups: 3,                     // number of old log files to retain
			MaxAge:     28,                    // days (kao u projects-service primeru)
			Compress:   true,                  // compress rotated files
		}

		Logger.SetOutput(logFile)

		// Postavljanje CustomFormatter-a sa SystemName-om "tasks-service"
		Logger.SetFormatter(&CustomFormatter{SystemName: "tasks-service"})

		// Postavljanje nivoa logovanja i reportovanja pozivaoca, kao u projects-service
		Logger.SetLevel(logrus.InfoLevel)
		Logger.SetReportCaller(true)

		// Dodavanje inicijalizacione poruke
		Logger.Infof("Event ID: LOGGER_INITIALIZED, Description: Logger initialized for tasks-service, output to: %s", logFile.Filename)
	})
}
