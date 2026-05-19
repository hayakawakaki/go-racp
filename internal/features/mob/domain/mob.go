package domain

type MobDrop struct {
	ItemAegis         string
	RandomOptionGroup string
	Rate              int
	Index             int
	StealProtected    bool
}

type DropOf struct {
	MobAegis string
	MobName  string
	MobID    int
	Rate     int
	IsMVP    bool
}

//nolint:govet // long-lived value, alignment over readability
type Mob struct {
	Drops           []MobDrop
	MvpDrops        []MobDrop
	AegisName       string
	Name            string
	JapaneseName    string
	Title           string
	AegisLower      string
	NameLower       string
	ID              int
	Level           int
	HP              int
	BaseExp         int
	JobExp          int
	MvpExp          int
	Attack          int
	Attack2         int
	Defense         int
	MagicDefense    int
	Resistance      int
	MagicResistance int
	Str             int
	Agi             int
	Vit             int
	Int             int
	Dex             int
	Luk             int
	AttackRange     int
	SkillRange      int
	ChaseRange      int
	WalkSpeed       int
	AttackDelay     int
	AttackMotion    int
	DamageMotion    int
	DamageTaken     int
	Race            Race
	Element         Element
	ElementLevel    uint8
	Size            Size
	Modes           ModeSet
}

func (m *Mob) IsMVP() bool {
	return m.Modes.Has(ModeMvp) || m.MvpExp > 0
}
