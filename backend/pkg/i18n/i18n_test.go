package i18n

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSupported(t *testing.T) {
	if len(Supported()) != 2 {
		t.Error("expected 2 supported langs")
	}
}

func TestParse(t *testing.T) {
	if Parse("", "") != Default {
		t.Error("empty should be default")
	}
	if Parse("en-US,en;q=0.9", "") != EnUS {
		t.Error("accept-language en should resolve EnUS")
	}
	if Parse("zh-CN", "") != ZhCN {
		t.Error("accept-language zh should resolve ZhCN")
	}
	// override header wins
	if Parse("en-US", "zh-CN") != ZhCN {
		t.Error("override should win")
	}
	// override unsupported falls back to accept
	if Parse("en-US", "fr-FR") != EnUS {
		t.Error("unsupported override should fall back to accept")
	}
}

func TestNormalize(t *testing.T) {
	cases := map[string]Lang{
		"zh-CN":  ZhCN,
		"zh":     ZhCN,
		"ZH-cn ": ZhCN,
		"en":     EnUS,
		"en-US":  EnUS,
		"fr":     "",
		"":       "",
	}
	for in, want := range cases {
		if Normalize(in) != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, Normalize(in), want)
		}
	}
}

func TestContext(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	if FromContext(c) != Default {
		t.Error("empty context should be default")
	}
	SetInContext(c, EnUS)
	if FromContext(c) != EnUS {
		t.Error("should read set lang")
	}
}
