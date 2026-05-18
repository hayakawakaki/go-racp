package transport

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

var (
	colorPattern  = regexp.MustCompile(`\^([0-9A-Fa-f]{6})(.*?)\^0{6}`)
	tipboxPattern = regexp.MustCompile(`<TIPBOX>(.*?)<INFO>(.*?)</INFO></TIPBOX>`)
	naviPattern   = regexp.MustCompile(`<NAVI>(.*?)<INFO>([^,]*),([^,]*),([^,]*),([^,]*),([^,]*),([^,]*),([^<]*)</INFO></NAVI>`)
)

type segmentKind uint8

const (
	segmentText segmentKind = iota
	segmentHTML
)

type segment struct {
	body string
	kind segmentKind
}

func renderDescription(lines []string) string {
	var builder strings.Builder
	for index, line := range lines {
		if index > 0 {
			builder.WriteString("<br>")
		}
		for _, seg := range tokenize(line) {
			if seg.kind == segmentText {
				builder.WriteString(html.EscapeString(seg.body))
			} else {
				builder.WriteString(seg.body)
			}
		}
	}

	return builder.String()
}

func tokenize(line string) []segment {
	segments := []segment{{kind: segmentText, body: line}}
	segments = expand(segments, naviPattern, naviReplace)
	segments = expand(segments, tipboxPattern, tipboxReplace)
	segments = expand(segments, colorPattern, colorReplace)

	return segments
}

func expand(input []segment, pattern *regexp.Regexp, replace func([]string) string) []segment {
	var out []segment
	for _, seg := range input {
		if seg.kind != segmentText {
			out = append(out, seg)

			continue
		}
		out = append(out, replaceInText(seg.body, pattern, replace)...)
	}

	return out
}

func replaceInText(body string, pattern *regexp.Regexp, replace func([]string) string) []segment {
	matches := pattern.FindAllStringSubmatchIndex(body, -1)
	if len(matches) == 0 {
		return []segment{{kind: segmentText, body: body}}
	}

	var out []segment
	cursor := 0
	for _, match := range matches {
		if match[0] > cursor {
			out = append(out, segment{kind: segmentText, body: body[cursor:match[0]]})
		}
		captures := make([]string, 0, len(match)/2)
		for index := 0; index < len(match); index += 2 {
			if match[index] < 0 {
				captures = append(captures, "")

				continue
			}
			captures = append(captures, body[match[index]:match[index+1]])
		}
		out = append(out, segment{kind: segmentHTML, body: replace(captures)})
		cursor = match[1]
	}
	if cursor < len(body) {
		out = append(out, segment{kind: segmentText, body: body[cursor:]})
	}

	return out
}

func colorReplace(captures []string) string {
	return fmt.Sprintf(`<span style="color: #%s">%s</span>`, strings.ToUpper(captures[1]), html.EscapeString(captures[2]))
}

func tipboxReplace(captures []string) string {
	return fmt.Sprintf(`<span class="text-blue-400">%s</span>`, html.EscapeString(captures[1]))
}

func naviReplace(captures []string) string {
	label := html.EscapeString(captures[1])
	mapName := strings.TrimSpace(captures[2])
	x := strings.TrimSpace(captures[3])
	y := strings.TrimSpace(captures[4])
	payload := fmt.Sprintf("/navi %s %s/%s", mapName, x, y)

	return fmt.Sprintf(`<span class="navi-link cursor-pointer underline text-blue-400" data-navi=%q>%s</span>`, payload, label)
}
