package cli

import (
	"fmt"
	"strings"
)

// Table renders a styled ASCII table.
type Table struct {
	Headers []string
	Rows    [][]string
}

// NewTable creates a new table with headers.
func NewTable(headers ...string) *Table {
	return &Table{Headers: headers}
}

// AddRow adds a row to the table.
func (t *Table) AddRow(cols ...string) {
	t.Rows = append(t.Rows, cols)
}

// Render prints the table to stdout.
func (t *Table) Render() {
	if len(t.Headers) == 0 {
		return
	}

	// Calculate column widths
	widths := make([]int, len(t.Headers))
	for i, h := range t.Headers {
		widths[i] = len(h)
	}
	for _, row := range t.Rows {
		for i, col := range row {
			if i < len(widths) && len(col) > widths[i] {
				widths[i] = len(col)
			}
		}
	}

	// Add padding
	for i := range widths {
		widths[i] += 2
	}

	// Build separator
	sepParts := make([]string, len(widths))
	for i, w := range widths {
		sepParts[i] = strings.Repeat("─", w)
	}
	topSep := "  ┌" + strings.Join(sepParts, "┬") + "┐"
	midSep := "  ├" + strings.Join(sepParts, "┼") + "┤"
	botSep := "  └" + strings.Join(sepParts, "┴") + "┘"

	// Print header
	fmt.Println(topSep)
	fmt.Print("  │")
	for i, h := range t.Headers {
		fmt.Printf("%s%s%s%s│", colorBold, padCenter(h, widths[i]), colorReset, "")
	}
	fmt.Println()
	fmt.Println(midSep)

	// Print rows
	for _, row := range t.Rows {
		fmt.Print("  │")
		for i := range t.Headers {
			val := ""
			if i < len(row) {
				val = row[i]
			}
			// Apply color based on content
			colored := colorizeValue(val)
			// Pad based on raw value length
			padding := widths[i] - len(val)
			if padding < 0 {
				padding = 0
				val = val[:widths[i]]
				colored = colorizeValue(val)
			}
			leftPad := 1
			rightPad := padding - leftPad
			if rightPad < 0 {
				rightPad = 0
			}
			fmt.Printf("%s%s%s│", strings.Repeat(" ", leftPad), colored, strings.Repeat(" ", rightPad))
		}
		fmt.Println()
	}

	fmt.Println(botSep)
}

// padCenter centers a string within a given width.
func padCenter(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	total := width - len(s)
	left := total / 2
	right := total - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

// colorizeValue applies color to special values.
func colorizeValue(val string) string {
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "active", "running":
		return colorGreen + val + colorReset
	case "dormant", "pending", "sent":
		return colorYellow + val + colorReset
	case "dead", "error", "stopped":
		return colorRed + val + colorReset
	case "complete":
		return colorCyan + val + colorReset
	case "windows", "win10", "win11":
		return colorBlue + val + colorReset
	case "linux":
		return colorGreen + val + colorReset
	default:
		return val
	}
}
