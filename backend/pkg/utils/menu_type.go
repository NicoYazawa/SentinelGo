package utils

import "strconv"

// MenuTypeString 将菜单类型的数值编码转为字母编码。
//
// 映射规则：
//
//	1 → "M" (Menu/目录)
//	2 → "C" (Component/菜单)
//	3 → "F" (Function/按钮)
//
// 其他值直接转为字符串。
func MenuTypeString(t int) string {
	switch t {
	case 1:
		return "M" // 目录
	case 2:
		return "C" // 菜单
	case 3:
		return "F" // 按钮
	default:
		return strconv.Itoa(t)
	}
}
