package ui

type SpriteService struct {
	Item func(id int) string
	Mob  func(id int) string
}

var Sprites = SpriteService{
	Item: func(int) string { return "" },
	Mob:  func(int) string { return "" },
}

func itemSpriteFile(id int) string {
	if name := Sprites.Item(id); name != "" {
		return name
	}

	return "unknown"
}

func mobSpriteFile(id int) string {
	if name := Sprites.Mob(id); name != "" {
		return name
	}

	return "unknown"
}
