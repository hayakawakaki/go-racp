package domain

var jobBitFromString = map[string]uint64{
	"All":           1 << 0,
	"Acolyte":       1 << 1,
	"Alchemist":     1 << 2,
	"Archer":        1 << 3,
	"Assassin":      1 << 4,
	"BardDancer":    1 << 5,
	"Blacksmith":    1 << 6,
	"Crusader":      1 << 7,
	"Gunslinger":    1 << 8,
	"Hunter":        1 << 9,
	"KagerouOboro":  1 << 10,
	"Knight":        1 << 11,
	"Mage":          1 << 12,
	"Merchant":      1 << 13,
	"Monk":          1 << 14,
	"Ninja":         1 << 15,
	"Novice":        1 << 16,
	"Priest":        1 << 17,
	"Rebellion":     1 << 18,
	"Rogue":         1 << 19,
	"Sage":          1 << 20,
	"SoulLinker":    1 << 21,
	"StarGladiator": 1 << 22,
	"Summoner":      1 << 23,
	"SuperNovice":   1 << 24,
	"Swordman":      1 << 25,
	"Taekwon":       1 << 26,
	"Thief":         1 << 27,
	"Wizard":        1 << 28,
}

var classBitFromString = map[string]uint16{
	"All":         1 << 0,
	"Normal":      1 << 1,
	"Upper":       1 << 2,
	"Baby":        1 << 3,
	"Third":       1 << 4,
	"Third_Upper": 1 << 5,
	"Third_Baby":  1 << 6,
	"Fourth":      1 << 7,
	"All_Upper":   1 << 8,
	"All_Baby":    1 << 9,
	"All_Third":   1 << 10,
}

func JobsFromMap(input map[string]bool) JobMask {
	var out JobMask
	for name, enabled := range input {
		if !enabled {
			continue
		}
		if bit, ok := jobBitFromString[name]; ok {
			out |= JobMask(bit)
		}
	}

	return out
}

func ClassesFromMap(input map[string]bool) ClassMask {
	var out ClassMask
	for name, enabled := range input {
		if !enabled {
			continue
		}
		if bit, ok := classBitFromString[name]; ok {
			out |= ClassMask(bit)
		}
	}

	return out
}
