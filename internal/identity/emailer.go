package identity

import (
	"fmt"
	"math/rand"
	"net/smtp"
	"time"
)

const (
	SMTPHost     = "smtp.gmail.com"
	SMTPPort     = "587"
	SenderEmail  = "sasbergson@gmail.com"
	SenderAppPwd = "tgnw azxw lfjr jsuz"
)

// GenerateOTP will generate a 6 digits number
func GenerateOTP() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("%06d", r.Intn(1000000))
}

// SendOTPEmail will send the mail in a synchronous way but will be call inside  a goroutine so that execution will be asynchronous
func SendOTPEmail(toEmail, otpCode string) error {
	auth := smtp.PlainAuth("", SenderEmail, SenderAppPwd, SMTPHost)
	subject := "Subject: Your Nexus OTP Code\n"
	mime := "MIME-version: 1.0;\nContent-Type: text/plain; charset=\"UTF-8\";\n\n"
	body := fmt.Sprintf("Hello,\n\nYour OTP code for Nexus is: %s\n\nThis code expires in 5 minutes.", otpCode)

	msg := []byte(subject + mime + body)

	addr := fmt.Sprintf("%s:%s", SMTPHost, SMTPPort)

	fmt.Printf("-> Sending email to %s...\n", toEmail)
	if err := smtp.SendMail(addr, auth, SenderEmail, []string{toEmail}, msg); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}
	fmt.Printf("-> Email sent to %s\n", toEmail)
	return nil
}
