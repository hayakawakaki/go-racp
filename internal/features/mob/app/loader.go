package app

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/mob/domain"
	"github.com/hayakawakaki/go-racp/internal/features/mob/infra"
	refdata "github.com/hayakawakaki/go-racp/internal/platform/refdata"
	"github.com/hayakawakaki/go-racp/server/config"
)

type MobCache = refdata.Cache[[]*domain.Mob]

type Sources struct {
	Logger  *slog.Logger
	Cache   *MobCache
	BaseDir string
	YAML    refdata.SourceGroup
}

func ParseSources(sources Sources) (*domain.Snapshot, error) {
	logger := cmp.Or(sources.Logger, slog.Default())
	overall := time.Now()

	yamlPaths, err := ResolveSourcePaths(sources)
	if err != nil {
		return nil, fmt.Errorf("app.ParseSources: %w", err)
	}

	if snap, ok := tryLoadFromCache(sources.Cache, yamlPaths, logger); ok {
		logger.Info("mob: snapshot restored from cache", "mobs", snap.SourceCount, "took", time.Since(overall).String())
		return snap, nil
	}

	mobs, err := parseAndBuildMobs(sources.Logger, yamlPaths)
	if err != nil {
		return nil, fmt.Errorf("app.ParseSources: %w", err)
	}
	if len(mobs) == 0 {
		return domain.EmptySnapshot(), nil
	}

	snap := assembleSnapshot(mobs)
	logger.Info("mob: snapshot built", "mobs", len(mobs), "total", time.Since(overall).String())

	persistCache(sources.Cache, mobs, yamlPaths, logger)

	return snap, nil
}

func ResolveSourcePaths(sources Sources) ([]string, error) {
	if len(sources.YAML.Files) == 0 {
		return nil, nil
	}
	projectRoot, err := config.ProjectRootForBase(sources.BaseDir)
	if err != nil {
		return nil, fmt.Errorf("app.ResolveSourcePaths: %w", err)
	}

	return refdata.ResolvePaths(projectRoot, sources.BaseDir, sources.YAML.Files), nil
}

func tryLoadFromCache(cache *MobCache, paths []string, logger *slog.Logger) (*domain.Snapshot, bool) {
	if cache == nil {
		return nil, false
	}
	mobs, ok := cache.Load(paths)
	if !ok || len(mobs) == 0 {
		return nil, false
	}
	start := time.Now()
	snap := assembleSnapshot(mobs)
	cmp.Or(logger, slog.Default()).Info("mob: snapshot assembled", "mobs", snap.SourceCount, "took", time.Since(start).String())

	return snap, true
}

func persistCache(cache *MobCache, mobs []*domain.Mob, paths []string, logger *slog.Logger) {
	if cache == nil {
		return
	}
	if err := cache.Save(mobs, paths); err != nil {
		logger.Warn("mob: cache save failed", "err", err)
	}
}

func parseAndBuildMobs(logger *slog.Logger, yamlPaths []string) ([]*domain.Mob, error) {
	log := cmp.Or(logger, slog.Default())

	start := time.Now()
	inputs, err := loadAllYAML(logger, yamlPaths)
	if err != nil {
		return nil, err
	}
	log.Info("mob: yaml parsed", "mobs", len(inputs), "took", time.Since(start).String())

	if len(inputs) == 0 {
		return nil, nil
	}

	byID := map[int]*infra.YAMLInput{}
	for index := range inputs {
		byID[inputs[index].ID] = &inputs[index]
	}
	mobs := make([]*domain.Mob, 0, len(byID))
	for _, input := range byID {
		mobs = append(mobs, buildMob(input))
	}
	sort.Slice(mobs, func(i, j int) bool { return mobs[i].ID < mobs[j].ID })

	return mobs, nil
}

func assembleSnapshot(mobs []*domain.Mob) *domain.Snapshot {
	snap := &domain.Snapshot{
		LoadedAt:    time.Now(),
		ByID:        make(map[int]*domain.Mob, len(mobs)),
		ByAegis:     make(map[string]*domain.Mob, len(mobs)),
		DroppedBy:   make(map[string][]domain.DropOf, len(mobs)),
		Sorted:      mobs,
		SourceCount: len(mobs),
	}
	for _, mob := range mobs {
		snap.ByID[mob.ID] = mob
		snap.ByAegis[mob.AegisLower] = mob
		appendDrops(snap.DroppedBy, mob, mob.Drops, false)
		appendDrops(snap.DroppedBy, mob, mob.MvpDrops, true)
	}
	for key := range snap.DroppedBy {
		entries := snap.DroppedBy[key]
		sort.Slice(entries, func(i, j int) bool { return entries[i].Rate > entries[j].Rate })
		snap.DroppedBy[key] = entries
	}

	return snap
}

func appendDrops(index map[string][]domain.DropOf, mob *domain.Mob, drops []domain.MobDrop, isMVP bool) {
	for _, drop := range drops {
		if drop.ItemAegis == "" {
			continue
		}
		key := strings.ToLower(drop.ItemAegis)
		index[key] = append(index[key], domain.DropOf{
			MobID:    mob.ID,
			MobAegis: mob.AegisName,
			MobName:  mob.Name,
			Rate:     drop.Rate,
			IsMVP:    isMVP,
		})
	}
}

func loadAllYAML(logger *slog.Logger, paths []string) ([]infra.YAMLInput, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	log := cmp.Or(logger, slog.Default())

	var all []infra.YAMLInput
	for _, path := range paths {
		rows, err := infra.ReadYAML(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				log.Warn("mob: yaml file not found", "resolved", path)
				continue
			}
			return nil, fmt.Errorf("app.loadAllYAML(%s): %w", path, err)
		}
		all = append(all, rows...)
	}

	return all, nil
}

func buildMob(input *infra.YAMLInput) *domain.Mob {
	mob := &domain.Mob{
		ID:              input.ID,
		AegisName:       input.AegisName,
		Name:            input.Name,
		JapaneseName:    input.JapaneseName,
		Title:           input.Title,
		Level:           input.Level,
		HP:              input.HP,
		BaseExp:         input.BaseExp,
		JobExp:          input.JobExp,
		MvpExp:          input.MvpExp,
		Attack:          input.Attack,
		Attack2:         input.Attack2,
		Defense:         input.Defense,
		MagicDefense:    input.MagicDefense,
		Resistance:      input.Resistance,
		MagicResistance: input.MagicResistance,
		Str:             input.Str,
		Agi:             input.Agi,
		Vit:             input.Vit,
		Int:             input.Int,
		Dex:             input.Dex,
		Luk:             input.Luk,
		AttackRange:     input.AttackRange,
		SkillRange:      input.SkillRange,
		ChaseRange:      input.ChaseRange,
		WalkSpeed:       input.WalkSpeed,
		AttackDelay:     input.AttackDelay,
		AttackMotion:    input.AttackMotion,
		DamageMotion:    input.DamageMotion,
		DamageTaken:     input.DamageTaken,
		Race:            asRace(input.Race),
		Element:         asElement(input.Element),
		Size:            asSize(input.Size),
		ElementLevel:    uint8(input.ElementLevel), //nolint:gosec // bounded YAML value
		Modes:           domain.ModesFromMap(input.Modes),
		Drops:           toMobDrops(input.Drops),
		MvpDrops:        toMobDrops(input.MvpDrops),
	}
	mob.AegisLower = strings.ToLower(mob.AegisName)
	mob.NameLower = strings.ToLower(mob.Name)

	return mob
}

func asRace(name string) domain.Race {
	if value, ok := domain.RaceFromString(name); ok {
		return value
	}

	return domain.RaceFormless
}

func asElement(name string) domain.Element {
	if value, ok := domain.ElementFromString(name); ok {
		return value
	}

	return domain.ElementNeutral
}

func asSize(name string) domain.Size {
	if value, ok := domain.SizeFromString(name); ok {
		return value
	}

	return domain.SizeMedium
}

func toMobDrops(inputs []infra.DropInput) []domain.MobDrop {
	if len(inputs) == 0 {
		return nil
	}
	out := make([]domain.MobDrop, 0, len(inputs))
	for _, drop := range inputs {
		out = append(out, domain.MobDrop{
			ItemAegis:         drop.Item,
			RandomOptionGroup: drop.RandomOptionGroup,
			Rate:              drop.Rate,
			Index:             drop.Index,
			StealProtected:    drop.StealProtected,
		})
	}

	return out
}
