// Package ui provides terminal UI helpers using lipgloss for styled output.
package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	green  = lipgloss.Color("#25D366") // WhatsApp green — used as accent
	red    = lipgloss.Color("#FF4444")
	yellow = lipgloss.Color("#FFAA00")
	cyan   = lipgloss.Color("#00BBFF")
	gray   = lipgloss.Color("#888888")
	white  = lipgloss.Color("#FFFFFF")
	dim    = lipgloss.Color("#666666")

	// Styles
	successMark = lipgloss.NewStyle().Foreground(green).Bold(true).Render("✔")
	errorMark   = lipgloss.NewStyle().Foreground(red).Bold(true).Render("✗")
	warnMark    = lipgloss.NewStyle().Foreground(yellow).Bold(true).Render("⚠")
	infoMark    = lipgloss.NewStyle().Foreground(cyan).Bold(true).Render("ℹ")

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(white)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(gray)

	keyStyle = lipgloss.NewStyle().
			Foreground(cyan).
			Bold(true)

	valueStyle = lipgloss.NewStyle().
			Foreground(white)

	dimStyle = lipgloss.NewStyle().
			Foreground(dim)

	brandStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(green)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(green).
			Padding(1, 2)

	dangerBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(red).
			Padding(1, 2)
)

// Success prints a success message with a green checkmark.
func Success(msg string) {
	fmt.Fprintf(os.Stderr, "%s %s\n", successMark, msg)
}

// Successf prints a formatted success message.
func Successf(format string, args ...interface{}) {
	Success(fmt.Sprintf(format, args...))
}

// Error prints an error message with a red X.
func Error(msg string) {
	fmt.Fprintf(os.Stderr, "%s %s\n", errorMark, msg)
}

// Errorf prints a formatted error message.
func Errorf(format string, args ...interface{}) {
	Error(fmt.Sprintf(format, args...))
}

// Warning prints a warning message.
func Warning(msg string) {
	fmt.Fprintf(os.Stderr, "%s %s\n", warnMark, msg)
}

// Warningf prints a formatted warning message.
func Warningf(format string, args ...interface{}) {
	Warning(fmt.Sprintf(format, args...))
}

// Info prints an info message.
func Info(msg string) {
	fmt.Fprintf(os.Stderr, "%s %s\n", infoMark, msg)
}

// Infof prints a formatted info message.
func Infof(format string, args ...interface{}) {
	Info(fmt.Sprintf(format, args...))
}

// Title prints a bold title.
func Title(msg string) {
	fmt.Fprintln(os.Stderr, titleStyle.Render(msg))
}

// Subtitle prints a gray subtitle.
func Subtitle(msg string) {
	fmt.Fprintln(os.Stderr, subtitleStyle.Render(msg))
}

// Dim prints dimmed text.
func Dim(msg string) {
	fmt.Fprintln(os.Stderr, dimStyle.Render(msg))
}

// Dimf prints formatted dimmed text.
func Dimf(format string, args ...interface{}) {
	Dim(fmt.Sprintf(format, args...))
}

// Brand prints text in the Wotp brand color (green accent).
func Brand(msg string) string {
	return brandStyle.Render(msg)
}

// KeyValue prints a labeled key-value pair.
func KeyValue(key, value string) {
	fmt.Fprintf(os.Stderr, "%s %s  %s\n",
		successMark,
		keyStyle.Render(key),
		valueStyle.Render(value),
	)
}

// Box prints content in a rounded bordered box.
func Box(content string) {
	fmt.Fprintln(os.Stderr, boxStyle.Render(content))
}

// DangerBox prints content in a red bordered box.
func DangerBox(content string) {
	fmt.Fprintln(os.Stderr, dangerBoxStyle.Render(content))
}

// Blank prints an empty line.
func Blank() {
	fmt.Fprintln(os.Stderr)
}

// Spinner displays a simple text-based spinner with the given message.
// It returns a stop function that should be called when the operation is complete.
func Spinner(msg string) func() {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	done := make(chan struct{})

	go func() {
		i := 0
		for {
			select {
			case <-done:
				// Clear the spinner line
				fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", len(msg)+4))
				return
			default:
				fmt.Fprintf(os.Stderr, "\r%s %s",
					lipgloss.NewStyle().Foreground(cyan).Render(frames[i%len(frames)]),
					msg,
				)
				time.Sleep(80 * time.Millisecond)
				i++
			}
		}
	}()

	return func() {
		close(done)
		time.Sleep(100 * time.Millisecond) // let the goroutine clean up
	}
}

// PrintKeys displays the API keys in formatted output matching the spec §2 experience.
func PrintKeys(anonKey, serviceKey string) {
	fmt.Fprintf(os.Stderr, "%s Anon key:    %s\n", successMark, keyStyle.Render(anonKey))
	fmt.Fprintf(os.Stderr, "%s Service key: %s\n", successMark, keyStyle.Render(serviceKey))
}

// PrintStartBanner displays the startup complete banner matching spec §2.
func PrintStartBanner(port int, anonKey, serviceKey string) {
	Blank()
	Successf("API ready at http://localhost:%d", port)
	Successf("Dashboard at http://localhost:%d/dashboard", port)
	PrintKeys(anonKey, serviceKey)
	Blank()
	fmt.Fprintln(os.Stderr, Brand("Wotp is running.")+" Run "+keyStyle.Render("wotp status")+" anytime to check health.")
}

// PrintQRInstructions prints the QR code scanning instructions. The QR
// itself is rendered by the dashboard (project-scoped, see
// core/internal/api/projects.go), not by the CLI.
func PrintQRInstructions(port int) {
	Blank()
	Title("Scan this QR code with WhatsApp (Settings → Linked Devices):")
	Blank()
	Infof("Open http://localhost:%d/dashboard to see the QR code.", port)
}

// ConfirmPrompt asks the user for a yes/no confirmation. Returns true if user confirms.
func ConfirmPrompt(msg string) bool {
	fmt.Fprintf(os.Stderr, "%s %s [y/N]: ", warnMark, msg)
	var response string
	_, _ = fmt.Scanln(&response)
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

// DoubleConfirmPrompt asks for confirmation twice (for destructive operations).
func DoubleConfirmPrompt(msg, confirmWord string) bool {
	if !ConfirmPrompt(msg) {
		return false
	}
	fmt.Fprintf(os.Stderr, "%s Type %s to confirm: ",
		lipgloss.NewStyle().Foreground(red).Bold(true).Render("⚠"),
		keyStyle.Render(confirmWord),
	)
	var response string
	_, _ = fmt.Scanln(&response)
	return strings.TrimSpace(response) == confirmWord
}

// PrintStatus displays a formatted status output. Per-number connection
// state is in the dashboard's Numbers screen — this only reports whether
// the instance's API process itself is up.
func PrintStatus(status string, uptimeSeconds int64) {
	Blank()
	Title("Wotp Status")
	Blank()

	statusColor := green
	statusIcon := "●"
	if status != "ok" {
		statusColor = red
		statusIcon = "○"
	}
	statusStyle := lipgloss.NewStyle().Foreground(statusColor).Bold(true)

	fmt.Fprintf(os.Stderr, "  Status:  %s %s\n", statusStyle.Render(statusIcon), statusStyle.Render(status))
	if uptimeSeconds > 0 {
		duration := time.Duration(uptimeSeconds) * time.Second
		fmt.Fprintf(os.Stderr, "  Uptime:  %s\n", valueStyle.Render(formatDuration(duration)))
	}
	Blank()
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
