package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func (m model) pickerView() string {
	base := m
	base.mode = ""
	lines := strings.Split(base.View(), "\n")
	if len(lines) < 6 {
		return base.View()
	}

	width := max(1, m.width-1)
	bottomBorder := len(lines) - 4
	dialogHeight := min(7, max(3, bottomBorder-1))
	dialogWidth := min(40, max(20, width-4))
	dialog := m.pickerDialog(dialogWidth, dialogHeight)
	startY := min(
		(len(lines)-len(dialog))/2,
		bottomBorder-len(dialog),
	)
	startY = max(0, startY)
	startX := max(0, (width-dialogWidth)/2)

	for i, overlayLine := range dialog {
		background := ansi.Strip(lines[startY+i])
		left := ansi.Cut(background, 0, startX)
		right := ansi.Cut(
			background, startX+dialogWidth, width,
		)
		lines[startY+i] = dimStyle.Render(fitWidth(left, startX)) +
			overlayLine +
			dimStyle.Render(fitWidth(
				right, width-startX-dialogWidth,
			))
	}
	lines[len(lines)-2] = " " + accentStyle.Render(fitWidth(
		"  tab/shift+tab move   enter open   esc close", width-2,
	))
	return strings.Join(lines, "\n")
}

func (m model) pickerDialog(width, height int) []string {
	dialog := []string{dimStyle.Render(frameTop("RESOURCES", width))}
	optionCount := min(len(kinds), height-2)
	first := max(0, m.pickerCursor-optionCount+1)
	first = min(first, len(kinds)-optionCount)

	for i := first; i < first+optionCount; i++ {
		label := "  " + strings.ToUpper(kinds[i])
		style := textStyle
		if i == m.pickerCursor {
			label = "› " + strings.ToUpper(kinds[i])
			style = selectStyle
		}
		content := style.Render(fitWidth(label, width-2))
		dialog = append(dialog, frameRow(content, width))
	}
	if height > optionCount+2 {
		help := fitWidth("  enter open · esc close", width-2)
		dialog = append(dialog, frameRow(dimStyle.Render(help), width))
	}
	dialog = append(dialog, frameBottom(width))
	return dialog
}
