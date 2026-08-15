// Package ui centralizes lipgloss styling, huh confirmations, and spinners so
// every command shares the same visual language as the original Python tool.
package ui

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// AutoConfirm short-circuits every confirmation prompt (GH_COMMIT_AUTO / --auto).
var AutoConfirm bool

var (
	red         = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	green       = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	greenBold   = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	yellow      = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	yellowBold  = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	magenta     = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	magentaBold = lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Bold(true)
	cyan        = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	dim         = lipgloss.NewStyle().Faint(true)
	bold        = lipgloss.NewStyle().Bold(true)

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("5")).
			Padding(0, 1)
)

// Println prints a blank line.
func Println() { fmt.Println() }

// Printf prints unstyled text with a trailing newline.
func Printf(format string, a ...any) { fmt.Printf(format+"\n", a...) }

// Errorf prints red text.
func Errorf(format string, a ...any) { fmt.Println(red.Render(fmt.Sprintf(format, a...))) }

// Successf prints green text.
func Successf(format string, a ...any) { fmt.Println(green.Render(fmt.Sprintf(format, a...))) }

// SuccessBoldf prints bold green text.
func SuccessBoldf(format string, a ...any) {
	fmt.Println(greenBold.Render(fmt.Sprintf(format, a...)))
}

// Warnf prints yellow text.
func Warnf(format string, a ...any) { fmt.Println(yellow.Render(fmt.Sprintf(format, a...))) }

// WarnBoldf prints bold yellow text.
func WarnBoldf(format string, a ...any) { fmt.Println(yellowBold.Render(fmt.Sprintf(format, a...))) }

// Infof prints cyan text.
func Infof(format string, a ...any) { fmt.Println(cyan.Render(fmt.Sprintf(format, a...))) }

// Magentaf prints magenta text.
func Magentaf(format string, a ...any) { fmt.Println(magenta.Render(fmt.Sprintf(format, a...))) }

// MagentaBoldf prints bold magenta text.
func MagentaBoldf(format string, a ...any) {
	fmt.Println(magentaBold.Render(fmt.Sprintf(format, a...)))
}

// Dimf prints faint text.
func Dimf(format string, a ...any) { fmt.Println(dim.Render(fmt.Sprintf(format, a...))) }

// Bold returns s rendered bold, for inline composition.
func Bold(s string) string { return bold.Render(s) }

// Panel prints text inside a magenta rounded-border panel.
func Panel(text string) { fmt.Println(panelStyle.Render(text)) }

// Cyan returns s rendered cyan, for inline composition.
func Cyan(s string) string { return cyan.Render(s) }

// Green returns s rendered green, for inline composition.
func Green(s string) string { return green.Render(s) }

// Yellow returns s rendered yellow, for inline composition.
func Yellow(s string) string { return yellow.Render(s) }

// Red returns s rendered red, for inline composition.
func Red(s string) string { return red.Render(s) }

// Confirm asks a yes/no question, defaulting to yes. AutoConfirm returns true
// immediately; a prompt failure (e.g. no TTY, Ctrl-C) counts as no.
func Confirm(msg string) bool {
	if AutoConfirm {
		return true
	}
	v := true
	if err := huh.NewConfirm().Title(msg).Value(&v).Run(); err != nil {
		return false
	}
	return v
}

// Spin runs fn behind a spinner titled title. Without a TTY (or in auto mode)
// it prints the title once and runs fn directly.
func Spin(title string, fn func()) {
	if AutoConfirm || !term.IsTerminal(int(os.Stdout.Fd())) || !term.IsTerminal(int(os.Stdin.Fd())) {
		Dimf("%s", title)
		fn()
		return
	}
	_ = spinner.New().Title(title).Action(fn).Run()
}
