package utils

import (
	"fmt"
	"net/smtp"
	"os"
	"trello-project/microservices/users-service/logging" // Dodato za pristup custom loggeru
)

// SendEmailPassword šalje email sa zadatim sadržajem
func SendEmailPassword(to, subject, body string) error {
	logging.Logger.Debugf("Event ID: SEND_EMAIL_PASSWORD_START, Description: Attempting to send email to '%s' with subject: '%s'", to, subject)

	from := "trixtix9@gmail.com" // Ovo bi idealno trebalo da bude iz konfiguracije/env varijable
	password := os.Getenv("EMAIL_PASSWORD")

	// SMTP server konfiguracija
	smtpHost := "smtp.gmail.com"
	smtpPort := "587"

	if password == "" {
		logging.Logger.Errorf("Event ID: SEND_EMAIL_PASSWORD_MISSING_ENV, Description: EMAIL_PASSWORD environment variable is not set.")
		return fmt.Errorf("EMAIL_PASSWORD nije postavljena")
	}
	logging.Logger.Debug("Event ID: SEND_EMAIL_PASSWORD_ENV_CHECK_SUCCESS, Description: EMAIL_PASSWORD environment variable found.")

	// Priprema sadržaja poruke
	message := []byte("Subject: " + subject + "\r\n" +
		"From: " + from + "\r\n" +
		"To: " + to + "\r\n" +
		"Content-Type: text/html; charset=\"UTF-8\"\r\n\r\n" +
		body + "\r\n")
	logging.Logger.Debug("Event ID: SEND_EMAIL_PASSWORD_MESSAGE_COMPOSED, Description: Email message composed.")

	auth := smtp.PlainAuth("", from, password, smtpHost)

	// Slanje emaila
	err := smtp.SendMail(smtpHost+":"+smtpPort, auth, from, []string{to}, message)
	if err != nil {
		logging.Logger.Errorf("Event ID: SEND_EMAIL_PASSWORD_FAILED, Description: Failed to send email to '%s' with subject '%s': %v", to, subject, err)
		return fmt.Errorf("failed to send email: %v", err)
	}

	logging.Logger.Infof("Event ID: SEND_EMAIL_PASSWORD_SUCCESS, Description: Email successfully sent to '%s' with subject: '%s'", to, subject)
	return nil
}
