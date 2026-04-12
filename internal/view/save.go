// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package view

import "strings"

const (
	saveScopeContentLabel  = "\U0001F4C4"
	saveScopeViewportLabel = "\U0001F50D"
	saveFormatANSIHint     = "\U0001F3A8"
	saveFormatPlainHint    = "\u26AB"
)

type saveConfirmState struct {
	path   string
	export ExportOptions
	prompt string
}

func defaultSaveExportOptions() ExportOptions {
	return ExportOptions{
		Scope:  ExportScopeContent,
		Format: ExportFormatANSI,
	}
}

// BeginSavePrompt opens the built-in save prompt when a command handler is configured.
func (v *Viewer) BeginSavePrompt() bool {
	if v.cfg.CommandHandler == nil {
		return false
	}
	v.beginPrompt(promptSave)
	return true
}

func saveScopeHintLabel(scope ExportScope) string {
	switch scope {
	case ExportScopeViewport:
		return saveScopeViewportLabel
	default:
		return saveScopeContentLabel
	}
}

func saveFormatHintLabel(format ExportFormat) string {
	switch format {
	case ExportFormatPlain:
		return saveFormatPlainHint
	default:
		return saveFormatANSIHint
	}
}

func (v *Viewer) saveModeHintText() string {
	if v.prompt == nil || v.prompt.saveConfirm != nil {
		return ""
	}
	return "F2:" + saveScopeHintLabel(v.prompt.export.Scope) + " F3:" + saveFormatHintLabel(v.prompt.export.Format)
}

func (v *Viewer) cycleSaveScope() ExportScope {
	if v.prompt == nil {
		return ExportScopeContent
	}
	if v.prompt.saveConfirm != nil {
		return v.prompt.export.Scope
	}
	if v.prompt.export.Scope == ExportScopeViewport {
		v.prompt.export.Scope = ExportScopeContent
		return v.prompt.export.Scope
	}
	v.prompt.export.Scope = ExportScopeViewport
	return v.prompt.export.Scope
}

func (v *Viewer) cycleSaveFormat() ExportFormat {
	if v.prompt == nil {
		return ExportFormatANSI
	}
	if v.prompt.saveConfirm != nil {
		return v.prompt.export.Format
	}
	if v.prompt.export.Format == ExportFormatPlain {
		v.prompt.export.Format = ExportFormatANSI
		return v.prompt.export.Format
	}
	v.prompt.export.Format = ExportFormatPlain
	return v.prompt.export.Format
}

func buildSaveCommand(path string, opts ExportOptions) Command {
	path = strings.TrimSpace(path)
	args := make([]string, 0, 3)
	raw := "save"
	switch normalizeExportOptions(opts).Scope {
	case ExportScopeViewport:
		args = append(args, "--view")
		raw += " --view"
	default:
		args = append(args, "--file")
		raw += " --file"
	}
	switch normalizeExportOptions(opts).Format {
	case ExportFormatPlain:
		args = append(args, "--mono")
		raw += " --mono"
	default:
		args = append(args, "--ansi")
		raw += " --ansi"
	}
	if path != "" {
		args = append(args, path)
		raw += " " + path
	}
	return Command{
		Raw:  raw,
		Name: "save",
		Args: args,
	}
}

func (v *Viewer) runSavePrompt(path string) (commit bool, quit bool) {
	if v.prompt != nil {
		v.prompt.errText = ""
	}
	if v.prompt != nil && v.prompt.saveConfirm != nil {
		answer := strings.TrimSpace(path)
		confirm := v.prompt.saveConfirm
		v.prompt.saveConfirm = nil
		if !strings.EqualFold(answer, "yes") {
			v.setTransientMessage("save canceled")
			return true, false
		}
		cmd := buildSaveCommand(confirm.path, confirm.export)
		cmd.Confirmed = true
		result := v.cfg.CommandHandler(cmd)
		if result.PromptText != "" {
			v.prompt.saveConfirm = &saveConfirmState{
				path:   confirm.path,
				export: confirm.export,
				prompt: result.PromptText,
			}
			v.prompt.prefix = result.PromptText
			v.prompt.editor.Clear()
		}
		if result.Message != "" {
			if result.KeepPrompt && v.prompt != nil {
				v.prompt.errText = result.Message
				v.setMessage("")
			} else if result.Transient {
				v.setTransientMessage(result.Message)
			} else {
				v.setMessage(result.Message)
			}
		}
		if result.Handled {
			return !result.KeepPrompt, result.Quit
		}
		v.setTransientMessage(v.text.CommandUnknown("save"))
		return true, false
	}
	if v.cfg.CommandHandler == nil {
		v.setTransientMessage(v.text.CommandUnknown("save"))
		return true, false
	}
	cmd := buildSaveCommand(path, v.prompt.export)
	result := v.cfg.CommandHandler(cmd)
	if result.PromptText != "" && v.prompt != nil {
		v.prompt.saveConfirm = &saveConfirmState{
			path:   strings.TrimSpace(path),
			export: v.prompt.export,
			prompt: result.PromptText,
		}
		v.prompt.prefix = result.PromptText
		v.prompt.editor.Clear()
	}
	if result.Message != "" {
		if result.KeepPrompt && v.prompt != nil {
			v.prompt.errText = result.Message
			v.setMessage("")
		} else if result.Transient {
			v.setTransientMessage(result.Message)
		} else {
			v.setMessage(result.Message)
		}
	}
	if result.Handled {
		return !result.KeepPrompt, result.Quit
	}
	v.setTransientMessage(v.text.CommandUnknown("save"))
	return true, false
}
