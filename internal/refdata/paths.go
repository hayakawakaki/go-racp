package refdata

import "path/filepath"

func ResolvePath(projectRoot, configRoot, relative string) string {
	if filepath.IsAbs(relative) {
		return relative
	}
	joined := filepath.Join(configRoot, relative)
	if filepath.IsAbs(joined) {
		return joined
	}

	return filepath.Join(projectRoot, joined)
}

func ResolvePaths(projectRoot, configRoot string, relatives []string) []string {
	if len(relatives) == 0 {
		return nil
	}
	out := make([]string, 0, len(relatives))
	for _, relative := range relatives {
		out = append(out, ResolvePath(projectRoot, configRoot, relative))
	}

	return out
}
