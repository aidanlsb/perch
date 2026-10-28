package cli

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/aidanlsb/perch/internal/config"
)

var layoutKey = regexp.MustCompile(`(?:^|,)(kind|group_by|group-by|granularity|sort|filter|fields|view|name|source|type|query|id|bind|inputs|split)=`)

func parseKVList(vals []string) (map[string]string, error) {
	out := map[string]string{}
	for _, raw := range vals {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		pairs, err := splitSimpleKV(raw)
		if err != nil {
			return nil, err
		}
		for k, v := range pairs {
			out[k] = v
		}
	}
	return out, nil
}

func splitSimpleKV(s string) (map[string]string, error) {
	out := map[string]string{}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		k, v, ok := strings.Cut(part, "=")
		if !ok || strings.TrimSpace(k) == "" {
			return nil, fail(CodeInvalidInput, "expected k=v, got "+part)
		}
		out[strings.TrimSpace(k)] = v
	}
	return out, nil
}

func splitLayoutKV(s string) (map[string]string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return map[string]string{}, nil
	}
	out := map[string]string{}
	start := 0
	for start < len(s) {
		eq := strings.IndexByte(s[start:], '=')
		if eq < 0 {
			return nil, fail(CodeInvalidInput, "expected k=v, got "+s[start:])
		}
		key := strings.TrimSpace(s[start : start+eq])
		if key == "" {
			return nil, fail(CodeInvalidInput, "empty key in "+s)
		}
		rest := s[start+eq+1:]
		val, consumed := nextLayoutValue(rest)
		out[strings.ReplaceAll(key, "-", "_")] = val
		start = start + eq + 1 + consumed
		if start < len(s) && s[start] == ',' {
			start++
		}
	}
	return out, nil
}

func nextLayoutValue(rest string) (string, int) {
	if rest == "" {
		return "", 0
	}
	idx := -1
	for _, m := range layoutKey.FindAllStringIndex(","+rest, -1) {
		if m[0] == 0 {
			continue
		}
		idx = m[0] - 1
		break
	}
	if idx < 0 {
		return strings.TrimSpace(rest), len(rest)
	}
	return strings.TrimSpace(rest[:idx]), idx
}

func parsePresent(s string) (config.Present, string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return config.Present{Kind: config.KindList}, "", nil
	}
	if !strings.Contains(s, "=") {
		kind := strings.ToLower(s)
		switch kind {
		case config.KindList, config.KindBoard, config.KindDate:
			return config.Present{Kind: kind}, "", nil
		default:
			return config.Present{}, s, nil
		}
	}
	kv, err := splitLayoutKV(s)
	if err != nil {
		return config.Present{}, "", err
	}
	p := config.Present{
		Kind:        kv["kind"],
		GroupBy:     firstKV(kv, "group_by", "group-by"),
		Granularity: kv["granularity"],
		Sort:        kv["sort"],
		Filter:      kv["filter"],
	}
	if f := kv["fields"]; f != "" {
		p.Fields = splitFields(f)
	}
	return p, "", nil
}

func firstKV(kv map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := kv[k]; v != "" {
			return v
		}
	}
	return ""
}

func splitFields(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	sep := ","
	if strings.Contains(s, "+") && !strings.Contains(s, ",") {
		sep = "+"
	}
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parsePanelFlag(s string) (config.Panel, error) {
	kv, err := splitLayoutKV(s)
	if err != nil {
		return config.Panel{}, err
	}
	p := config.Panel{
		View:  kv["view"],
		Type:  kv["type"],
		Query: kv["query"],
		Bind:  kv["bind"],
		Split: kv["split"],
		ID:    kv["id"],
		Name:  kv["name"],
	}
	if raw := kv["inputs"]; raw != "" {
		ins, err := splitSimpleKV(raw)
		if err != nil {
			return config.Panel{}, err
		}
		p.Inputs = ins
	}
	if kind := kv["kind"]; kind != "" || kv["group_by"] != "" || kv["fields"] != "" || kv["sort"] != "" || kv["filter"] != "" || kv["granularity"] != "" {
		p.Present = config.Present{
			Kind:        kind,
			GroupBy:     firstKV(kv, "group_by", "group-by"),
			Granularity: kv["granularity"],
			Sort:        kv["sort"],
			Filter:      kv["filter"],
		}
		if f := kv["fields"]; f != "" {
			p.Fields = splitFields(f)
		}
	}
	if p.View == "" && p.Type == "" && p.Query == "" && p.Split == "" && len(p.Panels) == 0 {
		return config.Panel{}, fail(CodeInvalidInput, "panel needs view=, type=, query=, or split=")
	}
	return p, nil
}

func parseLayoutSnippet(raw string) (config.Layout, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return config.Layout{}, fail(CodeInvalidInput, "empty --from snippet")
	}
	if raw == "-" {
		b, err := os.ReadFile("/dev/stdin")
		if err != nil {
			return config.Layout{}, fail(CodeFileReadError, err.Error())
		}
		raw = string(b)
	} else if looksLikePath(raw) {
		if b, err := os.ReadFile(raw); err == nil {
			raw = string(b)
		}
	}
	raw = strings.TrimSpace(raw)
	var l config.Layout
	if strings.HasPrefix(raw, "{") {
		if err := json.Unmarshal([]byte(raw), &l); err != nil {
			return config.Layout{}, fail(CodeInvalidJSON, err.Error())
		}
		return l, nil
	}
	if _, err := toml.Decode(raw, &l); err != nil {
		return config.Layout{}, fail(CodeInvalidInput, err.Error())
	}
	return l, nil
}

func looksLikePath(s string) bool {
	if strings.Contains(s, "\n") {
		return false
	}
	if strings.Contains(s, "/") || strings.HasSuffix(s, ".toml") || strings.HasSuffix(s, ".json") {
		return true
	}
	if _, err := os.Stat(s); err == nil {
		return true
	}
	return false
}
