package generator

import (
	"strings"
	"unicode"
)

// ToCamelCase 将 snake_case 转为 CamelCase（大驼峰）。
//
//	order_items → OrderItem
//	user       → User
func ToCamelCase(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if len(p) > 0 {
			runes := []rune(p)
			runes[0] = unicode.ToUpper(runes[0])
			parts[i] = string(runes)
		}
	}
	return strings.Join(parts, "")
}

// ToLowerCamelCase 将 snake_case 转为 lowerCamelCase（小驼峰）。
//
//	order_items → orderItem
//	user       → user
func ToLowerCamelCase(s string) string {
	camel := ToCamelCase(s)
	if len(camel) == 0 {
		return camel
	}
	runes := []rune(camel)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

// ToSnakeCase 将 CamelCase 转为 snake_case。
//
//	OrderItem → order_item
//	User      → user
func ToSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		// 遇到大写字母（非首字符）时插入下划线
		if unicode.IsUpper(r) && i > 0 {
			result.WriteRune('_')
		}
		result.WriteRune(unicode.ToLower(r))
	}
	return result.String()
}

// ToKebabCase 将 CamelCase 转为 kebab-case。
//
//	OrderItem → order-item
func ToKebabCase(s string) string {
	return strings.ReplaceAll(ToSnakeCase(s), "_", "-")
}
