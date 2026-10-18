// Package config loads the exporter's lightweight JSON configuration file.
package config

import (
	"encoding/json"
	"fmt"
	"os"
)

const (
	DefaultPort            = 9101
	DefaultMetricNamespace = "temperature"
	DefaultMetricSubsystem = "temper"
)

// Config holds the exporter's runtime settings.
type Config struct {
	Port            int    `json:"port"`
	MetricNamespace string `json:"metric_namespace"`
	MetricSubsystem string `json:"metric_subsystem"`
}

// fileConfig mirrors Config but with optional fields, so a config file only
// needs to specify the settings it wants to override.
type fileConfig struct {
	Port            *int    `json:"port"`
	MetricNamespace *string `json:"metric_namespace"`
	MetricSubsystem *string `json:"metric_subsystem"`
}

// Load reads the config file at path and overlays it onto the defaults. If
// the file does not exist, the defaults are returned unchanged.
func Load(path string) (*Config, error) {
	cfg := &Config{
		Port:            DefaultPort,
		MetricNamespace: DefaultMetricNamespace,
		MetricSubsystem: DefaultMetricSubsystem,
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading config file %q: %w", path, err)
	}

	var fc fileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("parsing config file %q: %w", path, err)
	}

	if fc.Port != nil {
		cfg.Port = *fc.Port
	}
	if fc.MetricNamespace != nil {
		cfg.MetricNamespace = *fc.MetricNamespace
	}
	if fc.MetricSubsystem != nil {
		cfg.MetricSubsystem = *fc.MetricSubsystem
	}

	return cfg, nil
}
