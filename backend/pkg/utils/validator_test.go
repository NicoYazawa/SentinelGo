package utils

import (
	"strings"
	"testing"

	"sentinelgo/pkg/i18n"

	"github.com/go-playground/validator/v10"
)

type sample struct {
	Name  string `validate:"required"`
	Age   int    `validate:"gte=18"`
	Email string `validate:"email"`
}

func runValidation() error {
	v := validator.New()
	return v.Struct(&sample{Name: "", Age: 10, Email: "bad"})
}

func TestTranslateValidationError(t *testing.T) {
	if err := runValidation(); err == nil {
		t.Fatal("expected validation error")
	} else {
		zh := TranslateValidationError(err, i18n.ZhCN)
		if !strings.Contains(zh, "不能为空") {
			t.Errorf("zh translation missing required msg: %q", zh)
		}
		en := TranslateValidationError(err, i18n.EnUS)
		if !strings.Contains(en, "is required") {
			t.Errorf("en translation missing required msg: %q", en)
		}
	}

	// unknown language falls back to default (zh)
	fallback := TranslateValidationError(runValidation(), i18n.Lang("fr"))
	if !strings.Contains(fallback, "不能为空") {
		t.Errorf("fallback translation wrong: %q", fallback)
	}

	// non-validation error returns fallback msg
	nonVal := TranslateValidationError(errFake{}, i18n.EnUS)
	if nonVal != "Invalid request parameters" {
		t.Errorf("non-validation error should return fallback, got %q", nonVal)
	}
}

type errFake struct{}

func (errFake) Error() string { return "boom" }
