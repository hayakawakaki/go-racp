package domain

type Race uint8

const (
	RaceFormless Race = iota
	RaceUndead
	RaceBrute
	RacePlant
	RaceInsect
	RaceFish
	RaceDemon
	RaceDemihuman
	RaceAngel
	RaceDragon
)

//nolint:goconst // intentional cross-enum reuse
var raceFromString = map[string]Race{
	"Formless":  RaceFormless,
	"Undead":    RaceUndead,
	"Brute":     RaceBrute,
	"Plant":     RacePlant,
	"Insect":    RaceInsect,
	"Fish":      RaceFish,
	"Demon":     RaceDemon,
	"Demihuman": RaceDemihuman,
	"Angel":     RaceAngel,
	"Dragon":    RaceDragon,
}

var raceDisplay = map[Race]string{
	RaceFormless:  "Formless",
	RaceUndead:    "Undead",
	RaceBrute:     "Brute",
	RacePlant:     "Plant",
	RaceInsect:    "Insect",
	RaceFish:      "Fish",
	RaceDemon:     "Demon",
	RaceDemihuman: "Demihuman",
	RaceAngel:     "Angel",
	RaceDragon:    "Dragon",
}

func RaceFromString(name string) (Race, bool) {
	value, ok := raceFromString[name]

	return value, ok
}

func (r Race) Display() string {
	if name, ok := raceDisplay[r]; ok {
		return name
	}

	return "Unknown" //nolint:goconst // shared fallback across enum displays
}

type Element uint8

const (
	ElementNeutral Element = iota
	ElementWater
	ElementEarth
	ElementFire
	ElementWind
	ElementPoison
	ElementHoly
	ElementDark
	ElementGhost
	ElementUndead
)

var elementFromString = map[string]Element{
	"Neutral": ElementNeutral,
	"Water":   ElementWater,
	"Earth":   ElementEarth,
	"Fire":    ElementFire,
	"Wind":    ElementWind,
	"Poison":  ElementPoison,
	"Holy":    ElementHoly,
	"Dark":    ElementDark,
	"Ghost":   ElementGhost,
	"Undead":  ElementUndead,
}

var elementDisplay = map[Element]string{
	ElementNeutral: "Neutral",
	ElementWater:   "Water",
	ElementEarth:   "Earth",
	ElementFire:    "Fire",
	ElementWind:    "Wind",
	ElementPoison:  "Poison",
	ElementHoly:    "Holy",
	ElementDark:    "Dark",
	ElementGhost:   "Ghost",
	ElementUndead:  "Undead",
}

func ElementFromString(name string) (Element, bool) {
	value, ok := elementFromString[name]

	return value, ok
}

func (e Element) Display() string {
	if name, ok := elementDisplay[e]; ok {
		return name
	}

	return "Unknown" //nolint:goconst // shared fallback across enum displays
}

type Size uint8

const (
	SizeSmall Size = iota
	SizeMedium
	SizeLarge
)

var sizeFromString = map[string]Size{
	"Small":  SizeSmall,
	"Medium": SizeMedium,
	"Large":  SizeLarge,
}

var sizeDisplay = map[Size]string{
	SizeSmall:  "Small",
	SizeMedium: "Medium",
	SizeLarge:  "Large",
}

func SizeFromString(name string) (Size, bool) {
	value, ok := sizeFromString[name]

	return value, ok
}

func (s Size) Display() string {
	if name, ok := sizeDisplay[s]; ok {
		return name
	}

	return "Unknown" //nolint:goconst // shared fallback across enum displays
}

type Mode uint8

const (
	ModeCanMove Mode = iota
	ModeLooter
	ModeAggressive
	ModeAssist
	ModeCastSensorIdle
	ModeNoRandomWalk
	ModeNoCast
	ModeCanAttack
	ModeCastSensorChase
	ModeChangeChase
	ModeAngry
	ModeChangeTargetMelee
	ModeChangeTargetChase
	ModeTargetWeak
	ModeRandomTarget
	ModeIgnoreMelee
	ModeIgnoreMagic
	ModeIgnoreRanged
	ModeMvp
	ModeIgnoreMisc
	ModeKnockBackImmune
	ModeTeleportBlock
	ModeFixedItemDrop
	ModeDetector
	ModeStatusImmune
	ModeSkillImmune
)

var modeFromString = map[string]Mode{
	"CanMove":           ModeCanMove,
	"Looter":            ModeLooter,
	"Aggressive":        ModeAggressive,
	"Assist":            ModeAssist,
	"CastSensorIdle":    ModeCastSensorIdle,
	"NoRandomWalk":      ModeNoRandomWalk,
	"NoCast":            ModeNoCast,
	"CanAttack":         ModeCanAttack,
	"CastSensorChase":   ModeCastSensorChase,
	"ChangeChase":       ModeChangeChase,
	"Angry":             ModeAngry,
	"ChangeTargetMelee": ModeChangeTargetMelee,
	"ChangeTargetChase": ModeChangeTargetChase,
	"TargetWeak":        ModeTargetWeak,
	"RandomTarget":      ModeRandomTarget,
	"IgnoreMelee":       ModeIgnoreMelee,
	"IgnoreMagic":       ModeIgnoreMagic,
	"IgnoreRanged":      ModeIgnoreRanged,
	"Mvp":               ModeMvp,
	"IgnoreMisc":        ModeIgnoreMisc,
	"KnockBackImmune":   ModeKnockBackImmune,
	"TeleportBlock":     ModeTeleportBlock,
	"FixedItemDrop":     ModeFixedItemDrop,
	"Detector":          ModeDetector,
	"StatusImmune":      ModeStatusImmune,
	"SkillImmune":       ModeSkillImmune,
}

var modeDisplay = map[Mode]string{
	ModeCanMove:           "Can Move",
	ModeLooter:            "Looter",
	ModeAggressive:        "Aggressive",
	ModeAssist:            "Assist",
	ModeCastSensorIdle:    "Cast Sensor Idle",
	ModeNoRandomWalk:      "No Random Walk",
	ModeNoCast:            "No Cast",
	ModeCanAttack:         "Can Attack",
	ModeCastSensorChase:   "Cast Sensor Chase",
	ModeChangeChase:       "Change Chase",
	ModeAngry:             "Angry",
	ModeChangeTargetMelee: "Change Target Melee",
	ModeChangeTargetChase: "Change Target Chase",
	ModeTargetWeak:        "Target Weak",
	ModeRandomTarget:      "Random Target",
	ModeIgnoreMelee:       "Ignore Melee",
	ModeIgnoreMagic:       "Ignore Magic",
	ModeIgnoreRanged:      "Ignore Ranged",
	ModeMvp:               "MVP",
	ModeIgnoreMisc:        "Ignore Misc",
	ModeKnockBackImmune:   "KnockBack Immune",
	ModeTeleportBlock:     "Teleport Block",
	ModeFixedItemDrop:     "Fixed Item Drop",
	ModeDetector:          "Detector",
	ModeStatusImmune:      "Status Immune",
	ModeSkillImmune:       "Skill Immune",
}

func ModeFromString(name string) (Mode, bool) {
	value, ok := modeFromString[name]

	return value, ok
}
