package domain

type ModeSet uint32

func (s ModeSet) Has(m Mode) bool { return s&(1<<m) != 0 }
func (s *ModeSet) Set(m Mode)     { *s |= 1 << m }

func (s ModeSet) Display() []string {
	var out []string
	for m := ModeCanMove; m <= ModeSkillImmune; m++ {
		if !s.Has(m) {
			continue
		}
		if name, ok := modeDisplay[m]; ok {
			out = append(out, name)
		}
	}

	return out
}

func ModesFromMap(input map[string]bool) ModeSet {
	var out ModeSet
	for name, enabled := range input {
		if !enabled {
			continue
		}
		if mode, ok := modeFromString[name]; ok {
			out.Set(mode)
		}
	}

	return out
}
