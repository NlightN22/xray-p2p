package xrayconfig

import "strings"

var supportedAPIServices = map[string]struct{}{
	"HandlerService": {},
	"RoutingService": {},
	"StatsService":   {},
	"LoggerService":  {},
}

func SupportedAPIServices(services []string) []string {
	result := make([]string, 0, len(services))
	seen := make(map[string]struct{}, len(services))
	for _, service := range services {
		service = strings.TrimSpace(service)
		if service == "" {
			continue
		}
		if _, ok := supportedAPIServices[service]; !ok {
			continue
		}
		key := strings.ToLower(service)
		if _, ok := seen[key]; ok {
			continue
		}
		result = append(result, service)
		seen[key] = struct{}{}
	}
	return result
}
