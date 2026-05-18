package infra

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"strconv"
	"unicode/utf8"

	"golang.org/x/text/encoding/korean"
)

type LuaInfo struct {
	DisplayName string
	Resource    string
	Description []string
}

var (
	itemStartPattern       = regexp.MustCompile(`\[(\d+)\]\s*=\s*\{`)
	displayNamePattern     = regexp.MustCompile(`\bidentifiedDisplayName\s*=\s*"([^"]*)"`)
	resourceNamePattern    = regexp.MustCompile(`\bidentifiedResourceName\s*=\s*"([^"]*)"`)
	descriptionPattern     = regexp.MustCompile(`\bidentifiedDescriptionName\s*=\s*\{([\s\S]*?)\n\s*\}`)
	descriptionLinePattern = regexp.MustCompile(`"([^"]*)"`)
)

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

	decoded, err := decodeToUTF8(data)
	if err != nil {
		return nil, fmt.Errorf("infra.ReadLua: %w", err)
	}

	return parseIteminfo(decoded), nil
}

func decodeToUTF8(data []byte) (string, error) {
	if utf8.Valid(data) {
		return string(data), nil
	}
	decoded, err := korean.EUCKR.NewDecoder().Bytes(data)
	if err != nil {
		return "", fmt.Errorf("decode EUC-KR: %w", err)
	}

	return string(decoded), nil
}

func parseIteminfo(source string) map[int]LuaInfo {
	out := map[int]LuaInfo{}
	matches := itemStartPattern.FindAllStringSubmatchIndex(source, -1)
	for _, match := range matches {
		id, err := strconv.Atoi(source[match[2]:match[3]])
		if err != nil {
			continue
		}
		bodyStart := match[1]
		bodyEnd := findMatchingBrace(source, match[1]-1)
		if bodyEnd < 0 {
			continue
		}
		body := source[bodyStart:bodyEnd]
		out[id] = extractFields(body)
	}

	return out
}

func findMatchingBrace(source string, openIndex int) int {
	depth := 0
	inString := false
	escape := false
	for index := openIndex; index < len(source); index++ {
		character := source[index]
		if escape {
			escape = false

			continue
		}
		if character == '\\' {
			escape = true

			continue
		}
		if inString {
			if character == '"' {
				inString = false
			}

			continue
		}
		switch character {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return index
			}
		}
	}

	return -1
}

func extractFields(body string) LuaInfo {
	info := LuaInfo{}
	if display := displayNamePattern.FindStringSubmatch(body); len(display) >= 2 {
		info.DisplayName = display[1]
	}
	if resource := resourceNamePattern.FindStringSubmatch(body); len(resource) >= 2 {
		info.Resource = resource[1]
	}
	if description := descriptionPattern.FindStringSubmatch(body); len(description) >= 2 {
		info.Description = extractStrings(description[1])
	}

	return info
}

func extractStrings(block string) []string {
	matches := descriptionLinePattern.FindAllStringSubmatch(block, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		out = append(out, match[1])
	}

	return out
}
