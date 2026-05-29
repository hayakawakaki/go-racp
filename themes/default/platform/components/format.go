package components

import "strconv"

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
