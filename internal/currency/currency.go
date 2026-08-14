package currency

import (
	"regexp"
	"strings"
)

// Preset 常用法定货币预设,管理员可在「定价设置」中直接选用。
type Preset struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Symbol string `json:"symbol"`
}

// Presets 预设的常用法定货币。
var Presets = []Preset{
	{Code: "USD", Name: "美元", Symbol: "$"},
	{Code: "CNY", Name: "人民币", Symbol: "¥"},
	{Code: "EUR", Name: "欧元", Symbol: "€"},
	{Code: "GBP", Name: "英镑", Symbol: "£"},
	{Code: "JPY", Name: "日元", Symbol: "¥"},
	{Code: "HKD", Name: "港元", Symbol: "HK$"},
	{Code: "AUD", Name: "澳元", Symbol: "A$"},
	{Code: "CAD", Name: "加元", Symbol: "C$"},
	{Code: "SGD", Name: "新加坡元", Symbol: "S$"},
	{Code: "KRW", Name: "韩元", Symbol: "₩"},
	{Code: "TWD", Name: "新台币", Symbol: "NT$"},
	{Code: "CHF", Name: "瑞士法郎", Symbol: "CHF"},
}

// Default 系统默认币种。
const Default = "USD"

var reCode = regexp.MustCompile(`^[A-Z]{3,8}$`)

// Normalize 规范化币种代码:去空白并转大写。
func Normalize(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

// Valid 校验币种代码:3-8 位大写字母(ISO 4217 风格,预设或自定义均可)。
func Valid(code string) bool {
	return reCode.MatchString(Normalize(code))
}

// Symbol 返回币种符号;预设返回其符号,自定义返回代码本身。
func Symbol(code string) string {
	if p := PresetByCode(code); p != nil {
		return p.Symbol
	}
	return Normalize(code)
}

// Name 返回币种名称;预设返回中文名,自定义返回代码。
func Name(code string) string {
	if p := PresetByCode(code); p != nil {
		return p.Name
	}
	return Normalize(code)
}

// PresetByCode 返回指定预设,未命中返回 nil。
func PresetByCode(code string) *Preset {
	n := Normalize(code)
	for i := range Presets {
		if Presets[i].Code == n {
			return &Presets[i]
		}
	}
	return nil
}
