package evidence

import (
	"math/big"
	"strings"
	"unicode"
)

func currencySupported(currency, quote string) bool {
	return currency != "" && currencyInText(quote) == currency
}

func number(v string) (*big.Rat, bool) {
	v = strings.TrimSpace(v)
	if strings.Contains(v, ",") {
		if strings.Contains(v, ".") {
			if strings.LastIndex(v, ",") > strings.LastIndex(v, ".") {
				v = strings.ReplaceAll(v, ".", "")
				v = strings.ReplaceAll(v, ",", ".")
			} else {
				v = strings.ReplaceAll(v, ",", "")
			}
		} else {
			parts := strings.Split(v, ",")
			if len(parts[len(parts)-1]) == 3 {
				v = strings.Join(parts, "")
			} else {
				v = strings.ReplaceAll(v, ",", ".")
			}
		}
	} else if strings.Count(v, ".") > 1 || strings.Contains(v, ".") && len(v)-strings.LastIndex(v, ".")-1 == 3 {
		v = strings.ReplaceAll(v, ".", "")
	}
	r, ok := new(big.Rat).SetString(v)
	return r, ok && r.Sign() >= 0
}

func samePrice(price, text string) bool {
	var nums []string
	var b strings.Builder
	flush := func() {
		if b.Len() > 0 {
			nums = append(nums, b.String())
			b.Reset()
		}
	}
	for _, r := range text {
		if unicode.IsDigit(r) || r == '.' || r == ',' {
			b.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	p, ok := number(price)
	if !ok {
		return false
	}
	if len(nums) == 0 {
		return false
	}
	v, ok := number(nums[0])
	return ok && v.Cmp(p) == 0
}
