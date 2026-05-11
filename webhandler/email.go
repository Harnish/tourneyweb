package webhandler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/smtp"
	"strings"
	"time"
)

type EmailService struct {
	smtpHost     string
	smtpPort     string
	smtpUsername string
	smtpPassword string
	fromEmail    string
	baseURL      string
}

func NewEmailService(host, port, username, password, fromEmail, baseURL string) *EmailService {
	return &EmailService{
		smtpHost:     host,
		smtpPort:     port,
		smtpUsername: username,
		smtpPassword: password,
		fromEmail:    fromEmail,
		baseURL:      baseURL,
	}
}

func (s *EmailService) sendEmail(toEmail, subject, htmlBody string) error {
	addr := fmt.Sprintf("%s:%s", s.smtpHost, s.smtpPort)
	auth := smtp.PlainAuth("", s.smtpUsername, s.smtpPassword, s.smtpHost)

	msgIDBytes := make([]byte, 16)
	rand.Read(msgIDBytes)
	senderDomain := "mail"
	if idx := strings.LastIndex(s.fromEmail, "@"); idx >= 0 {
		senderDomain = s.fromEmail[idx+1:]
	}
	msgID := fmt.Sprintf("<%s@%s>", hex.EncodeToString(msgIDBytes), senderDomain)
	safeSubject := strings.ReplaceAll(strings.ReplaceAll(subject, "\r", ""), "\n", "")

	msg := fmt.Sprintf(
		"From: TourneyWeb <%s>\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMessage-ID: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=\"UTF-8\"\r\n\r\n%s",
		s.fromEmail, toEmail, safeSubject,
		time.Now().Format(time.RFC1123Z),
		msgID, htmlBody,
	)

	if err := smtp.SendMail(addr, auth, s.fromEmail, []string{toEmail}, []byte(msg)); err != nil {
		log.Printf("email: FAILED to=%s subject=%q err=%v", toEmail, safeSubject, err)
		return err
	}
	log.Printf("email: OK to=%s subject=%q", toEmail, safeSubject)
	return nil
}

func (s *EmailService) SendVerificationEmail(toEmail, name, token string) error {
	url := fmt.Sprintf("%s/verify?token=%s", s.baseURL, token)
	body := fmt.Sprintf(`<html><body style="font-family:Arial,sans-serif;max-width:600px;margin:0 auto;padding:20px;">
<h2>Verify your TourneyWeb email</h2>
<p>Hi %s,</p>
<p>Click the button below to verify your email address.</p>
<p style="text-align:center;margin:30px 0;">
  <a href="%s" style="background-color:#2a6099;color:white;padding:12px 24px;text-decoration:none;border-radius:6px;display:inline-block;">Verify Email</a>
</p>
<p style="color:#6b7280;font-size:.9em;">If you didn't create an account you can ignore this email.</p>
<p style="color:#6b7280;font-size:.85em;">Or copy this link: %s</p>
</body></html>`, name, url, url)
	return s.sendEmail(toEmail, "Verify your TourneyWeb email", body)
}

func (s *EmailService) SendPasswordResetEmail(toEmail, name, token string) error {
	url := fmt.Sprintf("%s/password-reset/confirm?token=%s", s.baseURL, token)
	body := fmt.Sprintf(`<html><body style="font-family:Arial,sans-serif;max-width:600px;margin:0 auto;padding:20px;">
<h2>Reset your TourneyWeb password</h2>
<p>Hi %s,</p>
<p>Click the button below to choose a new password. This link expires in <strong>1 hour</strong>.</p>
<p style="text-align:center;margin:30px 0;">
  <a href="%s" style="background-color:#2a6099;color:white;padding:12px 24px;text-decoration:none;border-radius:6px;display:inline-block;">Reset Password</a>
</p>
<p style="color:#6b7280;font-size:.9em;">If you didn't request a reset, ignore this email.</p>
<p style="color:#6b7280;font-size:.85em;">Or copy this link: %s</p>
</body></html>`, name, url, url)
	return s.sendEmail(toEmail, "Reset your TourneyWeb password", body)
}

func (s *EmailService) SendInvitationEmail(toEmail, tournamentName, role string) error {
	url := fmt.Sprintf("%s/register", s.baseURL)
	body := fmt.Sprintf(`<html><body style="font-family:Arial,sans-serif;max-width:600px;margin:0 auto;padding:20px;">
<h2>You've been invited to manage %s</h2>
<p>You have been invited as <strong>%s</strong> for the tournament <strong>%s</strong>.</p>
<p>Create your TourneyWeb account and your role will be assigned automatically.</p>
<p style="text-align:center;margin:30px 0;">
  <a href="%s" style="background-color:#2a6099;color:white;padding:12px 24px;text-decoration:none;border-radius:6px;display:inline-block;">Create Account</a>
</p>
<p style="color:#6b7280;font-size:.85em;">Or copy this link: %s</p>
</body></html>`, tournamentName, role, tournamentName, url, url)
	return s.sendEmail(toEmail, fmt.Sprintf("You've been invited to manage %s", tournamentName), body)
}

// GenerateToken returns a cryptographically random 32-byte hex string.
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
