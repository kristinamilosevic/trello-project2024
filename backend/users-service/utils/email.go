package utils

import (
	"fmt"
	"net/smtp"
	"os"
)

// SendEmail šalje email na zadatu adresu sa naslovom i sadržajem koristeći net/smtp biblioteku
func SendEmail(to, subject, body string) error {
	// Email podaci
	from := "trixtix9@gmail.com"
	password := os.Getenv("EMAIL_PASSWORD")

	// SMTP server konfiguracija
	smtpHost := "smtp.gmail.com"
	smtpPort := "587"

	// Provera da li je postavljena lozinka
	if password == "" {
		return fmt.Errorf("EMAIL_PASSWORD nije postavljena")
	}

	// Priprema sadržaja poruke
	message := []byte("Subject: " + subject + "\r\n" +
		"From: " + from + "\r\n" +
		"To: " + to + "\r\n" +
		"Content-Type: text/html; charset=\"UTF-8\"\r\n\r\n" +
		body + "\r\n")

	// Autentifikacija sa SMTP serverom
	auth := smtp.PlainAuth("", from, password, smtpHost)

	// Slanje emaila
	err := smtp.SendMail(smtpHost+":"+smtpPort, auth, from, []string{to}, message)
	if err != nil {
		return fmt.Errorf("failed to send email: %v", err)
	}

	return nil
}
