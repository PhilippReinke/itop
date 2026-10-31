package ui

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"github.com/philippreinke/itop/internal/containercli"
	"strings"
)

var (
	brandStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#C4B5FD")).Bold(true)
	accentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD")).Bold(true)
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#8892A6"))
	textStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#E6E9F0"))
	goodStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#84E1BC"))
	warnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#F9C97C"))
	badStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF8298"))
	selectStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#101522")).Background(lipgloss.Color("#8BE9FD")).Bold(true)
)

func clipWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	var out strings.Builder
	used := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if used+rw > width-1 {
			break
		}
		out.WriteRune(r)
		used += rw
	}
	return out.String() + "…"
}
func fitWidth(s string, width int) string {
	clipped := clipWidth(s, width)
	return clipped + strings.Repeat(" ", max(0, width-lipgloss.Width(clipped)))
}
func linePair(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return fitWidth(left, width)
	}
	return left + strings.Repeat(" ", gap) + right
}
func frameTop(title string, width int) string {
	label := "╭─ " + title + " "
	room := width - lipgloss.Width(label) - 1
	if room < 0 {
		return "╭" + strings.Repeat("─", max(0, width-2)) + "╮"
	}
	return label + strings.Repeat("─", room) + "╮"
}
func frameRow(content string, width int) string {
	return dimStyle.Render("│") + content + dimStyle.Render("│")
}
func frameBottom(width int) string {
	return dimStyle.Render("╰" + strings.Repeat("─", max(0, width-2)) + "╯")
}
func stateStyle(state string) lipgloss.Style {
	s := strings.ToLower(state)
	if strings.Contains(s, "running") || strings.Contains(s, "active") {
		return goodStyle
	}
	if strings.Contains(s, "error") || strings.Contains(s, "failed") || strings.Contains(s, "dead") {
		return badStyle
	}
	return warnStyle
}
func (m model) actionHints() string {
	if m.service == containercli.ServiceStopped {
		return "↵ / s start service   r retry   : switch resource   ? help"
	}
	switch m.kind() {
	case "containers":
		return "↵ inspect   l logs   v stats   e shell   s start   x stop   d delete"
	case "images":
		return "↵ inspect   n pull   d delete"
	default:
		return "↵ inspect   n create   d delete"
	}
}
func (m model) View() string {
	w := max(1, m.width-1)
	if m.height <= 9 || w < 20 {
		return clipWidth("itop · resize terminal", w)
	}
	inner := w - 2
	page := m.pageSize()
	var b strings.Builder
	brand := brandStyle.Render("▌ ITOP") + dimStyle.Render("  Apple container")
	service := warnStyle.Render("● CHECKING")
	switch m.service {
	case containercli.ServiceRunning:
		service = goodStyle.Render("● ONLINE")
	case containercli.ServiceStopped:
		service = warnStyle.Render("● STOPPED")
	case containercli.ServiceUnknown:
		service = badStyle.Render("● ERROR")
	}
	b.WriteString(linePair(" "+brand, service+"  ", w) + "\n")
	stamp := "waiting for service"
	if !m.lastRefresh.IsZero() {
		stamp = "updated " + m.lastRefresh.Format("15:04:05")
	}
	b.WriteString(linePair(dimStyle.Render("  LOCAL ENGINE"), dimStyle.Render(stamp+"  ↻ 4s  "), w) + "\n")
	b.WriteString(" " + dimStyle.Render(fitWidth(m.actionHints(), w-2)) + "\n")
	rows := m.visible()
	name := strings.ToUpper(m.kind())
	position := "0/0"
	if len(rows) > 0 {
		position = fmt.Sprintf("%d/%d", m.cursor+1, len(rows))
	}
	title := fmt.Sprintf("%s  %s", name, position)
	if m.filter != "" && m.mode != "panel" {
		title += "  /" + m.filter
	}
	if m.mode == "panel" {
		title = strings.ToUpper(m.panelTitle)
	}
	b.WriteString(dimStyle.Render(frameTop(clipWidth(title, max(1, w-6)), w)) + "\n")
	if m.mode == "panel" {
		b.WriteString(frameRow(" "+dimStyle.Render(fitWidth("J/K SCROLL  •  ESC BACK", inner-2))+" ", w) + "\n")
		lines := strings.Split(m.panelBody, "\n")
		maxScroll := max(0, len(lines)-page)
		scroll := min(m.panelScroll, maxScroll)
		for i := 0; i < page; i++ {
			content := ""
			if scroll+i < len(lines) {
				content = lines[scroll+i]
			}
			b.WriteString(frameRow(" "+textStyle.Render(fitWidth(content, inner-2))+" ", w) + "\n")
		}
	} else {
		nameW := min(36, max(12, inner*39/100))
		stateW := min(18, max(10, inner*21/100))
		detailW := max(1, inner-11-nameW-stateW)
		header := fmt.Sprintf("  %3s  %s  %s  %s", "#", fitWidth("NAME", nameW), fitWidth("STATUS / SIZE", stateW), fitWidth("IMAGE / DETAIL", detailW))
		b.WriteString(frameRow(dimStyle.Render(fitWidth(header, inner)), w) + "\n")
		for slot := 0; slot < page; slot++ {
			i := m.offset + slot
			if i >= len(rows) {
				content := ""
				if len(rows) == 0 && slot == 1 {
					content = "    No " + m.kind() + " found. Press n to create one."
					if m.service == containercli.ServiceChecking {
						content = "    Checking Apple container service…"
					}
					if m.service == containercli.ServiceStopped {
						content = "    Apple container is stopped"
					}
					if m.service == containercli.ServiceUnknown {
						content = "    Could not check service. Press r to retry."
					}
				}
				if len(rows) == 0 && m.service == containercli.ServiceStopped && slot == 2 {
					content = "    [ Enter / s ] Start service"
				}
				style := dimStyle
				if len(rows) == 0 && m.service == containercli.ServiceStopped && slot == 2 {
					style = accentStyle
				}
				b.WriteString(frameRow(style.Render(fitWidth(content, inner)), w) + "\n")
				continue
			}
			r := rows[i]
			prefix := fmt.Sprintf("  %3d  ", i+1)
			nameCol := fitWidth(r.Name, nameW)
			stateCol := fitWidth(r.State, stateW)
			detailCol := fitWidth(r.Detail, detailW)
			plain := fitWidth(prefix+nameCol+"  "+stateCol+"  "+detailCol, inner)
			if i == m.cursor {
				b.WriteString(frameRow(selectStyle.Render(plain), w) + "\n")
			} else {
				body := prefix + textStyle.Render(nameCol) + "  " + stateStyle(r.State).Render(stateCol) + "  " + dimStyle.Render(detailCol)
				b.WriteString(frameRow(fitWidth(body, inner), w) + "\n")
			}
		}
	}
	b.WriteString(frameBottom(w) + "\n")
	status := m.status
	if status == "" {
		status = "Ready"
	}
	statusStyle := goodStyle
	if m.statusError {
		statusStyle = badStyle
	}
	b.WriteString(" " + statusStyle.Render("● "+fitWidth(status, w-4)) + "\n")
	prompt := "  : resource   / filter   ? help"
	if m.service == containercli.ServiceStopped {
		prompt = "  [ Enter / s ] Start container service    [ r ] Retry"
	}
	switch m.mode {
	case "confirm":
		prompt = "  Confirm " + m.pendingLabel + "?  y / n"
	case "filter":
		prompt = "  /" + m.input + "█"
	case "command":
		prompt = "  :" + m.input + "█"
	case "run":
		prompt = "  Run image › " + m.input + "█"
	case "pull":
		prompt = "  Pull image › " + m.input + "█"
	case "create":
		prompt = "  Create " + strings.TrimSuffix(m.kind(), "s") + " › " + m.input + "█"
	case "panel":
		prompt = "  Esc back   J/K scroll   PgUp/PgDn page"
	}
	b.WriteString(" " + accentStyle.Render(fitWidth(prompt, w-2)) + "\n")
	b.WriteString(" " + dimStyle.Render(fitWidth("esc back   j/k move   r refresh   q quit", w-2)))
	return b.String()
}

const helpText = `Navigation
  :              Switch resource view (:containers, :images, :networks, :volumes)
  j/k or arrows  Move selection
  g / G          First / last item
  /              Filter current view
  r              Refresh
  S              Start container service (also Enter / s when stopped)
  esc            Back to previous resource view, clear filter, or close panel

Containers
  enter / i      Inspect JSON
  l              Last 300 log lines
  v              One-shot stats
  s / x          Start / stop
  e              Interactive /bin/sh
  n              Run image detached
  d              Delete selected container

Images, networks, volumes
  enter / i      Inspect JSON
  n              Pull image or create network/volume
  d              Delete selected resource

Deletion and stop ask for confirmation. Press q to quit.`
