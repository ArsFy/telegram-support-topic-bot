package email

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/smtp"
	"topic-bot/config"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
)

type EmailConfig struct {
	IMAPServer string
	SMTPServer string
	IMAPPort   string
	SMTPPort   string
	Username   string
	Password   string
	UseTLS     bool
}

type IMAPClient struct {
	client *client.Client
	config *EmailConfig
}

type SMTPClient struct {
	config *EmailConfig
}

func NewEmailConfig(imapServer, smtpServer, imapPort, smtpPort, username, password string, useTLS bool) *EmailConfig {
	return &EmailConfig{
		IMAPServer: imapServer,
		SMTPServer: smtpServer,
		IMAPPort:   imapPort,
		SMTPPort:   smtpPort,
		Username:   username,
		Password:   password,
		UseTLS:     useTLS,
	}
}

func NewIMAPClient(config *EmailConfig) (*IMAPClient, error) {
	var c *client.Client
	var err error

	addr := fmt.Sprintf("%s:%s", config.IMAPServer, config.IMAPPort)

	if config.UseTLS {
		c, err = client.DialTLS(addr, &tls.Config{ServerName: config.IMAPServer})
	} else {
		c, err = client.Dial(addr)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to IMAP server: %v", err)
	}

	if err := c.Login(config.Username, config.Password); err != nil {
		c.Logout()
		return nil, fmt.Errorf("failed to login to IMAP server: %v", err)
	}

	return &IMAPClient{
		client: c,
		config: config,
	}, nil
}

func NewSMTPClient(config *EmailConfig) *SMTPClient {
	return &SMTPClient{
		config: config,
	}
}

func (ic *IMAPClient) Close() error {
	if ic.client != nil {
		ic.client.Logout()
		return ic.client.Close()
	}
	return nil
}

func (ic *IMAPClient) ListMailboxes() ([]*imap.MailboxInfo, error) {
	mailboxes := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)

	go func() {
		done <- ic.client.List("", "*", mailboxes)
	}()

	var result []*imap.MailboxInfo
	for m := range mailboxes {
		result = append(result, m)
	}

	if err := <-done; err != nil {
		return nil, fmt.Errorf("failed to list mailboxes: %v", err)
	}

	return result, nil
}

func (ic *IMAPClient) SelectMailbox(mailbox string) (*imap.MailboxStatus, error) {
	mbox, err := ic.client.Select(mailbox, false)
	if err != nil {
		return nil, fmt.Errorf("failed to select mailbox %s: %v", mailbox, err)
	}
	return mbox, nil
}

func (ic *IMAPClient) GetMessageBody(msg *imap.Message) (string, error) {
	if msg.Body == nil {
		return "", fmt.Errorf("message body is nil")
	}

	section := &imap.BodySectionName{}
	r := msg.GetBody(section)
	if r == nil {
		return "", fmt.Errorf("message body reader is nil")
	}

	mr, err := mail.CreateReader(r)
	if err != nil {
		return "", err
	}

	var htmlBody string
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			contentType, _, _ := h.ContentType()
			b, err := io.ReadAll(p.Body)
			if err != nil {
				continue
			}

			if contentType == "text/plain" {
				return string(b), nil
			}
			if contentType == "text/html" {
				htmlBody = string(b)
			}
		case *mail.AttachmentHeader:
			// 跳过附件
			continue
		default:
			_ = h
			continue
		}
	}

	if htmlBody != "" {
		return htmlBody, nil
	}

	return "", fmt.Errorf("no readable body found")
}

func (sc *SMTPClient) ReplyEmail(replyTo, subject, messageId string, replyBody string) error {
	replySubject := sc.buildReplySubject(subject)

	return sc.ReplyToEmail(messageId, replyTo, replySubject, replyBody)
}

func (sc *SMTPClient) buildReplySubject(originalSubject string) string {
	if originalSubject == "" {
		return "Re: "
	}

	if len(originalSubject) >= 3 && originalSubject[:3] == "Re:" {
		return originalSubject
	}

	return "Re: " + originalSubject
}

func (sc *SMTPClient) ReplyToEmail(messageId string, replyTo, replySubject, replyBody string) error {
	addr := fmt.Sprintf("%s:%s", sc.config.SMTPServer, sc.config.SMTPPort)
	auth := smtp.PlainAuth("", sc.config.Username, sc.config.Password, sc.config.SMTPServer)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nIn-Reply-To: %s\r\nReferences: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		sc.config.Username, replyTo, replySubject, messageId, messageId, replyBody)

	var err error
	if sc.config.UseTLS {
		err = sc.sendWithTLS(addr, auth, replyTo, msg)
	} else {
		err = smtp.SendMail(addr, auth, sc.config.Username, []string{replyTo}, []byte(msg))
	}

	if err != nil {
		return fmt.Errorf("failed to send reply email: %v", err)
	}

	return nil
}

func (sc *SMTPClient) SendEmail(to, subject, body string) error {
	addr := fmt.Sprintf("%s:%s", sc.config.SMTPServer, sc.config.SMTPPort)

	auth := smtp.PlainAuth("", sc.config.Username, sc.config.Password, sc.config.SMTPServer)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		sc.config.Username, to, subject, body)

	var err error
	if sc.config.UseTLS {
		err = sc.sendWithTLS(addr, auth, to, msg)
	} else {
		err = smtp.SendMail(addr, auth, sc.config.Username, []string{to}, []byte(msg))
	}

	if err != nil {
		return fmt.Errorf("failed to send email: %v", err)
	}

	return nil
}

func (sc *SMTPClient) sendWithTLS(addr string, auth smtp.Auth, to, msg string) error {
	c, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("failed to dial SMTP server: %v", err)
	}
	defer c.Quit()

	if ok, _ := c.Extension("STARTTLS"); ok {
		config := &tls.Config{ServerName: sc.config.SMTPServer}
		if err = c.StartTLS(config); err != nil {
			return fmt.Errorf("failed to start TLS: %v", err)
		}
	}

	if err := c.Auth(auth); err != nil {
		return fmt.Errorf("failed to authenticate: %v", err)
	}

	if err := c.Mail(sc.config.Username); err != nil {
		return fmt.Errorf("failed to set sender: %v", err)
	}

	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("failed to set recipient: %v", err)
	}

	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("failed to get data writer: %v", err)
	}
	defer w.Close()

	_, err = w.Write([]byte(msg))
	if err != nil {
		return fmt.Errorf("failed to write message: %v", err)
	}

	return nil
}

var ImapClient *IMAPClient
var SmtpClient *SMTPClient

func Init() {
	config := NewEmailConfig(
		config.Conf.Email.IMAP,
		config.Conf.Email.SMTP,
		"993",
		"587",
		config.Conf.Email.Username,
		config.Conf.Email.Password,
		true,
	)

	var err error

	ImapClient, err = NewIMAPClient(config)
	if err != nil {
		log.Fatalf("Failed to create IMAP client: %v", err)
	}
	SmtpClient = NewSMTPClient(config)

	// err = smtpClient.SendEmail(
	// 	"coconut@noy.asia",
	// 	"Test Email",
	// 	"This is a test email sent from Go IMAP/SMTP client.",
	// )
	// if err != nil {
	// 	log.Printf("Failed to send email: %v", err)
	// } else {
	// 	fmt.Println("Email sent successfully!")
	// }
}
