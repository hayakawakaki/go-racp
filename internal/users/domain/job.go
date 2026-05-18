package domain

import "fmt"

var jobNames = map[int]string{
	0:    "Novice",
	1:    "Swordman",
	2:    "Mage",
	3:    "Archer",
	4:    "Acolyte",
	5:    "Merchant",
	6:    "Thief",
	7:    "Knight",
	8:    "Priest",
	9:    "Wizard",
	10:   "Blacksmith",
	11:   "Hunter",
	12:   "Assassin",
	14:   "Crusader",
	15:   "Monk",
	16:   "Sage",
	17:   "Rogue",
	18:   "Alchemist",
	19:   "Bard",
	20:   "Dancer",
	23:   "Super Novice",
	24:   "Gunslinger",
	25:   "Ninja",
	4001: "Novice High",
	4002: "Swordman High",
	4003: "Mage High",
	4004: "Archer High",
	4005: "Acolyte High",
	4006: "Merchant High",
	4007: "Thief High",
}

func JobName(classID int) string {
	if name, ok := jobNames[classID]; ok {
		return name
	}

	return fmt.Sprintf("class_%d", classID)
}
