// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadProgramConfigMissing(t *testing.T) {
	cfg, err := loadProgramConfigAtPath(filepath.Join(t.TempDir(), "missing.json"), true)
	if err != nil {
		t.Fatalf("loadProgramConfig(missing) failed: %v", err)
	}
	if cfg != (programConfig{}) {
		t.Fatalf("loadProgramConfig(missing) = %+v, want zero config", cfg)
	}
}

func TestLoadProgramConfigRejectsUnknownFields(t *testing.T) {
	path := writeTestProgramConfig(t, `{"bogus":true}`)

	_, err := loadProgramConfigAtPath(path, true)
	if err == nil {
		t.Fatal("loadProgramConfig(...) = nil error, want parse error")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Fatalf("loadProgramConfig(...) error = %q, want unknown field detail", err)
	}
}

func TestLoadProgramConfigRejectsInvalidPreset(t *testing.T) {
	path := writeTestProgramConfig(t, `{"theme":"bogus"}`)

	_, err := loadProgramConfigAtPath(path, true)
	if err == nil {
		t.Fatal("loadProgramConfig(...) = nil error, want theme validation error")
	}
	if !strings.Contains(err.Error(), "unknown theme") {
		t.Fatalf("loadProgramConfig(...) error = %q, want theme validation detail", err)
	}
}

func TestParseProgramFlagsLoadsDefaultProgramConfig(t *testing.T) {
	setTestProgramConfigHome(t)
	writeDefaultTestProgramConfig(t, `{"theme":"dark","hidden":true,"line-numbers":true,"live-links":true,"secure":true}`)

	var out bytes.Buffer
	opts, args, err := parseProgramFlags([]string{"sample.txt"}, &out)
	if err != nil {
		t.Fatalf("parseProgramFlags(...) failed: %v", err)
	}
	if got, want := opts.presetName, "dark"; got != want {
		t.Fatalf("theme = %q, want %q", got, want)
	}
	if !opts.hidden {
		t.Fatal("hidden = false, want true from config")
	}
	if !opts.lineNumbers {
		t.Fatal("lineNumbers = false, want true from config")
	}
	if !opts.liveLinks {
		t.Fatal("liveLinks = false, want true from config")
	}
	if !opts.secure {
		t.Fatal("secure = false, want true from config")
	}
	if got, want := len(args), 1; got != want {
		t.Fatalf("len(args) = %d, want %d", got, want)
	}
}

func TestParseProgramFlagsCLIOverridesProgramConfig(t *testing.T) {
	setTestProgramConfigHome(t)
	writeDefaultTestProgramConfig(t, `{"theme":"dark","hidden":true,"line-numbers":true,"live-links":true,"secure":true}`)

	var out bytes.Buffer
	opts, _, err := parseProgramFlags([]string{"-theme", "light", "-N=false", "-hidden=false", "-live-links=false", "-secure=false"}, &out)
	if err != nil {
		t.Fatalf("parseProgramFlags(...) failed: %v", err)
	}
	if got, want := opts.presetName, "light"; got != want {
		t.Fatalf("theme = %q, want %q", got, want)
	}
	if opts.hidden {
		t.Fatal("hidden = true, want CLI override false")
	}
	if opts.lineNumbers {
		t.Fatal("lineNumbers = true, want CLI override false")
	}
	if opts.liveLinks {
		t.Fatal("liveLinks = true, want CLI override false")
	}
	if opts.secure {
		t.Fatal("secure = true, want CLI override false")
	}
}

func TestParseProgramFlagsExplicitConfigOverridesDefaultPath(t *testing.T) {
	setTestProgramConfigHome(t)
	writeDefaultTestProgramConfig(t, `{"theme":"dark","line-numbers":true}`)
	explicitPath := writeTestProgramConfig(t, `{"theme":"light","hidden":true}`)

	var out bytes.Buffer
	opts, _, err := parseProgramFlags([]string{"-config", explicitPath}, &out)
	if err != nil {
		t.Fatalf("parseProgramFlags(...) failed: %v", err)
	}
	if got, want := opts.configPath, explicitPath; got != want {
		t.Fatalf("configPath = %q, want %q", got, want)
	}
	if got, want := opts.presetName, "light"; got != want {
		t.Fatalf("theme = %q, want %q", got, want)
	}
	if !opts.hidden {
		t.Fatal("hidden = false, want true from explicit config")
	}
	if opts.lineNumbers {
		t.Fatal("lineNumbers = true, want false because explicit config replaces default config source")
	}
}

func TestParseProgramFlagsLoadsEnvProgramConfig(t *testing.T) {
	setTestProgramConfigHome(t)
	envPath := writeTestProgramConfig(t, `{"theme":"light","hidden":true,"line-numbers":true}`)
	t.Setenv("GOLESS_CONFIG", envPath)

	var out bytes.Buffer
	opts, _, err := parseProgramFlags([]string{"sample.txt"}, &out)
	if err != nil {
		t.Fatalf("parseProgramFlags(...) failed: %v", err)
	}
	if got, want := opts.configPath, envPath; got != want {
		t.Fatalf("configPath = %q, want %q", got, want)
	}
	if got, want := opts.presetName, "light"; got != want {
		t.Fatalf("theme = %q, want %q", got, want)
	}
	if !opts.hidden {
		t.Fatal("hidden = false, want true from GOLESS_CONFIG")
	}
	if !opts.lineNumbers {
		t.Fatal("lineNumbers = false, want true from GOLESS_CONFIG")
	}
}

func TestParseProgramFlagsEnvConfigOverridesDefaultPath(t *testing.T) {
	setTestProgramConfigHome(t)
	writeDefaultTestProgramConfig(t, `{"theme":"dark","hidden":false}`)
	envPath := writeTestProgramConfig(t, `{"theme":"light","hidden":true}`)
	t.Setenv("GOLESS_CONFIG", envPath)

	var out bytes.Buffer
	opts, _, err := parseProgramFlags([]string{"sample.txt"}, &out)
	if err != nil {
		t.Fatalf("parseProgramFlags(...) failed: %v", err)
	}
	if got, want := opts.presetName, "light"; got != want {
		t.Fatalf("theme = %q, want %q", got, want)
	}
	if !opts.hidden {
		t.Fatal("hidden = false, want true from GOLESS_CONFIG")
	}
}

func TestParseProgramFlagsExplicitConfigOverridesEnvConfig(t *testing.T) {
	setTestProgramConfigHome(t)
	envPath := writeTestProgramConfig(t, `{"theme":"dark","hidden":true}`)
	explicitPath := writeTestProgramConfigAtPath(t, filepath.Join(t.TempDir(), "explicit.json"), `{"theme":"light","hidden":false,"line-numbers":true}`)
	t.Setenv("GOLESS_CONFIG", envPath)

	var out bytes.Buffer
	opts, _, err := parseProgramFlags([]string{"-config", explicitPath}, &out)
	if err != nil {
		t.Fatalf("parseProgramFlags(...) failed: %v", err)
	}
	if got, want := opts.configPath, explicitPath; got != want {
		t.Fatalf("configPath = %q, want %q", got, want)
	}
	if got, want := opts.presetName, "light"; got != want {
		t.Fatalf("theme = %q, want %q", got, want)
	}
	if opts.hidden {
		t.Fatal("hidden = true, want false from explicit config")
	}
	if !opts.lineNumbers {
		t.Fatal("lineNumbers = false, want true from explicit config")
	}
}

func TestParseProgramFlagsRejectsMissingExplicitConfig(t *testing.T) {
	setTestProgramConfigHome(t)

	var out bytes.Buffer
	_, _, err := parseProgramFlags([]string{"-config", filepath.Join(t.TempDir(), "missing.json")}, &out)
	if err == nil {
		t.Fatal("parseProgramFlags(-config missing) = nil error, want error")
	}
	if !strings.Contains(err.Error(), "read config") {
		t.Fatalf("parseProgramFlags(-config missing) error = %q, want read config detail", err)
	}
}

func TestParseProgramFlagsHelpMentionsConfig(t *testing.T) {
	setTestProgramConfigHome(t)

	var out bytes.Buffer
	opts, args, err := parseProgramFlags([]string{"--help"}, &out)
	if err != nil {
		t.Fatalf("parseProgramFlags(--help) failed: %v", err)
	}
	if !opts.showHelp {
		t.Fatal("parseProgramFlags(--help) did not set showHelp")
	}
	if len(args) != 0 {
		t.Fatalf("len(args) after --help = %d, want 0", len(args))
	}
	if got := out.String(); !strings.Contains(got, "GOLESS_CONFIG") {
		t.Fatalf("help output = %q, want config path note", got)
	}
}

func setTestProgramConfigHome(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
}

func writeDefaultTestProgramConfig(t *testing.T, body string) string {
	t.Helper()
	path, err := defaultProgramConfigPath()
	if err != nil {
		t.Fatalf("defaultProgramConfigPath() failed: %v", err)
	}
	return writeProgramConfigFile(t, path, body)
}

func writeTestProgramConfig(t *testing.T, body string) string {
	t.Helper()
	return writeTestProgramConfigAtPath(t, filepath.Join(t.TempDir(), "config.json"), body)
}

func writeTestProgramConfigAtPath(t *testing.T, path, body string) string {
	t.Helper()
	return writeProgramConfigFile(t, path, body)
}

func writeProgramConfigFile(t *testing.T, path, body string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) failed: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) failed: %v", path, err)
	}
	return path
}
