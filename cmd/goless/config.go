// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type programConfig struct {
	Hidden      *bool  `json:"hidden"`
	LineNumbers *bool  `json:"line-numbers"`
	LiveLinks   *bool  `json:"live-links"`
	Secure      *bool  `json:"secure"`
	Theme       string `json:"theme"`
}

func defaultProgramConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "goless", "config.json"), nil
}

func envProgramConfigPath() string {
	return os.Getenv("GOLESS_CONFIG")
}

func programConfigPath() (string, bool, error) {
	if path := envProgramConfigPath(); path != "" {
		return path, false, nil
	}
	path, err := defaultProgramConfigPath()
	if err != nil {
		return "", false, err
	}
	return path, true, nil
}

func loadDefaultProgramConfig() (programConfig, string, error) {
	path, optional, err := programConfigPath()
	if err != nil {
		return programConfig{}, "", nil
	}
	cfg, err := loadProgramConfigAtPath(path, optional)
	return cfg, path, err
}

func loadRequiredProgramConfig(path string) (programConfig, error) {
	return loadProgramConfigAtPath(path, false)
}

func loadProgramConfigAtPath(path string, optional bool) (programConfig, error) {
	if path == "" {
		return programConfig{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if optional && errors.Is(err, os.ErrNotExist) {
			return programConfig{}, nil
		}
		return programConfig{}, fmt.Errorf("read config %q: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return programConfig{}, nil
	}

	var cfg programConfig
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return programConfig{}, fmt.Errorf("parse config %q: %w", path, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return programConfig{}, fmt.Errorf("parse config %q: unexpected trailing content", path)
		}
		return programConfig{}, fmt.Errorf("parse config %q: %w", path, err)
	}
	if cfg.Theme != "" {
		if _, err := programPreset(cfg.Theme); err != nil {
			return programConfig{}, fmt.Errorf("parse config %q: %w", path, err)
		}
	}
	return cfg, nil
}

func applyProgramConfig(opts programOptions, cfg programConfig) programOptions {
	if cfg.Hidden != nil {
		opts.hidden = *cfg.Hidden
	}
	if cfg.LineNumbers != nil {
		opts.lineNumbers = *cfg.LineNumbers
	}
	if cfg.LiveLinks != nil {
		opts.liveLinks = *cfg.LiveLinks
	}
	if cfg.Secure != nil {
		opts.secure = *cfg.Secure
	}
	if cfg.Theme != "" {
		opts.presetName = cfg.Theme
	}
	return opts
}
