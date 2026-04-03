// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package goless

import "github.com/gdamore/tcell/v3"

// Option configures a pager during construction.
//
// The interface is intentionally sealed so only this package can define new
// option implementations.
type Option interface {
	apply(*Config)
}

// RuntimeOption configures a running pager instance.
//
// RuntimeOption extends Option so the same option can be used both at
// construction time and later through Pager.Configure.
type RuntimeOption interface {
	Option
	applyRuntime(*Pager)
}

type optionFunc func(*Config)

func (f optionFunc) apply(cfg *Config) {
	if f != nil {
		f(cfg)
	}
}

type runtimeOptionFunc struct {
	optionFunc
	runtime func(*Pager)
}

func (f runtimeOptionFunc) applyRuntime(p *Pager) {
	if f.runtime != nil {
		f.runtime(p)
	}
}

// apply lets Config act as a construction-time compatibility option for New.
func (cfg Config) apply(dst *Config) {
	*dst = cfg
}

// WithTabWidth controls tab expansion during layout. Values <= 0 default to 8.
func WithTabWidth(width int) RuntimeOption {
	return runtimeOptionFunc{
		optionFunc: func(cfg *Config) {
			cfg.TabWidth = width
		},
		runtime: func(p *Pager) {
			p.viewer.SetTabWidth(width)
		},
	}
}

// WithWrapMode selects horizontal scrolling or soft wrapping.
func WithWrapMode(mode WrapMode) RuntimeOption {
	return runtimeOptionFunc{
		optionFunc: func(cfg *Config) {
			cfg.WrapMode = mode
		},
		runtime: func(p *Pager) {
			p.viewer.SetWrapMode(toInternalWrapMode(mode))
		},
	}
}

// WithSearchCaseMode selects smart-case, case-sensitive, or case-insensitive search behavior.
func WithSearchCaseMode(mode SearchCaseMode) RuntimeOption {
	return runtimeOptionFunc{
		optionFunc: func(cfg *Config) {
			cfg.SearchCase = mode
		},
		runtime: func(p *Pager) {
			p.viewer.SetSearchCaseMode(toInternalSearchCaseMode(mode))
		},
	}
}

// WithSearchMode selects substring, whole-word, or regex search behavior.
func WithSearchMode(mode SearchMode) RuntimeOption {
	return runtimeOptionFunc{
		optionFunc: func(cfg *Config) {
			cfg.SearchMode = mode
		},
		runtime: func(p *Pager) {
			p.viewer.SetSearchMode(toInternalSearchMode(mode))
		},
	}
}

// WithSqueezeBlankLines collapses repeated blank lines in the current view.
func WithSqueezeBlankLines(enabled bool) RuntimeOption {
	return runtimeOptionFunc{
		optionFunc: func(cfg *Config) {
			cfg.SqueezeBlankLines = enabled
		},
		runtime: func(p *Pager) {
			p.viewer.SetSqueezeBlankLines(enabled)
		},
	}
}

// WithLineNumbers enables an adaptive line-number gutter.
func WithLineNumbers(enabled bool) RuntimeOption {
	return runtimeOptionFunc{
		optionFunc: func(cfg *Config) {
			cfg.LineNumbers = enabled
		},
		runtime: func(p *Pager) {
			p.viewer.SetLineNumbers(enabled)
		},
	}
}

// WithHeaderLines pins the first N logical lines at the top of the viewport.
func WithHeaderLines(count int) RuntimeOption {
	return runtimeOptionFunc{
		optionFunc: func(cfg *Config) {
			cfg.HeaderLines = count
		},
		runtime: func(p *Pager) {
			p.viewer.SetHeaderLines(count)
		},
	}
}

// WithHeaderColumns pins the first N display columns at the left edge of the viewport.
func WithHeaderColumns(count int) RuntimeOption {
	return runtimeOptionFunc{
		optionFunc: func(cfg *Config) {
			cfg.HeaderColumns = count
		},
		runtime: func(p *Pager) {
			p.viewer.SetHeaderColumns(count)
		},
	}
}

// WithTheme remaps content default colors and ANSI 0-15 without affecting chrome.
func WithTheme(theme Theme) RuntimeOption {
	return runtimeOptionFunc{
		optionFunc: func(cfg *Config) {
			cfg.Theme = theme
		},
		runtime: func(p *Pager) {
			p.viewer.SetTheme(toInternalTheme(theme))
		},
	}
}

// WithVisualization controls pager-added markers for otherwise hidden structure.
func WithVisualization(visual Visualization) RuntimeOption {
	return runtimeOptionFunc{
		optionFunc: func(cfg *Config) {
			cfg.Visualization = visual
		},
		runtime: func(p *Pager) {
			p.viewer.SetVisualization(toInternalVisualization(visual))
		},
	}
}

// WithHyperlinkHandler controls how parsed OSC 8 hyperlinks are rendered.
func WithHyperlinkHandler(handler HyperlinkHandler) RuntimeOption {
	return runtimeOptionFunc{
		optionFunc: func(cfg *Config) {
			cfg.HyperlinkHandler = handler
		},
		runtime: func(p *Pager) {
			p.viewer.SetHyperlinkHandler(toInternalHyperlinkHandler(handler))
		},
	}
}

// WithCommandHandler handles unknown ':' commands after built-in pager commands decline them.
func WithCommandHandler(handler func(Command) CommandResult) RuntimeOption {
	return runtimeOptionFunc{
		optionFunc: func(cfg *Config) {
			cfg.CommandHandler = handler
		},
		runtime: func(p *Pager) {
			p.viewer.SetCommandHandler(toInternalCommandHandler(handler))
		},
	}
}

// WithKeyGroup selects a bundled set of key bindings for a new pager.
func WithKeyGroup(group KeyGroup) Option {
	return optionFunc(func(cfg *Config) {
		cfg.KeyGroup = group
	})
}

// WithUnboundKeys removes exact bundled bindings from a new pager.
func WithUnboundKeys(keys ...KeyStroke) Option {
	dup := append([]KeyStroke(nil), keys...)
	return optionFunc(func(cfg *Config) {
		cfg.UnbindKeys = dup
	})
}

// WithKeyBindings prepends custom bindings ahead of bundled defaults for a new pager.
func WithKeyBindings(bindings ...KeyBinding) Option {
	dup := append([]KeyBinding(nil), bindings...)
	return optionFunc(func(cfg *Config) {
		cfg.KeyBindings = dup
	})
}

// WithRenderMode controls how escapes and control sequences are presented.
func WithRenderMode(mode RenderMode) Option {
	return optionFunc(func(cfg *Config) {
		cfg.RenderMode = mode
	})
}

// WithChrome configures optional frame, title, and prompt/status styling.
func WithChrome(chrome Chrome) RuntimeOption {
	return runtimeOptionFunc{
		optionFunc: func(cfg *Config) {
			cfg.Chrome = chrome
		},
		runtime: func(p *Pager) {
			p.viewer.SetChrome(toInternalChrome(chrome))
		},
	}
}

// WithShowStatus enables or disables the status bar on the last screen row.
func WithShowStatus(enabled bool) RuntimeOption {
	return runtimeOptionFunc{
		optionFunc: func(cfg *Config) {
			cfg.ShowStatus = enabled
		},
		runtime: func(p *Pager) {
			p.viewer.SetShowStatus(enabled)
		},
	}
}

// WithCaptureKey reserves keys for the embedder before normal pager handling.
func WithCaptureKey(fn func(*tcell.EventKey) bool) Option {
	return optionFunc(func(cfg *Config) {
		cfg.CaptureKey = fn
	})
}

// WithText overrides help text, status text, prompt text, and UI strings.
func WithText(text Text) RuntimeOption {
	return runtimeOptionFunc{
		optionFunc: func(cfg *Config) {
			cfg.Text = text
		},
		runtime: func(p *Pager) {
			p.viewer.SetText(toInternalText(text))
		},
	}
}
