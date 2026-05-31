package helpers

import (
	"strconv"
	"strings"
)

var currencySign = map[string]string{
	"USD": "$",
	"EUR": "€",
	"JPY": "¥",
}

func FormatAmount(value int64) string {
	if value < 0 {
		return strconv.FormatInt(value, 10)
	}

	digits := strconv.FormatInt(value, 10)
	count := len(digits)
	out := make([]byte, 0, count+(count-1)/3)
	for index := range count {
		if index > 0 && (count-index)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, digits[index])
	}

	return string(out)
}

func FormatPrice(value int64, currency string) string {
	code := strings.ToUpper(currency)
	amount := FormatAmount(value)
	if sign, ok := currencySign[code]; ok {
		return sign + amount + " " + code
	}

	return amount + " " + code
}
