// Example: a search provider. Rows appear only for queries it recognises, and Enter copies the
// converted value. It needs no commands because nothing here is durable or shortcut-able.
package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

type temperature struct{}

var (
	_ api.Extension      = temperature{}
	_ api.SearchProvider = temperature{}
)

func (temperature) Manifest() api.Manifest {
	return api.Manifest{
		APIVersion: "1.0",
		Identity:   api.Identity{ID: "acme.temperature", Title: "Temperature"},
	}
}

func (temperature) SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	value, unit, ok := parseTemperature(query)
	if !ok {
		return nil, nil
	}
	converted, target := convertTemperature(value, unit)
	copied := fmt.Sprintf("%.1f °%s", converted, target)
	return []api.Item{{
		ID:       "convert:" + unit,
		Title:    copied,
		Subtitle: fmt.Sprintf("%.1f °%s is %s", value, unit, copied),
		Icon:     "thermometer",
		Score:    10,
		Primary:  api.Primary(api.CopyText("Copy", copied, false)),
	}}, nil
}

func parseTemperature(query string) (float64, string, bool) {
	fields := strings.Fields(query)
	if len(fields) != 2 {
		return 0, "", false
	}
	value, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, "", false
	}
	unit := strings.ToUpper(fields[1])
	if unit != "C" && unit != "F" {
		return 0, "", false
	}
	return value, unit, true
}

func convertTemperature(value float64, unit string) (float64, string) {
	if unit == "C" {
		return value*9/5 + 32, "F"
	}
	return (value - 32) * 5 / 9, "C"
}
