package sms

import (
	"context"
	"testing"
)

func TestDevSenderSend(t *testing.T) {
	if err := NewDevSender().Send(context.Background(), "13800000000", "123456"); err != nil {
		t.Fatalf("DevSender.Send error: %v", err)
	}
}
