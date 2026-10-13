package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestNewZapLogger(t *testing.T) {
	z := NewZapLogger(200 * time.Millisecond)
	if z == nil || z.SlowThreshold != 200*time.Millisecond {
		t.Fatal("invalid logger")
	}
	// LogMode returns self
	if z.LogMode(gormlogger.Info) != z {
		t.Error("LogMode should return self")
	}
	// no-op sinks
	z.Info(context.Background(), "info")
	z.Warn(context.Background(), "warn")
	z.Error(context.Background(), "error")

	// normal trace (also exercises slow-query branch)
	z.Trace(context.Background(), time.Now().Add(-300*time.Millisecond), func() (string, int64) {
		return "SELECT 1", 1
	}, nil)

	// trace with ErrRecordNotFound is swallowed (no error log)
	z.Trace(context.Background(), time.Now(), func() (string, int64) {
		return "SELECT 1", 0
	}, gorm.ErrRecordNotFound)

	// trace with real error
	z.Trace(context.Background(), time.Now(), func() (string, int64) {
		return "SELECT 1", 0
	}, errors.New("boom"))

	// trace with trace_id carried from context
	ctx := context.WithValue(context.Background(), TraceIDKey, "trace-xyz")
	z.Trace(ctx, time.Now(), func() (string, int64) {
		return "SELECT 1", 0
	}, nil)
}

func TestRegisterTracePluginAndInject(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	RegisterTracePlugin(db)

	// injectTraceID bridges the generic "trace_id" key into TraceIDKey
	inner := context.WithValue(context.Background(), "trace_id", "abc")
	db.Statement = &gorm.Statement{Context: inner}
	injectTraceID(db)
	if v, ok := db.Statement.Context.Value(TraceIDKey).(string); !ok || v != "abc" {
		t.Errorf("trace id not injected: %v", v)
	}

	// already has TraceIDKey -> idempotent
	db.Statement = &gorm.Statement{Context: context.WithValue(context.Background(), TraceIDKey, "existing")}
	injectTraceID(db)
	if v, _ := db.Statement.Context.Value(TraceIDKey).(string); v != "existing" {
		t.Errorf("should keep existing trace id, got %v", v)
	}
}

func TestGetDB(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	got := GetDB(context.Background(), db)
	if got == nil {
		t.Error("GetDB returned nil")
	}
}
