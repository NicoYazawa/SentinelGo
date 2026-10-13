package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"reflect"
	"testing"
	"time"

	"github.com/go-viper/mapstructure/v2"
)

func TestDecryptHook(t *testing.T) {
	hookVal := reflect.ValueOf(DecryptHook())

	call := func(from, to reflect.Type, data any) (any, error) {
		res := hookVal.Call([]reflect.Value{reflect.ValueOf(from), reflect.ValueOf(to), reflect.ValueOf(data)})
		out, _ := res[0].Interface(), res[1].Interface()
		if err, ok := out.(error); ok {
			return nil, err
		}
		return out, nil
	}

	// non-string source -> passthrough
	out, err := call(reflect.TypeOf(1), reflect.TypeOf(1), 5)
	if err != nil || out != 5 {
		t.Errorf("non-string passthrough wrong: %v %v", out, err)
	}

	// non-ENC string -> passthrough
	out, err = call(reflect.TypeOf(""), reflect.TypeOf(""), "plain")
	if err != nil || out != "plain" {
		t.Errorf("plain passthrough wrong: %v %v", out, err)
	}

	// ENC without key -> base64 body returned (decryptString is the real logic)
	t.Setenv("APP_SECRET_KEY", "")
	if got := decryptString("ENC(aGVsbG8=)"); got != "aGVsbG8=" {
		t.Errorf("ENC without key should return base64 body, got %q", got)
	}
}

func TestDecryptStringErrors(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef"
	t.Setenv("APP_SECRET_KEY", key)

	// malformed base64 inside ENC
	if got := decryptString("ENC(@@@notbase64@@@)"); got != "ENC(@@@notbase64@@@)" {
		t.Errorf("bad base64 should return original, got %q", got)
	}
	// ciphertext too short for aesDecrypt (decodes to 3 bytes < 16)
	if got := decryptString("ENC(" + "YWJj" + ")"); got != "YWJj" {
		t.Errorf("short ciphertext should return base64 body, got %q", got)
	}
}

func TestDecryptStringWithKey(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef"
	t.Setenv("APP_SECRET_KEY", key)

	ct := testEncrypt([]byte("hello-world"), []byte(key))
	if got := decryptString("ENC(" + ct + ")"); got != "hello-world" {
		t.Errorf("decrypt with key wrong: %q", got)
	}
}

func TestAesDecrypt(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	if _, err := aesDecrypt([]byte("short"), key); err == nil {
		t.Error("short ciphertext should error")
	}
}

func TestPadKey(t *testing.T) {
	if len(padKey([]byte("abc"))) != 32 {
		t.Error("padKey should produce 32 bytes")
	}
	if len(padKey(make([]byte, 40))) != 32 {
		t.Error("padKey should truncate to 32 bytes")
	}
}

// testEncrypt mirrors the AES-256-CTR encryption used by the app so tests can
// produce valid ENC() ciphertext for round-trip verification.
func testEncrypt(plain, key []byte) string {
	block, _ := aes.NewCipher(padKey(key))
	iv := make([]byte, aes.BlockSize)
	rand.Read(iv)
	stream := cipher.NewCTR(block, iv)
	ct := make([]byte, len(plain))
	stream.XORKeyStream(ct, plain)
	out := append(append([]byte{}, iv...), ct...)
	return base64.StdEncoding.EncodeToString(out)
}

func TestUnmarshalConfHookComposes(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef"
	t.Setenv("APP_SECRET_KEY", key)
	var result struct {
		Password string        `mapstructure:"password"`
		Timeout  time.Duration `mapstructure:"timeout"`
		Hosts    []string      `mapstructure:"hosts"`
	}
	// Use the production hook composition instead of re-creating it in the test.
	cfg := *unmarshalConf.DecoderConfig
	cfg.Result = &result
	decoder, err := mapstructure.NewDecoder(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(map[string]any{"password": "ENC(" + testEncrypt([]byte("database-secret"), []byte(key)) + ")", "timeout": "2s", "hosts": "db1,db2"}); err != nil {
		t.Fatal(err)
	}
	if result.Password != "database-secret" || result.Timeout != 2*time.Second || !reflect.DeepEqual(result.Hosts, []string{"db1", "db2"}) {
		t.Fatalf("生产配置 hook 未正确组合: %+v", result)
	}
}
