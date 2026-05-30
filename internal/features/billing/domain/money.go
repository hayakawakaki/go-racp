package domain

import (
	"fmt"
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
