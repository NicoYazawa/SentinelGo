package config

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
	"os"
	"reflect"
	"regexp"

	"github.com/go-viper/mapstructure/v2"
)

// encPattern 匹配 ENC(base64-ciphertext) 格式的加密配置值。
// 密钥从环境变量 APP_SECRET_KEY 中读取。
var encPattern = regexp.MustCompile(`^ENC\((.+)\)$`)

// DecryptHook 返回 mapstructure 的 DecodeHook 函数，在配置反序列化时自动解密 ENC(...) 格式的值。
//
// 使用方式（koanf）：
//
//	koanf.UnmarshalConf{ DecodeHook: config.DecryptHook() }
//
// 工作原理：
//   - 检测字符串值是否匹配 ENC(...) 格式
//   - 提取 Base64 编码的密文
//   - 使用 AES-256-CTR 解密
//   - 返回明文填充到配置结构体
func DecryptHook() mapstructure.DecodeHookFunc {
	return func(f reflect.Type, t reflect.Type, data any) (any, error) {
		// 仅处理 string → string 的映射
		if f.Kind() != reflect.String || t.Kind() != reflect.String {
			return data, nil
		}
		s, ok := data.(string)
		if !ok {
			return data, nil
		}
		if !encPattern.MatchString(s) {
			return data, nil
		}
		return decryptString(s), nil
	}
}

// decryptString 解密单个 ENC(base64) 格式的字符串。
// 解密失败时返回原始密文的 Base64 部分，不阻断启动流程。
func decryptString(encValue string) string {
	matches := encPattern.FindStringSubmatch(encValue)
	if len(matches) != 2 {
		return encValue // 格式不匹配，原样返回
	}

	// 解码 Base64 密文
	cipherText, err := base64.StdEncoding.DecodeString(matches[1])
	if err != nil {
		return encValue
	}

	// 从环境变量读取 AES 密钥（未设置则原样返回 Base64 明文）
	key := []byte(os.Getenv("APP_SECRET_KEY"))
	if len(key) == 0 {
		return matches[1]
	}

	// AES-256-CTR 解密
	plainText, err := aesDecrypt(cipherText, key)
	if err != nil {
		return matches[1]
	}
	return string(plainText)
}

// aesDecrypt 使用 AES-256-CTR 模式解密数据。
// 密文格式：IV(16 bytes) + 加密数据
func aesDecrypt(cipherText, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(padKey(key))
	if err != nil {
		return nil, fmt.Errorf("创建 AES cipher 失败: %w", err)
	}

	if len(cipherText) < aes.BlockSize {
		return nil, fmt.Errorf("密文长度不足")
	}

	// 前 BlockSize 字节是 IV（初始化向量）
	iv := cipherText[:aes.BlockSize]
	cipherText = cipherText[aes.BlockSize:]

	// CTR 模式是流密码，XORKeyStream 原地解密
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(cipherText, cipherText)

	return cipherText, nil
}

// padKey 将密钥填充或截断为 32 字节（AES-256 要求）。
// 不足 32 字节时右侧补 0，超出时截取前 32 字节。
func padKey(key []byte) []byte {
	const keyLen = 32
	if len(key) >= keyLen {
		return key[:keyLen]
	}
	padded := make([]byte, keyLen)
	copy(padded, key)
	return padded
}
