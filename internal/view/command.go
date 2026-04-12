// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package view

import "strings"

type Command struct {
	Raw       string
	Name      string
	Args      []string
	Confirmed bool
}

type CommandResult struct {
	Handled    bool
	Quit       bool
	Message    string
	KeepPrompt bool
	PromptText string
	Transient  bool
}

type CommandHandler func(Command) CommandResult

func parseCommand(text string) Command {
	fields := strings.Fields(text)
	cmd := Command{Raw: text}
	if len(fields) == 0 {
		return cmd
	}
	cmd.Name = fields[0]
	if len(fields) > 1 {
		cmd.Args = append([]string(nil), fields[1:]...)
	}
	return cmd
}
