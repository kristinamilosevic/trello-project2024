package utils

import (
	"fmt"
	"net/smtp"
	"os"
	"trello-project/microservices/users-service/logging" // Dodato za pristup custom loggeru
)

// SendEmail šalje email na zadatu adresu sa naslovom i sadržajem koristeći net/smtp biblioteku
func SendEmail(to, subject, body string) error {
	logging.Logger.Debugf("Event ID: SEND_EMAIL_START, Description: Attempting to send email to '%s' with subject: '%s'", to, subject)

	// Email podaci
	from := "trixtix9@gmail.com" // Ovo bi idealno trebalo da bude iz konfiguracije/env varijable
	password := os.Getenv("EMAIL_PASSWORD")

	// SMTP server konfiguracija
	smtpHost := "smtp.gmail.com"
	smtpPort := "587"

	// Provera da li je postavljena lozinka
	if password == "" {
		logging.Logger.Errorf("Event ID: SEND_EMAIL_MISSING_ENV, Description: EMAIL_PASSWORD environment variable is not set.")
		return fmt.Errorf("EMAIL_PASSWORD nije postavljena")
	}
	logging.Logger.Debug("Event ID: SEND_EMAIL_ENV_CHECK_SUCCESS, Description: EMAIL_PASSWORD environment variable found.")

	// Priprema sadržaja poruke
	message := []byte("Subject: " + subject + "\r\n" +
		"From: " + from + "\r\n" +
		"To: " + to + "\r\n" +
		"Content-Type: text/html; charset=\"UTF-8\"\r\n\r\n" +
		body + "\r\n")
	logging.Logger.Debug("Event ID: SEND_EMAIL_MESSAGE_COMPOSED, Description: Email message composed.")

	// Autentifikacija sa SMTP serverom
	auth := smtp.PlainAuth("", from, password, smtpHost)

	// Slanje emaila
	err := smtp.SendMail(smtpHost+":"+smtpPort, auth, from, []string{to}, message)
	if err != nil {
		logging.Logger.Errorf("Event ID: SEND_EMAIL_FAILED, Description: Failed to send email to '%s' with subject '%s': %v", to, subject, err)
		return fmt.Errorf("failed to send email: %v", err)
	}

	logging.Logger.Infof("Event ID: SEND_EMAIL_SUCCESS, Description: Email successfully sent to '%s' with subject: '%s'", to, subject)
	return nil
}

// SendRegistrationEmail šalje email za potvrdu registracije
func SendRegistrationEmail(to, token string) error {
	subject := "Confirm your registration"
	confirmationLink := fmt.Sprintf("https://localhost:4200/confirm-email?token=%s", token)
	body := fmt.Sprintf(`
		<h3>Welcome!</h3>
		<p>Please confirm your registration by clicking the link below:</p>
		<a href="%s">Confirm Email</a>
	`, confirmationLink)

	logging.Logger.Debugf("Event ID: SEND_REGISTRATION_EMAIL_PREPARING, Description: Preparing registration email for '%s'.", to)
	err := SendEmail(to, subject, body)
	if err != nil {
		logging.Logger.Errorf("Event ID: SEND_REGISTRATION_EMAIL_FAILED, Description: Failed to send registration email to '%s': %v", to, err)
		return err
	}
	logging.Logger.Infof("Event ID: SEND_REGISTRATION_EMAIL_SUCCESS, Description: Registration email sent to '%s'.", to)
	return nil
}

// SendPasswordResetEmail šalje email sa linkom za resetovanje lozinke
func SendPasswordResetEmail(to, token string) error {
	subject := "Reset your password"
	resetLink := fmt.Sprintf("https://localhost:4200/reset-password?token=%s", token)
	body := fmt.Sprintf(`
		<h3>Password Reset Request</h3>
		<p>Click the link below to reset your password:</p>
		<a href="%s">Reset Password</a>
	`, resetLink)

	logging.Logger.Debugf("Event ID: SEND_PASSWORD_RESET_EMAIL_PREPARING, Description: Preparing password reset email for '%s'.", to)
	err := SendEmail(to, subject, body)
	if err != nil {
		logging.Logger.Errorf("Event ID: SEND_PASSWORD_RESET_EMAIL_FAILED, Description: Failed to send password reset email to '%s': %v", to, err)
		return err
	}
	logging.Logger.Infof("Event ID: SEND_PASSWORD_RESET_EMAIL_SUCCESS, Description: Password reset email sent to '%s'.", to)
	return nil
}

// SendLoginNotification šalje email prilikom prijave (opciono)
func SendLoginNotification(to string) error {
	subject := "Login Notification"
	body := `
		<h3>Login Alert</h3>
		<p>Your account was just accessed. If this wasn't you, please secure your account.</p>
	`

	logging.Logger.Debugf("Event ID: SEND_LOGIN_NOTIFICATION_PREPARING, Description: Preparing login notification email for '%s'.", to)
	err := SendEmail(to, subject, body)
	if err != nil {
		logging.Logger.Errorf("Event ID: SEND_LOGIN_NOTIFICATION_FAILED, Description: Failed to send login notification email to '%s': %v", to, err)
		return err
	}
	logging.Logger.Infof("Event ID: SEND_LOGIN_NOTIFICATION_SUCCESS, Description: Login notification email sent to '%s'.", to)
	return nil
}
