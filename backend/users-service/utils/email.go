package utils

import (
	"fmt"
	"os"

	"gopkg.in/gomail.v2"
)

// SendEmail šalje email na zadatu adresu sa naslovom i sadržajem
func SendEmail(to, subject, body string) error {
	mailer := gomail.NewMessage()
	mailer.SetHeader("From", "trixtix9@gmail.com")
	mailer.SetHeader("To", to)
	mailer.SetHeader("Subject", subject)
	mailer.SetBody("text/html", body)

	emailPassword := os.Getenv("EMAIL_PASSWORD")
	if emailPassword == "" {
		return fmt.Errorf("EMAIL_PASSWORD nije postavljena")
	}

	dialer := gomail.NewDialer("smtp.gmail.com", 587, "trixtix9@gmail.com", emailPassword)
	return dialer.DialAndSend(mailer)
}
