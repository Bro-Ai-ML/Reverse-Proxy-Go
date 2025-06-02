package email

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"github.com/wneessen/go-mail"
)

// EmailService defines the interface for sending emails
type EmailService interface {
	Send(to, subject, templateName string, data interface{}) error
}

// SMTPService implements EmailService using SMTP
type SMTPService struct {
	config    Config
	templates *template.Template
	logger    *zerolog.Logger
}

// Config holds SMTP configuration
type Config struct {
	Host         string
	Port         int
	Username     string
	Password     string
	From         string
	TemplatePath string
}

// NewSMTPService creates a new SMTP email service
func NewSMTPService(cfg Config) *SMTPService {
	service := &SMTPService{
		config: cfg,
	}

	// Load email templates
	service.loadTemplates()

	return service
}

// WithLogger sets the logger for the email service
func (s *SMTPService) WithLogger(logger *zerolog.Logger) *SMTPService {
	s.logger = logger
	return s
}

// Send sends an email using the specified template and data
func (s *SMTPService) Send(to, subject, templateName string, data interface{}) error {
	m := mail.NewMsg()
	m.From(s.config.From)
	m.To(to)
	m.Subject(subject)
	var body bytes.Buffer
	tmpl := s.templates.Lookup(templateName)
	if tmpl == nil {
		return fmt.Errorf("template %s not found", templateName)
	}
	if err := tmpl.Execute(&body, data); err != nil {
		return err
	}
	m.SetBodyString(mail.TypeTextHTML, body.String())
	client, err := mail.NewClient(s.config.Host,
		mail.WithPort(s.config.Port),
		mail.WithSMTPAuth(s.config.Username, s.config.Password),
		mail.WithTLSPolicy(mail.TLSMandatory),
	)
	if err != nil {
		return err
	}
	return client.DialAndSend(m)
}

// loadTemplates loads all email templates from the configured directory
func (s *SMTPService) loadTemplates() {
	tmpl := template.New("")

	// Walk through the template directory and parse all .html files
	filepath.Walk(s.config.TemplatePath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-HTML files
		if info.IsDir() || filepath.Ext(path) != ".html" {
			return nil
		}

		// Read template file
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read template file %s: %w", path, err)
		}

		// Get template name (without extension)
		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

		// Parse template
		_, err = tmpl.New(name).Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse template %s: %w", path, err)
		}

		return nil
	})

	s.templates = tmpl
}

// MockEmailService is a mock implementation of EmailService for testing
type MockEmailService struct {
	SentEmails []SentEmail
}

// SentEmail represents a sent email in the mock service
type SentEmail struct {
	To           string
	Subject      string
	TemplateName string
	Data         interface{}
}

// NewMockEmailService creates a new mock email service
func NewMockEmailService() *MockEmailService {
	return &MockEmailService{
		SentEmails: make([]SentEmail, 0),
	}
}

// Send records the email in the mock service
func (m *MockEmailService) Send(to, subject, templateName string, data interface{}) error {
	m.SentEmails = append(m.SentEmails, SentEmail{
		To:           to,
		Subject:      subject,
		TemplateName: templateName,
		Data:         data,
	})
	return nil
}
