package domain

import "strings"

func LookupSprite(aegis string) (string, bool) {
	sprite, ok := spriteByAegis[strings.ToLower(aegis)]

	return sprite, ok
}
