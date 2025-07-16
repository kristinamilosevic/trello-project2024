package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"trello-project/microservices/users-service/logging" // Dodato za pristup custom loggeru
)

type CaptchaResponse struct {
	Success     bool     `json:"success"`
	ChallengeTs string   `json:"challenge_ts"`
	Hostname    string   `json:"hostname"`
	ErrorCodes  []string `json:"error-codes,omitempty"`
}

func VerifyCaptcha(token string) (bool, error) {
	logging.Logger.Debug("Event ID: VERIFY_CAPTCHA_START, Description: Attempting to verify reCAPTCHA token.")

	secretKey := os.Getenv("SECRET_KEY")
	if secretKey == "" {
		logging.Logger.Errorf("Event ID: VERIFY_CAPTCHA_SECRET_MISSING, Description: SECRET_KEY environment variable is not set.")
		return false, fmt.Errorf("SECRET_KEY is not set in environment variables")
	}
	logging.Logger.Debug("Event ID: VERIFY_CAPTCHA_SECRET_FOUND, Description: SECRET_KEY found in environment variables.")

	data := url.Values{}
	data.Set("secret", secretKey)
	data.Set("response", token)
	logging.Logger.Debug("Event ID: VERIFY_CAPTCHA_FORM_DATA_SET, Description: Form data for reCAPTCHA verification prepared.")

	resp, err := http.PostForm("https://www.google.com/recaptcha/api/siteverify", data)
	if err != nil {
		logging.Logger.Errorf("Event ID: VERIFY_CAPTCHA_HTTP_POST_FAILED, Description: Error sending request to Google API: %v", err)
		return false, fmt.Errorf("error sending request to Google API: %v", err)
	}
	defer resp.Body.Close()
	logging.Logger.Debugf("Event ID: VERIFY_CAPTCHA_HTTP_RESPONSE, Description: Received HTTP response from Google API with status: %s", resp.Status)

	var captchaResp CaptchaResponse
	if err := json.NewDecoder(resp.Body).Decode(&captchaResp); err != nil {
		logging.Logger.Errorf("Event ID: VERIFY_CAPTCHA_DECODE_FAILED, Description: Error decoding Google API response: %v", err)
		return false, fmt.Errorf("error decoding Google API response: %v", err)
	}
	logging.Logger.Debugf("Event ID: VERIFY_CAPTCHA_RESPONSE_DECODED, Description: Google API response decoded. Success: %t, ErrorCodes: %v", captchaResp.Success, captchaResp.ErrorCodes)

	if captchaResp.Success {
		logging.Logger.Infof("Event ID: VERIFY_CAPTCHA_SUCCESS, Description: reCAPTCHA verification successful.")
	} else {
		logging.Logger.Warnf("Event ID: VERIFY_CAPTCHA_FAILED, Description: reCAPTCHA verification failed. Error codes: %v", captchaResp.ErrorCodes)
	}

	return captchaResp.Success, nil
}
