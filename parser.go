package fbia

import (
	"encoding/json"
	"strings"
)

// parseFBJSON extracts the first JSON object from a Facebook GraphQL response.
// Facebook prepends "for (;;);" and may return multiple newline-separated JSON objects.
func parseFBJSON(body string) (map[string]any, error) {
	text := strings.TrimSpace(body)
	text = strings.TrimPrefix(text, "for (;;);")
	first, _, _ := strings.Cut(text, "\n")

	var result map[string]any
	err := json.Unmarshal([]byte(strings.TrimSpace(first)), &result)
	return result, err
}

// extractDataBlocks finds all top-level JSON objects that follow a "data" key
// in raw Facebook response text. Facebook's timeline/group responses contain
// multiple concatenated JSON payloads with inconsistent structure; brace-depth
// matching is the only reliable extraction method.
func extractDataBlocks(raw string) []map[string]any {
	raw = strings.ReplaceAll(raw, "for (;;);", "")
	raw = strings.TrimSpace(raw)

	var blocks []map[string]any
	i := 0
	n := len(raw)

	for i < n {
		idx := strings.Index(raw[i:], `"data"`)
		if idx == -1 {
			break
		}
		idx += i

		braceStart := strings.Index(raw[idx:], "{")
		if braceStart == -1 {
			break
		}
		braceStart += idx

		depth := 0
		matched := false
		for j := braceStart; j < n; j++ {
			switch raw[j] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					var block map[string]any
					if err := json.Unmarshal([]byte(raw[braceStart:j+1]), &block); err == nil {
						delete(block, "errors")
						delete(block, "extensions")
						blocks = append(blocks, block)
					}
					i = j + 1
					matched = true
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			break
		}
	}

	return blocks
}

// jsonNav traverses a nested map by successive keys.
// Returns nil if any key is missing or the value is not a map.
func jsonNav(m map[string]any, keys ...string) map[string]any {
	cur := m
	for _, k := range keys {
		if cur == nil {
			return nil
		}
		next, ok := cur[k].(map[string]any)
		if !ok {
			return nil
		}
		cur = next
	}
	return cur
}

func jsonStr(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, _ := m[key].(string)
	return v
}

func jsonFloat(m map[string]any, key string) float64 {
	if m == nil {
		return 0
	}
	v, _ := m[key].(float64)
	return v
}

func jsonSlice(m map[string]any, key string) []any {
	if m == nil {
		return nil
	}
	v, _ := m[key].([]any)
	return v
}

func jsonMap(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	v, _ := m[key].(map[string]any)
	return v
}
