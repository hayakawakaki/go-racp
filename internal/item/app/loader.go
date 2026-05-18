package app

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/hayakawakaki/go-racp/internal/item/domain"
	"github.com/hayakawakaki/go-racp/internal/item/infra"
	"github.com/hayakawakaki/go-racp/internal/refdata"
	"github.com/hayakawakaki/go-racp/server/config"
)

type ItemCache = refdata.Cache[[]*domain.Item]

type Sources struct {
	Logger *slog.Logger
	Cache  *ItemCache
	Root   string
	YAML   []string
	Lua    []string
}

func ParseSources(sources Sources) (*domain.Snapshot, error) {
	logger := loggerOrDefault(sources.Logger)
	overall := time.Now()

	yamlPaths, luaPaths, err := ResolveSourcePaths(sources)
	if err != nil {
		return nil, fmt.Errorf("app.ParseSources: %w", err)
	}
	allPaths := slices.Concat(yamlPaths, luaPaths)

	if snap, ok := tryLoadFromCache(sources.Cache, allPaths, logger); ok {
		logger.Info("item: snapshot restored from cache", "items", snap.SourceCount, "took", time.Since(overall).String())

		return snap, nil
	}

	items, err := parseAndBuildItems(sources.Logger, yamlPaths, luaPaths)
	if err != nil {
		return nil, fmt.Errorf("app.ParseSources: %w", err)
	}
	if len(items) == 0 {
		return domain.EmptySnapshot(), nil
	}

	snap := assembleSnapshot(items)
	logger.Info("item: snapshot built", "items", len(items), "total", time.Since(overall).String())

	persistCache(sources.Cache, items, allPaths, logger)

	return snap, nil
}

func ResolveSourcePaths(sources Sources) (yamlPaths, luaPaths []string, err error) {
	if len(sources.YAML) == 0 && len(sources.Lua) == 0 {
		return nil, nil, nil
	}
	projectRoot, err := config.ProjectRoot()
	if err != nil {
		return nil, nil, fmt.Errorf("app.ResolveSourcePaths: %w", err)
	}
	yamlPaths = refdata.ResolvePaths(projectRoot, sources.Root, sources.YAML)
	luaPaths = refdata.ResolvePaths(projectRoot, sources.Root, sources.Lua)

	return yamlPaths, luaPaths, nil
}

func tryLoadFromCache(cache *ItemCache, paths []string, logger *slog.Logger) (*domain.Snapshot, bool) {
	if cache == nil {
		return nil, false
	}
	items, ok := cache.Load(paths)
	if !ok || len(items) == 0 {
		return nil, false
	}
	assembleStart := time.Now()
	snap := assembleSnapshot(items)
	loggerOrDefault(logger).Info("item: snapshot assembled", "items", snap.SourceCount, "took", time.Since(assembleStart).String())

	return snap, true
}

func persistCache(cache *ItemCache, items []*domain.Item, paths []string, logger *slog.Logger) {
	if cache == nil {
		return
	}
	if err := cache.Save(items, paths); err != nil {
		logger.Warn("item: cache save failed", "err", err)
	}
}

func parseAndBuildItems(logger *slog.Logger, yamlPaths, luaPaths []string) ([]*domain.Item, error) {
	log := loggerOrDefault(logger)

	startYAML := time.Now()
	yamlInputs, err := loadAllYAML(logger, yamlPaths)
	if err != nil {
		return nil, err
	}
	log.Info("item: yaml parsed", "items", len(yamlInputs), "took", time.Since(startYAML).String())

	startLua := time.Now()
	luaInfos, err := loadAllLua(logger, luaPaths)
	if err != nil {
		return nil, err
	}
	log.Info("item: lua parsed", "entries", len(luaInfos), "took", time.Since(startLua).String())

	if len(yamlInputs) == 0 {
		return nil, nil
	}

	byID := map[int]*infra.YAMLInput{}
	for index := range yamlInputs {
		byID[yamlInputs[index].ID] = &yamlInputs[index]
	}
	items := make([]*domain.Item, 0, len(byID))
	for _, input := range byID {
		items = append(items, buildItem(input, luaInfos))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })

	return items, nil
}

func assembleSnapshot(items []*domain.Item) *domain.Snapshot {
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

	return snap
}

func loadAllYAML(logger *slog.Logger, paths []string) ([]infra.YAMLInput, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	log := loggerOrDefault(logger)

	var all []infra.YAMLInput
	for _, path := range paths {
		rows, err := infra.ReadYAML(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				log.Warn("item: yaml file not found", "resolved", path)

				continue
			}

			return nil, fmt.Errorf("app.loadAllYAML(%s): %w", path, err)
		}
		all = append(all, rows...)
	}

	return all, nil
}

func loadAllLua(logger *slog.Logger, paths []string) (map[int]infra.LuaInfo, error) {
	if len(paths) == 0 {
		return map[int]infra.LuaInfo{}, nil
	}

	log := loggerOrDefault(logger)

	merged := map[int]infra.LuaInfo{}
	for _, path := range paths {
		entries, err := infra.ReadLua(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				log.Warn("item: lua file not found", "resolved", path)

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
	applyLua(item, luaInfos)
	item.AegisNameLower = strings.ToLower(item.AegisName)
	item.ClientNameLower = strings.ToLower(item.ClientName)

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
	item.Image = info.Resource
	if len(info.Description) == 0 {
		item.Description = []string{"No description."}
	} else {
		item.Description = info.Description
	}
}
