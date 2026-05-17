package infra

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	lua "github.com/yuin/gopher-lua"
)

type LuaInfo struct {
	DisplayName string
	Resource    string
	Description []string
}

//nolint:cyclop // linear walk of Lua table, splitting hurts readability
func ReadLua(path string) (map[int]LuaInfo, error) {
	//nolint:gosec // G304: paths come from operator-controlled config.yml.
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fs.ErrNotExist
		}

		return nil, fmt.Errorf("infra.ReadLua: %w", err)
	}
	if len(data) == 0 {
		return map[int]LuaInfo{}, nil
	}

	state := lua.NewState(lua.Options{SkipOpenLibs: true})
	defer state.Close()

	if err := state.DoString(string(data)); err != nil {
		return nil, fmt.Errorf("infra.ReadLua: %w", err)
	}

	out := map[int]LuaInfo{}
	tbl, ok := state.GetGlobal("tbl").(*lua.LTable)
	if !ok {
		return out, nil
	}

	tbl.ForEach(func(key, value lua.LValue) {
		keyNumber, ok := key.(lua.LNumber)
		if !ok {
			return
		}
		entry, ok := value.(*lua.LTable)
		if !ok {
			return
		}
		info := LuaInfo{}
		if display, ok := entry.RawGetString("identifiedDisplayName").(lua.LString); ok {
			info.DisplayName = string(display)
		}
		if resource, ok := entry.RawGetString("identifiedResourceName").(lua.LString); ok {
			info.Resource = string(resource)
		}
		if description, ok := entry.RawGetString("identifiedDescriptionName").(*lua.LTable); ok {
			lines := make([]string, 0, description.Len())
			description.ForEach(func(_, line lua.LValue) {
				if asString, ok := line.(lua.LString); ok {
					lines = append(lines, string(asString))
				}
			})
			info.Description = lines
		}
		out[int(keyNumber)] = info
	})

	return out, nil
}
