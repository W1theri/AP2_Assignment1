package provider

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// SimulatedEmailProvider simulates sending emails with latency and occasional failures
type SimulatedEmailProvider struct {
	config *ProviderConfig
	random *rand.Rand
}

// NewSimulatedEmailProvider creates a simulated email provider
func NewSimulatedEmailProvider(config *ProviderConfig) *SimulatedEmailProvider {
	if config.SimulatedLatency == 0 {
		config.SimulatedLatency = 500 // 500ms default
	}

	return &SimulatedEmailProvider{
		config: config,
		random: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Send sends an email with simulated latency and occasional failures
func (sep *SimulatedEmailProvider) Send(ctx context.Context, to string, subject string, body string) error {
	// Simulate network latency
	latency := sep.simulateLatency()
	select {
	case <-time.After(latency):
	case <-ctx.Done():
		return ctx.Err()
	}

	// Simulate occasional failures
	if sep.shouldFail() {
		return fmt.Errorf("simulated email provider error: temporary failure")
	}

	// Log the "sent" email
	fmt.Printf("[SIMULATED EMAIL] To: %s, Subject: %s, Body: %s (latency: %v)\n", to, subject, body, latency)
	return nil
}

// Close closes the provider
func (sep *SimulatedEmailProvider) Close() error {
	return nil
}

// simulateLatency simulates network latency with some variance
func (sep *SimulatedEmailProvider) simulateLatency() time.Duration {
	// Add ±30% variance
	variance := float64(sep.config.SimulatedLatency) * 0.3
	randomVariance := (sep.random.Float64() * 2 * variance) - variance
	latency := float64(sep.config.SimulatedLatency) + randomVariance

	return time.Duration(math.Max(latency, 100)) * time.Millisecond
}

// shouldFail determines if this request should fail
func (sep *SimulatedEmailProvider) shouldFail() bool {
	failRate := sep.config.SimulatedFailRate
	if failRate == 0 {
		failRate = 10 // 10% failure rate by default
	}

	return sep.random.Intn(100) < failRate
}
