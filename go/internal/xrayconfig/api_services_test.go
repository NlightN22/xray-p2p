package xrayconfig

import (
	"reflect"
	"testing"
)

func TestSupportedAPIServicesFiltersUnsupportedServices(t *testing.T) {
	got := SupportedAPIServices([]string{
		"HandlerService",
		"ReflectionService",
		"RoutingService",
		"ObservatoryService",
		"StatsService",
		"HandlerService",
		"LoggerService",
	})
	want := []string{"HandlerService", "RoutingService", "StatsService", "LoggerService"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("services = %v, want %v", got, want)
	}
}
