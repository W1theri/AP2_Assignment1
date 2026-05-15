package provider

import (
	"context"
	"fmt"
)

// EmailProvider defines the interface for sending emails
type EmailProvider interface {
	Send(ctx context.Context, to string, subject string, body string) error
	Close() error
}

// SendEmailRequest represents a request to send an email
type SendEmailRequest struct {
	To      string
	Subject string
	Body    string
}

// SendEmailResponse represents the response from sending an email
type SendEmailResponse struct {
	MessageID string
	Status    string
	Error     error
}

// ProviderConfig holds configuration for email providers
type ProviderConfig struct {
	Mode              string // REAL or SIMULATED
	SMTPHost          string
	SMTPPort          int
	SMTPUser          string
	SMTPPassword      string
	SMTPFromEmail     string
	MailjetAPIKey     string
	MailjetAPISecret  string
	MailjetFromEmail  string
	SimulatedLatency  int // milliseconds
	SimulatedFailRate int // percentage (0-100)
}

// NewEmailProvider creates an email provider based on configuration
func NewEmailProvider(config *ProviderConfig) (EmailProvider, error) {
	switch config.Mode {
	case "REAL":
		return NewRealEmailProvider(config)
	case "SIMULATED":
		return NewSimulatedEmailProvider(config), nil
	default:
		return nil, fmt.Errorf("unknown provider mode: %s", config.Mode)
	}
}

// Real provider (placeholder for SMTP or Mailjet)
type RealEmailProvider struct {
	config *ProviderConfig
}

func NewRealEmailProvider(config *ProviderConfig) (*RealEmailProvider, error) {
	// TODO: Initialize SMTP or Mailjet client based on config
	if config.SMTPHost != "" {
		// Use SMTP
	} else if config.MailjetAPIKey != "" {
		// Use Mailjet
	} else {
		return nil, fmt.Errorf("no real provider credentials configured")
	}

	return &RealEmailProvider{config: config}, nil
}

func (rep *RealEmailProvider) Send(ctx context.Context, to string, subject string, body string) error {
	// TODO: Implement real SMTP or Mailjet sending
	fmt.Printf("[REAL EMAIL] To: %s, Subject: %s\n", to, subject)
	return nil
}

func (rep *RealEmailProvider) Close() error {
	return nil
}
