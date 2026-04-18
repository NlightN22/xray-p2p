package config

import "strings"

func envKeyToPath(prefix string) func(string) string {
	return func(key string) string {
		key = strings.TrimPrefix(key, prefix)
		if key == "" {
			return ""
		}

		parts := strings.Split(key, "_")
		for i := range parts {
			parts[i] = strings.ToLower(parts[i])
		}

		if len(parts) == 1 {
			return parts[0]
		}

		return parts[0] + "." + strings.Join(parts[1:], "_")
	}
}
