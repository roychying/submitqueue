package controller

import (
	"context"
	"testing"
	"time"

	pb "github.com/uber/submitqueue/orchestrator/protopb"
)

func TestNewPingService(t *testing.T) {
	service := NewPingService()
	if service == nil {
		t.Fatal("NewPingService() returned nil")
	}
}

func TestPing_DefaultMessage(t *testing.T) {
	service := NewPingService()
	ctx := context.Background()

	req := &pb.PingRequest{}
	resp, err := service.Ping(ctx, req)

	if err != nil {
		t.Fatalf("Ping() returned error: %v", err)
	}

	if resp.Message != "pong" {
		t.Errorf("Expected message 'pong', got '%s'", resp.Message)
	}
}

func TestPing_CustomMessage(t *testing.T) {
	service := NewPingService()
	ctx := context.Background()

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple message", "hello", "echo: hello"},
		{"message with spaces", "hello world", "echo: hello world"},
		{"special characters", "test!@#", "echo: test!@#"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &pb.PingRequest{Message: tc.input}
			resp, err := service.Ping(ctx, req)

			if err != nil {
				t.Fatalf("Ping() returned error: %v", err)
			}

			if resp.Message != tc.expected {
				t.Errorf("Expected message '%s', got '%s'", tc.expected, resp.Message)
			}
		})
	}
}

func TestPing_ServiceName(t *testing.T) {
	service := NewPingService()
	ctx := context.Background()

	req := &pb.PingRequest{}
	resp, err := service.Ping(ctx, req)

	if err != nil {
		t.Fatalf("Ping() returned error: %v", err)
	}

	if resp.ServiceName != "orchestrator" {
		t.Errorf("Expected service name 'orchestrator', got '%s'", resp.ServiceName)
	}
}

func TestPing_Timestamp(t *testing.T) {
	service := NewPingService()
	ctx := context.Background()

	before := time.Now().Unix()
	req := &pb.PingRequest{}
	resp, err := service.Ping(ctx, req)
	after := time.Now().Unix()

	if err != nil {
		t.Fatalf("Ping() returned error: %v", err)
	}

	if resp.Timestamp < before || resp.Timestamp > after {
		t.Errorf("Timestamp %d is not within expected range [%d, %d]", resp.Timestamp, before, after)
	}
}

func TestPing_Hostname(t *testing.T) {
	service := NewPingService()
	ctx := context.Background()

	req := &pb.PingRequest{}
	resp, err := service.Ping(ctx, req)

	if err != nil {
		t.Fatalf("Ping() returned error: %v", err)
	}

	// Hostname should be set (non-empty string)
	// We don't check the exact value as it depends on the environment
	if resp.Hostname == "" {
		t.Error("Expected hostname to be set, got empty string")
	}
}
