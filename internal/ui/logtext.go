package ui

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

const (
	maxPanelRow = 5000
	maxPanelCol = 4096
)

type textCanvas struct {
	lines    [][]rune
	row      int
	col      int
	savedRow int
	savedCol int
}

// cleanPanelText applies terminal cursor edits before placing text in a panel.
func cleanPanelText(body string) string {
	canvas := &textCanvas{lines: [][]rune{{}}}
	parser := ansi.NewParser()
	parser.SetHandler(ansi.Handler{
		Print:     canvas.print,
		Execute:   canvas.execute,
		HandleCsi: canvas.csi,
		HandleEsc: canvas.escape,
	})
	for i := 0; i < len(body); i++ {
		parser.Advance(body[i])
	}
	return canvas.String()
}

func (c *textCanvas) line() []rune {
	for len(c.lines) <= c.row {
		c.lines = append(c.lines, nil)
	}
	return c.lines[c.row]
}

func (c *textCanvas) print(r rune) {
	if c.col >= maxPanelCol {
		return
	}
	if unicode.IsControl(r) {
		r = ' '
	}
	line := c.line()
	for len(line) < c.col {
		line = append(line, ' ')
	}
	if c.col == len(line) {
		line = append(line, r)
	} else {
		line[c.col] = r
	}
	c.lines[c.row] = line
	c.col++
}

func (c *textCanvas) execute(b byte) {
	switch b {
	case '\n':
		c.row = min(maxPanelRow, c.row+1)
		c.col = 0
	case '\r':
		c.col = 0
	case '\b', '\x7f':
		if c.col > 0 {
			c.col--
			line := c.line()
			if c.col < len(line) {
				c.lines[c.row] = append(line[:c.col], line[c.col+1:]...)
			}
		}
	case '\t':
		c.print(' ')
	}
}

func (c *textCanvas) csi(cmd ansi.Cmd, params ansi.Params) {
	if cmd.Prefix() != 0 || cmd.Intermediate() != 0 {
		return
	}
	number, _, _ := params.Param(0, 1)
	number = max(1, number)
	switch cmd.Final() {
	case 'A':
		c.row = max(0, c.row-number)
	case 'B':
		c.row = min(maxPanelRow, c.row+number)
	case 'C':
		c.col = min(maxPanelCol, c.col+number)
	case 'D':
		c.col = max(0, c.col-number)
	case 'E':
		c.row = min(maxPanelRow, c.row+number)
		c.col = 0
	case 'F':
		c.row = max(0, c.row-number)
		c.col = 0
	case 'G':
		c.col = min(maxPanelCol, number-1)
	case 'H', 'f':
		column, _, _ := params.Param(1, 1)
		c.row = min(maxPanelRow, number-1)
		c.col = min(maxPanelCol, max(0, column-1))
	case 'K':
		mode, _, _ := params.Param(0, 0)
		c.eraseLine(mode)
	case 'J':
		mode, _, _ := params.Param(0, 0)
		c.eraseDisplay(mode)
	case 's':
		c.saveCursor()
	case 'u':
		c.restoreCursor()
	}
}

func (c *textCanvas) escape(cmd ansi.Cmd) {
	switch cmd.Final() {
	case '7':
		c.saveCursor()
	case '8':
		c.restoreCursor()
	}
}

func (c *textCanvas) saveCursor() {
	c.savedRow, c.savedCol = c.row, c.col
}

func (c *textCanvas) restoreCursor() {
	c.row, c.col = c.savedRow, c.savedCol
}

func (c *textCanvas) eraseLine(mode int) {
	line := c.line()
	switch mode {
	case 0:
		c.lines[c.row] = line[:min(c.col, len(line))]
	case 1:
		for i := 0; i < min(c.col+1, len(line)); i++ {
			line[i] = ' '
		}
	case 2:
		c.lines[c.row] = nil
	}
}

func (c *textCanvas) eraseDisplay(mode int) {
	switch mode {
	case 0:
		c.eraseLine(0)
		for i := c.row + 1; i < len(c.lines); i++ {
			c.lines[i] = nil
		}
	case 1:
		for i := 0; i < c.row && i < len(c.lines); i++ {
			c.lines[i] = nil
		}
		c.eraseLine(1)
	case 2:
		for i := range c.lines {
			c.lines[i] = nil
		}
	}
}

func (c *textCanvas) String() string {
	lines := make([]string, len(c.lines))
	for i, line := range c.lines {
		lines[i] = strings.TrimRight(string(line), " ")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}
