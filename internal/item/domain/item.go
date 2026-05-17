package domain

type ItemType uint8

const (
	ItemTypeUnknown ItemType = iota
	ItemTypeHealing
	ItemTypeUsable
	ItemTypeEtc
	ItemTypeWeapon
	ItemTypeArmor
	ItemTypeCard
	ItemTypePetEgg
	ItemTypePetArmor
	ItemTypeAmmo
	ItemTypeDelayConsume
	ItemTypeShadowGear
	ItemTypeCash
)

var itemTypeNames = map[ItemType]string{
	ItemTypeHealing:      "Healing",
	ItemTypeUsable:       "Usable",
	ItemTypeEtc:          "Etc",
	ItemTypeWeapon:       "Weapon",
	ItemTypeArmor:        "Armor",
	ItemTypeCard:         "Card",
	ItemTypePetEgg:       "PetEgg",
	ItemTypePetArmor:     "PetArmor",
	ItemTypeAmmo:         "Ammo",
	ItemTypeDelayConsume: "DelayConsume",
	ItemTypeShadowGear:   "ShadowGear",
	ItemTypeCash:         "Cash",
}

var itemTypeFromString = map[string]ItemType{
	"Healing":      ItemTypeHealing,
	"Usable":       ItemTypeUsable,
	"Etc":          ItemTypeEtc,
	"Weapon":       ItemTypeWeapon,
	"Armor":        ItemTypeArmor,
	"Card":         ItemTypeCard,
	"PetEgg":       ItemTypePetEgg,
	"PetArmor":     ItemTypePetArmor,
	"Ammo":         ItemTypeAmmo,
	"DelayConsume": ItemTypeDelayConsume,
	"ShadowGear":   ItemTypeShadowGear,
	"Cash":         ItemTypeCash,
}

var itemTypeDisplay = map[ItemType]string{
	ItemTypeHealing:      "Potion/Food",
	ItemTypeUsable:       "Usable",
	ItemTypeEtc:          "Etc",
	ItemTypeWeapon:       "Weapon",
	ItemTypeArmor:        "Armor",
	ItemTypeCard:         "Card",
	ItemTypePetEgg:       "Pet Egg",
	ItemTypePetArmor:     "Pet Equipment",
	ItemTypeAmmo:         "Arrow/Ammunition",
	ItemTypeDelayConsume: "Buff/Box",
	ItemTypeShadowGear:   "Shadow Equipment",
	ItemTypeCash:         "Cash Usable",
}

func (t ItemType) String() string { return itemTypeNames[t] }

func (t ItemType) Display() string {
	if name, ok := itemTypeDisplay[t]; ok {
		return name
	}

	return "Unknown Type"
}

func ItemTypeFromString(name string) (ItemType, bool) {
	value, ok := itemTypeFromString[name]

	return value, ok
}

type Gender uint8

const (
	GenderBoth Gender = iota
	GenderMale
	GenderFemale
)

var genderFromString = map[string]Gender{
	"Both":   GenderBoth,
	"Male":   GenderMale,
	"Female": GenderFemale,
}

func GenderFromString(name string) Gender {
	if value, ok := genderFromString[name]; ok {
		return value
	}

	return GenderBoth
}

type Location uint8

const (
	LocationNone Location = iota
	LocationHeadTop
	LocationHeadMid
	LocationHeadLow
	LocationArmor
	LocationRightHand
	LocationLeftHand
	LocationGarment
	LocationShoes
	LocationRightAccessory
	LocationLeftAccessory
	LocationCostumeHeadTop
	LocationCostumeHeadMid
	LocationCostumeHeadLow
	LocationCostumeGarment
	LocationAmmo
	LocationShadowArmor
	LocationShadowWeapon
	LocationShadowShield
	LocationShadowShoes
	LocationShadowRightAccessory
	LocationShadowLeftAccessory
	LocationBothHand
	LocationBothAccessory
)

var locationFromString = map[string]Location{
	"Head_Top":               LocationHeadTop,
	"Head_Mid":               LocationHeadMid,
	"Head_Low":               LocationHeadLow,
	"Armor":                  LocationArmor,
	"Right_Hand":             LocationRightHand,
	"Left_Hand":              LocationLeftHand,
	"Garment":                LocationGarment,
	"Shoes":                  LocationShoes,
	"Right_Accessory":        LocationRightAccessory,
	"Left_Accessory":         LocationLeftAccessory,
	"Costume_Head_Top":       LocationCostumeHeadTop,
	"Costume_Head_Mid":       LocationCostumeHeadMid,
	"Costume_Head_Low":       LocationCostumeHeadLow,
	"Costume_Garment":        LocationCostumeGarment,
	"Ammo":                   LocationAmmo,
	"Shadow_Armor":           LocationShadowArmor,
	"Shadow_Weapon":          LocationShadowWeapon,
	"Shadow_Shield":          LocationShadowShield,
	"Shadow_Shoes":           LocationShadowShoes,
	"Shadow_Right_Accessory": LocationShadowRightAccessory,
	"Shadow_Left_Accessory":  LocationShadowLeftAccessory,
	"Both_Hand":              LocationBothHand,
	"Both_Accessory":         LocationBothAccessory,
}

func LocationFromString(name string) (Location, bool) {
	value, ok := locationFromString[name]

	return value, ok
}

type LocationSet uint32

func (s LocationSet) Has(location Location) bool { return s&(1<<location) != 0 }
func (s *LocationSet) Set(location Location)     { *s |= 1 << location }

type JobMask uint64

type ClassMask uint16

type ItemTrade struct {
	Override       int  `yaml:"Override"`
	NoDrop         bool `yaml:"NoDrop"`
	NoTrade        bool `yaml:"NoTrade"`
	TradePartner   bool `yaml:"TradePartner"`
	NoSell         bool `yaml:"NoSell"`
	NoCart         bool `yaml:"NoCart"`
	NoStorage      bool `yaml:"NoStorage"`
	NoGuildStorage bool `yaml:"NoGuildStorage"`
	NoMail         bool `yaml:"NoMail"`
	NoAuction      bool `yaml:"NoAuction"`
}

type Item struct {
	Trade         *ItemTrade
	AegisName     string
	Name          string
	ClientName    string
	Image         string
	SubType       string
	Description   []string
	Weight        float64
	Buy           int
	Sell          int
	ID            int
	View          int
	EquipLevelMin int
	EquipLevelMax int
	Slots         int
	Attack        int
	MagicAttack   int
	Defense       int
	WeaponLevel   int
	Range         int
	ArmorLevel    int
	Jobs          JobMask
	Classes       ClassMask
	Locations     LocationSet
	Type          ItemType
	Gender        Gender
	Location      Location
	Refineable    bool
	Gradable      bool
}
