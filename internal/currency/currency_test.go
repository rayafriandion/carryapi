package currency

import "testing"

func TestNormalizeAndValid(t *testing.T) {
	if !Valid("USD") || !Valid("cny") || !Valid("czk") || !Valid("eur") {
		t.Error("expected valid for USD/CNY/CZK/EUR")
	}
	if Valid("") || Valid("1") || Valid("US") || Valid("AAAAAAAAA") || Valid("US$") {
		t.Error("expected invalid for bad codes")
	}
	if Normalize(" eur ") != "EUR" {
		t.Error("normalize failed")
	}
}

func TestSymbolAndName(t *testing.T) {
	if Symbol("USD") != "$" || Name("USD") != "美元" {
		t.Errorf("USD: %s %s", Symbol("USD"), Name("USD"))
	}
	if Symbol("CZK") != "CZK" || Name("CZK") != "CZK" {
		t.Errorf("CZK custom: %s %s", Symbol("CZK"), Name("CZK"))
	}
}
