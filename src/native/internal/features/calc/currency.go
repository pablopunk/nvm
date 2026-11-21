package calc

import (
	"regexp"
	"strings"
)

// currencyAliases maps symbols, names and codes (normalized) to ISO codes. Any other three-letter word is taken as
// a code as well; see currencyForAlias.
var currencyAliases = buildCurrencyAliases()

// buildCurrencyAliases declares the named currencies in the TypeScript original's order.
func buildCurrencyAliases() map[string]string {
	aliases := map[string]string{}
	add := func(code string, names ...string) {
		aliases[strings.ToLower(code)] = code
		for _, name := range names {
			aliases[normalizeCurrencyAlias(name)] = code
		}
	}
	add("USD", "$", "dollar", "dollars", "usd")
	add("EUR", "€", "euro", "euros", "eur")
	add("GBP", "£", "pound", "pounds", "sterling", "gbp")
	add("JPY", "¥", "yen", "jpy")
	add("INR", "₹", "rupee", "rupees", "inr")
	add("CHF", "franc", "francs", "chf")
	add("CAD", "cad")
	add("AUD", "aud")
	add("NZD", "nzd")
	add("CNY", "yuan", "rmb", "cny")
	return aliases
}

// normalizeCurrencyAlias trims, lower-cases and drops dots from a currency name or code.
func normalizeCurrencyAlias(alias string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(alias)), ".", "")
}

// threeLetterCode matches a bare three-letter word, which is taken as an ISO code even when it is not listed.
var threeLetterCode = regexp.MustCompile(`^[a-z]{3}$`)

// currencyForAlias resolves a currency name, symbol or code to its ISO code. Unknown words yield false unless they
// are three letters.
func currencyForAlias(alias string) (string, bool) {
	normalized := normalizeCurrencyAlias(alias)
	if threeLetterCode.MatchString(normalized) {
		return strings.ToUpper(normalized), true
	}
	code, ok := currencyAliases[normalized]
	return code, ok
}

// defaultTargetCurrency is the currency a bare amount converts into: dollars become euros and euros become dollars.
func defaultTargetCurrency(source string) (string, bool) {
	switch source {
	case "USD":
		return "EUR", true
	case "EUR":
		return "USD", true
	}
	return "", false
}

// currencySymbols are the en-US symbols for the currencies with one. Other codes are shown as the code.
var currencySymbols = map[string]string{
	"USD": "$",
	"EUR": "€",
	"GBP": "£",
	"JPY": "¥",
	"INR": "₹",
	"CNY": "CN¥",
	"CAD": "CA$",
	"AUD": "A$",
	"NZD": "NZ$",
}

// currencyText prefixes an absolute amount with the currency's symbol or code, as the en-US currency style does.
func currencyText(code, amount string) string {
	if symbol, ok := currencySymbols[code]; ok {
		return symbol + amount
	}
	return code + " " + amount
}

// currencyFractionDigits is the number of decimals a currency is usually written with.
func currencyFractionDigits(code string) int {
	if code == "JPY" {
		return 0
	}
	return 2
}

// normalizeRateUnit turns "hours" and "hour" into "hour", the form the rate result prints.
func normalizeRateUnit(unitName string) string {
	normalized := strings.ToLower(strings.TrimSpace(unitName))
	if normalized == "hours" {
		return "hour"
	}
	return strings.TrimSuffix(normalized, "s")
}
