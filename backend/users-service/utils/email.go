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

// SendRegistrationEmail šalje email za potvrdu registracije
func SendRegistrationEmail(to, token string) error {
	subject := "Confirm your registration"
	confirmationLink := fmt.Sprintf("http://localhost:4200/confirm-email?token=%s", token)
	body := fmt.Sprintf(`
		<h3>Welcome!</h3>
		<p>Please confirm your registration by clicking the link below:</p>
		<a href="%s">Confirm Email</a>
	`, confirmationLink)

	return SendEmail(to, subject, body)
}

// SendPasswordResetEmail šalje email sa linkom za resetovanje lozinke
func SendPasswordResetEmail(to, token string) error {
	subject := "Reset your password"
	resetLink := fmt.Sprintf("http://localhost:4200/reset-password?token=%s", token)
	body := fmt.Sprintf(`
		<h3>Password Reset Request</h3>
		<p>Click the link below to reset your password:</p>
		<a href="%s">Reset Password</a>
	`, resetLink)

	return SendEmail(to, subject, body)
}

// SendLoginNotification šalje email prilikom prijave (opciono)
func SendLoginNotification(to string) error {
	subject := "Login Notification"
	body := `
		<h3>Login Alert</h3>
		<p>Your account was just accessed. If this wasn't you, please secure your account.</p>
	`

	return SendEmail(to, subject, body)
}
