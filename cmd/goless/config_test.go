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

func TestDefaultProgramConfigValues(t *testing.T) {
	got := defaultProgramConfigValues()
	want := programConfigDefaults{
		Theme:       programDefaultTheme,
		Hidden:      false,
		LineNumbers: false,
		LiveLinks:   false,
		Secure:      false,
	}
	if got != want {
		t.Fatalf("defaultProgramConfigValues() = %+v, want %+v", got, want)
	}
}

func TestWriteDefaultProgramConfig(t *testing.T) {
	var out bytes.Buffer
	if err := writeDefaultProgramConfig(&out); err != nil {
		t.Fatalf("writeDefaultProgramConfig(...) failed: %v", err)
	}
	const want = "{\n  \"theme\": \"pretty\",\n  \"hidden\": false,\n  \"line-numbers\": false,\n  \"live-links\": false,\n  \"secure\": false\n}\n"
	if got := out.String(); got != want {
		t.Fatalf("writeDefaultProgramConfig(...) = %q, want %q", got, want)
	}
}

func TestLoadProgramConfigMissing(t *testing.T) {
	cfg, err := loadProgramConfigAtPath(filepath.Join(t.TempDir(), "missing.json"), true)
	if err != nil {
		t.Fatalf("loadProgramConfig(missing) failed: %v", err)
	}
	if cfg != (programConfig{}) {
		t.Fatalf("loadProgramConfig(missing) = %+v, want zero config", cfg)
	}
}

func TestLoadProgramConfigEmptyPath(t *testing.T) {
	cfg, err := loadProgramConfigAtPath("", true)
	if err != nil {
		t.Fatalf("loadProgramConfigAtPath(empty) failed: %v", err)
	}
	if cfg != (programConfig{}) {
		t.Fatalf("loadProgramConfigAtPath(empty) = %+v, want zero config", cfg)
	}
}

func TestLoadProgramConfigBlankFile(t *testing.T) {
	path := writeTestProgramConfig(t, "\n\t  \n")

	cfg, err := loadProgramConfigAtPath(path, true)
	if err != nil {
		t.Fatalf("loadProgramConfigAtPath(blank file) failed: %v", err)
	}
	if cfg != (programConfig{}) {
		t.Fatalf("loadProgramConfigAtPath(blank file) = %+v, want zero config", cfg)
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

func TestLoadProgramConfigRejectsTrailingContent(t *testing.T) {
	path := writeTestProgramConfig(t, "{\"theme\":\"dark\"}\n{}\n")

	_, err := loadProgramConfigAtPath(path, true)
	if err == nil {
		t.Fatal("loadProgramConfig(...) = nil error, want trailing content error")
	}
	if !strings.Contains(err.Error(), "unexpected trailing content") {
		t.Fatalf("loadProgramConfig(...) error = %q, want trailing content detail", err)
	}
}

func TestParseProgramFlagsLoadsDefaultProgramConfig(t *testing.T) {
	setTestProgramConfigHome(t)
	writeDefaultTestProgramConfig(t, `{"theme":"dark","hidden":true,"line-numbers":true,"live-links":true,"secure":true}`)

	opts, args, err := parseProgramFlags([]string{"sample.txt"})
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

	opts, _, err := parseProgramFlags([]string{"-theme", "light", "-N=false", "-hidden=false", "-live-links=false", "-secure=false"})
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

	opts, _, err := parseProgramFlags([]string{"-config", explicitPath})
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

	opts, _, err := parseProgramFlags([]string{"sample.txt"})
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

	opts, _, err := parseProgramFlags([]string{"sample.txt"})
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

	opts, _, err := parseProgramFlags([]string{"-config", explicitPath})
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

	_, _, err := parseProgramFlags([]string{"-config", filepath.Join(t.TempDir(), "missing.json")})
	if err == nil {
		t.Fatal("parseProgramFlags(-config missing) = nil error, want error")
	}
	if !strings.Contains(err.Error(), "read config") {
		t.Fatalf("parseProgramFlags(-config missing) error = %q, want read config detail", err)
	}
}

func TestParseProgramFlagsHelpSkipsBrokenDefaultProgramConfig(t *testing.T) {
	setTestProgramConfigHome(t)
	writeDefaultTestProgramConfig(t, `{"theme":"dark"`)

	opts, args, err := parseProgramFlags([]string{"--help"})
	if err != nil {
		t.Fatalf("parseProgramFlags(--help) failed: %v", err)
	}
	if !opts.showHelp {
		t.Fatal("parseProgramFlags(--help) did not set showHelp")
	}
	if len(args) != 0 {
		t.Fatalf("len(args) after --help = %d, want 0", len(args))
	}
}

func TestParseProgramFlagsVersionSkipsBrokenEnvProgramConfig(t *testing.T) {
	setTestProgramConfigHome(t)
	path := writeTestProgramConfig(t, `{"theme":"dark"`)
	t.Setenv("GOLESS_CONFIG", path)

	opts, args, err := parseProgramFlags([]string{"--version"})
	if err != nil {
		t.Fatalf("parseProgramFlags(--version) failed: %v", err)
	}
	if !opts.version {
		t.Fatal("parseProgramFlags(--version) did not set version")
	}
	if len(args) != 0 {
		t.Fatalf("len(args) after --version = %d, want 0", len(args))
	}
}

func TestParseProgramFlagsHelpSkipsBrokenExplicitProgramConfig(t *testing.T) {
	setTestProgramConfigHome(t)
	path := writeTestProgramConfig(t, `{"theme":"dark"`)

	opts, args, err := parseProgramFlags([]string{"--help", "-config", path})
	if err != nil {
		t.Fatalf("parseProgramFlags(--help, -config) failed: %v", err)
	}
	if !opts.showHelp {
		t.Fatal("parseProgramFlags(--help, -config) did not set showHelp")
	}
	if len(args) != 0 {
		t.Fatalf("len(args) after --help = %d, want 0", len(args))
	}
}

func TestParseProgramFlagsVersionSkipsBrokenExplicitProgramConfig(t *testing.T) {
	setTestProgramConfigHome(t)
	path := writeTestProgramConfig(t, `{"theme":"dark"`)

	opts, args, err := parseProgramFlags([]string{"--version", "-config", path})
	if err != nil {
		t.Fatalf("parseProgramFlags(--version, -config) failed: %v", err)
	}
	if !opts.version {
		t.Fatal("parseProgramFlags(--version, -config) did not set version")
	}
	if len(args) != 0 {
		t.Fatalf("len(args) after --version = %d, want 0", len(args))
	}
}

func TestParseProgramFlagsDefaultConfigSkipsBrokenExplicitProgramConfig(t *testing.T) {
	setTestProgramConfigHome(t)
	path := writeTestProgramConfig(t, `{"theme":"dark"`)

	opts, args, err := parseProgramFlags([]string{"--default-config", "-config", path})
	if err == nil {
		t.Fatal("parseProgramFlags(--default-config, -config) = nil error, want exclusivity error")
	}
	if !strings.Contains(err.Error(), "--default-config must be used alone") {
		t.Fatalf("parseProgramFlags(--default-config, -config) error = %q, want exclusivity detail", err)
	}
	if opts != (programOptions{}) {
		t.Fatalf("parseProgramFlags(--default-config, -config) opts = %+v, want zero options on error", opts)
	}
	if args != nil {
		t.Fatalf("parseProgramFlags(--default-config, -config) args = %v, want nil", args)
	}
}

func TestProgramHasImmediateExitFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "none", args: []string{"sample.txt"}, want: false},
		{name: "help long", args: []string{"--help"}, want: true},
		{name: "help short", args: []string{"-h"}, want: true},
		{name: "help question", args: []string{"-?"}, want: true},
		{name: "default config long", args: []string{"--default-config"}, want: true},
		{name: "default config explicit true", args: []string{"--default-config=true"}, want: true},
		{name: "default config explicit false", args: []string{"--default-config=false"}, want: false},
		{name: "version long", args: []string{"--version"}, want: true},
		{name: "version short", args: []string{"-version"}, want: true},
		{name: "help explicit true", args: []string{"--help=true"}, want: true},
		{name: "version explicit true", args: []string{"--version=true"}, want: true},
		{name: "help explicit false", args: []string{"--help=false"}, want: false},
		{name: "stops at double dash", args: []string{"--", "--help"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := programHasImmediateExitFlag(tt.args); got != tt.want {
				t.Fatalf("programHasImmediateExitFlag(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestProgramConfigPathFromArgsRejectsEmptyValue(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "separate arg", args: []string{"-config", ""}},
		{name: "inline short", args: []string{"-config="}},
		{name: "inline long", args: []string{"--config="}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := programConfigPathFromArgs(tt.args)
			if err == nil {
				t.Fatal("programConfigPathFromArgs(...) = nil error, want error")
			}
			if !strings.Contains(err.Error(), "non-empty argument") {
				t.Fatalf("programConfigPathFromArgs(...) error = %q, want non-empty argument detail", err)
			}
		})
	}
}

func TestProgramConfigPathFromArgsUsesLastExplicitConfig(t *testing.T) {
	got, explicit, err := programConfigPathFromArgs([]string{"-config", "first.json", "--config=second.json"})
	if err != nil {
		t.Fatalf("programConfigPathFromArgs(...) failed: %v", err)
	}
	if !explicit {
		t.Fatal("programConfigPathFromArgs(...) explicit = false, want true")
	}
	if want := "second.json"; got != want {
		t.Fatalf("programConfigPathFromArgs(...) path = %q, want %q", got, want)
	}
}

func TestProgramConfigPathFromArgsStopsAtDoubleDash(t *testing.T) {
	got, explicit, err := programConfigPathFromArgs([]string{"--", "-config", "ignored.json"})
	if err != nil {
		t.Fatalf("programConfigPathFromArgs(...) failed: %v", err)
	}
	if explicit {
		t.Fatal("programConfigPathFromArgs(...) explicit = true, want false")
	}
	if got != "" {
		t.Fatalf("programConfigPathFromArgs(...) path = %q, want empty", got)
	}
}

func TestWriteProgramUsageMentionsConfig(t *testing.T) {
	setTestProgramConfigHome(t)

	var out bytes.Buffer
	writeProgramUsage(&out)
	if got := out.String(); !strings.Contains(got, "Config:") {
		t.Fatalf("help output = %q, want config section", got)
	}
	if got := out.String(); !strings.Contains(got, "GOLESS_CONFIG") {
		t.Fatalf("help output = %q, want config path note", got)
	}
	path, err := defaultProgramConfigPath()
	if err != nil {
		t.Fatalf("defaultProgramConfigPath() failed: %v", err)
	}
	if got := out.String(); !strings.Contains(got, path) {
		t.Fatalf("help output = %q, want resolved default config path %q", got, path)
	}
	if got := out.String(); !strings.Contains(got, "--default-config") {
		t.Fatalf("help output = %q, want default config flag", got)
	}
}

func setTestProgramConfigHome(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("APPDATA", filepath.Join(dir, "AppData", "Roaming"))
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
