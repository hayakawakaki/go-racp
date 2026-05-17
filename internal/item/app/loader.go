package app

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hayakawakaki/go-racp/internal/item/domain"
	"github.com/hayakawakaki/go-racp/internal/item/infra"
	"github.com/hayakawakaki/go-racp/server/config"
)

type Sources struct {
	Logger *slog.Logger
	Root   string
	YAML   []string
	Lua    []string
}

func ParseSources(sources Sources) (*domain.Snapshot, error) {
	yamlInputs, err := loadAllYAML(sources)
	if err != nil {
		return nil, fmt.Errorf("app.ParseSources: %w", err)
	}

	luaInfos, err := loadAllLua(sources)
	if err != nil {
		return nil, fmt.Errorf("app.ParseSources: %w", err)
	}

	if len(yamlInputs) == 0 {
		return domain.EmptySnapshot(), nil
	}

	byID := map[int]*infra.YAMLInput{}
	for index := range yamlInputs {
		input := yamlInputs[index]
		byID[input.ID] = &input
	}

	items := make([]*domain.Item, 0, len(byID))
	for _, input := range byID {
		items = append(items, buildItem(input, luaInfos))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })

	snap := &domain.Snapshot{
		LoadedAt:    time.Now(),
		ByID:        make(map[int]*domain.Item, len(items)),
		ByName:      make(map[string]*domain.Item, len(items)),
		Sorted:      items,
		SourceCount: len(items),
	}
	for _, item := range items {
		snap.ByID[item.ID] = item
		snap.ByName[item.AegisName] = item
	}

	return snap, nil
}

func loadAllYAML(sources Sources) ([]infra.YAMLInput, error) {
	if len(sources.YAML) == 0 {
		return nil, nil
	}

	projectRoot, err := config.ProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("app.loadAllYAML: %w", err)
	}

	logger := loggerOrDefault(sources.Logger)

	var all []infra.YAMLInput
	for _, relative := range sources.YAML {
		path := resolvePath(projectRoot, sources.Root, relative)
		rows, err := infra.ReadYAML(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				logger.Warn("item: yaml file not found", "configured", relative, "resolved", path)

				continue
			}

			return nil, fmt.Errorf("app.loadAllYAML(%s): %w", path, err)
		}
		all = append(all, rows...)
	}

	return all, nil
}

func loadAllLua(sources Sources) (map[int]infra.LuaInfo, error) {
	if len(sources.Lua) == 0 {
		return map[int]infra.LuaInfo{}, nil
	}

	projectRoot, err := config.ProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("app.loadAllLua: %w", err)
	}

	logger := loggerOrDefault(sources.Logger)

	merged := map[int]infra.LuaInfo{}
	for _, relative := range sources.Lua {
		path := resolvePath(projectRoot, sources.Root, relative)
		entries, err := infra.ReadLua(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				logger.Warn("item: lua file not found", "configured", relative, "resolved", path)

				continue
			}

			return nil, fmt.Errorf("app.loadAllLua(%s): %w", path, err)
		}
		for id, info := range entries {
			if _, exists := merged[id]; exists {
				continue
			}
			merged[id] = info
		}
	}

	return merged, nil
}

func loggerOrDefault(logger *slog.Logger) *slog.Logger {
	if logger != nil {
		return logger
	}

	return slog.Default()
}

func resolvePath(projectRoot, configRoot, relative string) string {
	if filepath.IsAbs(relative) {
		return relative
	}
	joined := filepath.Join(configRoot, relative)
	if filepath.IsAbs(joined) {
		return joined
	}

	return filepath.Join(projectRoot, joined)
}

func buildItem(input *infra.YAMLInput, luaInfos map[int]infra.LuaInfo) *domain.Item {
	item := &domain.Item{
		ID:            input.ID,
		AegisName:     input.AegisName,
		Name:          input.Name,
		Type:          asItemType(input.Type),
		SubType:       input.SubType,
		Buy:           input.Buy,
		Sell:          sellOrDefault(input.Sell, input.Buy),
		Weight:        float64(input.Weight) / 10,
		Gender:        domain.GenderFromString(input.Gender),
		View:          input.View,
		EquipLevelMin: input.EquipLevelMin,
		EquipLevelMax: input.EquipLevelMax,
		Slots:         input.Slots,
		Attack:        input.Attack,
		MagicAttack:   input.MagicAttack,
		Defense:       input.Defense,
		WeaponLevel:   input.WeaponLevel,
		Range:         input.Range,
		ArmorLevel:    input.ArmorLevel,
		Refineable:    input.Refineable,
		Gradable:      input.Gradable,
		Jobs:          domain.JobsFromMap(input.Jobs),
		Classes:       domain.ClassesFromMap(input.Classes),
		Locations:     toLocationSet(input.Locations),
		Trade:         toItemTrade(input.Trade),
	}
	if location, ok := domain.LocationFromString(input.Location); ok {
		item.Location = location
	}
	applyLua(item, luaInfos)

	return item
}

func asItemType(name string) domain.ItemType {
	if value, ok := domain.ItemTypeFromString(name); ok {
		return value
	}

	return domain.ItemTypeUnknown
}

func sellOrDefault(sell, buy int) int {
	if sell > 0 {
		return sell
	}
	if buy > 0 {
		return buy / 2
	}

	return 0
}

func toLocationSet(input map[string]bool) domain.LocationSet {
	var set domain.LocationSet
	for name, enabled := range input {
		if !enabled {
			continue
		}
		if location, ok := domain.LocationFromString(name); ok {
			set.Set(location)
		}
	}

	return set
}

func toItemTrade(input map[string]any) *domain.ItemTrade {
	if len(input) == 0 {
		return nil
	}
	trade := &domain.ItemTrade{}
	trade.Override = asInt(input["Override"])
	trade.NoDrop = asBool(input["NoDrop"])
	trade.NoTrade = asBool(input["NoTrade"])
	trade.TradePartner = asBool(input["TradePartner"])
	trade.NoSell = asBool(input["NoSell"])
	trade.NoCart = asBool(input["NoCart"])
	trade.NoStorage = asBool(input["NoStorage"])
	trade.NoGuildStorage = asBool(input["NoGuildStorage"])
	trade.NoMail = asBool(input["NoMail"])
	trade.NoAuction = asBool(input["NoAuction"])

	return trade
}

func asInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	}

	return 0
}

func asBool(value any) bool {
	asBoolean, _ := value.(bool)

	return asBoolean
}

func applyLua(item *domain.Item, luaInfos map[int]infra.LuaInfo) {
	info, ok := luaInfos[item.ID]
	if !ok {
		item.Description = []string{"No description."}
		item.Image = "unknown"
		if item.ClientName == "" {
			item.ClientName = item.Name
		}

		return
	}
	if info.DisplayName != "" {
		item.ClientName = info.DisplayName
	} else {
		item.ClientName = item.Name
	}
	item.Image = strings.ToLower(info.Resource)
	if len(info.Description) == 0 {
		item.Description = []string{"No description."}
	} else {
		item.Description = info.Description
	}
}
