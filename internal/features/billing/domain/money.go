package domain

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
)

var minorUnitExponent = map[string]int64{
	"USD": 100,
	"EUR": 100,
	"JPY": 1,
}

func minorFactor(currency string) (int64, error) {
	factor, ok := minorUnitExponent[strings.ToUpper(currency)]
	if !ok {
		return 0, fmt.Errorf("billing: unsupported currency %q", currency)
	}

	return factor, nil
}

func ToMinorUnits(amount int64, currency string) (int64, error) {
	factor, err := minorFactor(currency)
	if err != nil {
		return 0, err
	}

	return amount * factor, nil
}

func ToDecimalString(amount int64, currency string) (string, error) {
	factor, err := minorFactor(currency)
	if err != nil {
		return "", err
	}

	if factor == 1 {
		return strconv.FormatInt(amount, 10), nil
	}

	decimals := len(strconv.FormatInt(factor, 10)) - 1

	return fmt.Sprintf("%d.%0*d", amount, decimals, 0), nil
}

func IsSupportedCurrency(currency string) bool {
	_, err := minorFactor(currency)

	return err == nil
}

func SupportedCurrencies() []string {
	return slices.Sorted(maps.Keys(minorUnitExponent))
}
