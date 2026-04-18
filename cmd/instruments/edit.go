package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jebbisson/spice-synth/instrument"
)

type pane int

const (
	paneGroups pane = iota
	paneInstruments
	paneVariants
)

type previewFinishedMsg struct {
	seq uint64
	err error
}

type tuiStyles struct {
	app        lipgloss.Style
	header     lipgloss.Style
	panel      lipgloss.Style
	focusPanel lipgloss.Style
	title      lipgloss.Style
	selected   lipgloss.Style
	muted      lipgloss.Style
	detail     lipgloss.Style
	footer     lipgloss.Style
	input      lipgloss.Style
	statusOK   lipgloss.Style
	statusWarn lipgloss.Style
}

type editModel struct {
	file               *instrument.File
	path               string
	groups             []string
	groupNames         []string
	selectedGroup      int
	instrumentNames    []string
	selectedInstrument int
	variantNames       []string
	selectedVariant    int
	focus              pane
	status             string
	statusIsError      bool
	previewSeq         uint64
	previewNote        string
	previewNoteManual  bool
	inputMode          string
	inputLabel         string
	inputValue         string
	width              int
	height             int
	styles             tuiStyles
}

func runEdit(path string) error {
	f, err := instrument.LoadFile(path)
	if err != nil {
		return err
	}
	m := editModel{
		file:   f,
		path:   path,
		styles: newTUIStyles(),
		status: "Navigate with arrows or hjkl. Enter previews. n changes note. e/E edit levels, f feedback, c connection, x export, s save.",
	}
	m.buildGroups()
	m.refreshInstruments()
	m.refreshVariants()
	m.syncPreviewNoteToSelection()
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func newTUIStyles() tuiStyles {
	border := lipgloss.RoundedBorder()
	return tuiStyles{
		app:        lipgloss.NewStyle().Padding(1, 2),
		header:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("62")).Padding(0, 1),
		panel:      lipgloss.NewStyle().Border(border).BorderForeground(lipgloss.Color("238")).Padding(0, 1),
		focusPanel: lipgloss.NewStyle().Border(border).BorderForeground(lipgloss.Color("69")).Padding(0, 1),
		title:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("75")),
		selected:   lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("63")).Bold(true),
		muted:      lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		detail:     lipgloss.NewStyle().Border(border).BorderForeground(lipgloss.Color("240")).Padding(0, 1),
		footer:     lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Background(lipgloss.Color("236")).Padding(0, 1),
		input:      lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("238")).Padding(0, 1),
		statusOK:   lipgloss.NewStyle().Foreground(lipgloss.Color("114")),
		statusWarn: lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true),
	}
}

func (m editModel) Init() tea.Cmd { return nil }

func (m editModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case previewFinishedMsg:
		if msg.seq != m.previewSeq {
			return m, nil
		}
		if msg.err != nil {
			m.setStatus(msg.err.Error(), true)
		} else {
			m.setStatus("Preview finished", false)
		}
		return m, nil
	case tea.KeyMsg:
		if m.inputMode != "" {
			return m.handleInput(msg)
		}
		switch msg.String() {
		case "q", "ctrl+c":
			stopPreview()
			return m, tea.Quit
		case "left", "h":
			if m.focus > paneGroups {
				m.focus--
			}
		case "right", "l":
			if m.focus < paneVariants {
				m.focus++
			}
		case "up", "k":
			m.move(-1)
		case "down", "j":
			m.move(1)
		case "enter", "p":
			cmd := m.previewSelected()
			return m, cmd
		case "n":
			m.beginInput("note", "Preview note (e.g. C3, F#4, A2)")
		case "e":
			m.beginInput("op2.l", "Carrier level (0-63)")
		case "E":
			m.beginInput("op1.l", "Modulator level (0-63)")
		case "f":
			m.beginInput("feedback", "Feedback (0-7)")
		case "c":
			m.beginInput("connection", "Connection (0=FM, 1=Additive)")
		case "t":
			m.toggleFlag("op2.t")
		case "v":
			m.toggleFlag("op2.v")
		case "r":
			m.toggleFlag("op2.kr")
		case "u":
			m.toggleFlag("op2.su")
		case "x":
			m.beginInput("export", "Export path (.go, or all.go for all variants)")
		case "s":
			if err := instrument.SaveFile(m.path, m.file); err != nil {
				m.setStatus(err.Error(), true)
			} else {
				m.setStatus("Saved YAML file", false)
			}
		}
	}
	return m, nil
}

func (m editModel) View() string {
	if m.width == 0 {
		return "Loading TUI..."
	}
	header := m.styles.header.Width(max(0, m.width-4)).Render("SpiceSynth Instruments")
	toolbar := m.renderToolbar()
	body := m.renderBody()
	footer := m.renderFooter()
	parts := []string{header, toolbar, body, footer}
	if m.inputMode != "" {
		parts = append(parts, m.styles.input.Width(max(0, m.width-4)).Render(m.inputLabel+": "+m.inputValue))
	}
	return m.styles.app.Render(strings.Join(parts, "\n"))
}

func (m *editModel) buildGroups() {
	groups := sortedGroups(m.file)
	m.groupNames = append([]string{"all"}, groups...)
	if len(m.groupNames) == 0 {
		m.groupNames = []string{"all"}
	}
	if m.selectedGroup >= len(m.groupNames) {
		m.selectedGroup = 0
	}
}

func (m *editModel) refreshInstruments() {
	selectedGroupName := m.groupNames[m.selectedGroup]
	names := make([]string, 0)
	for _, inst := range m.file.Instruments {
		if selectedGroupName != "all" && inst.Group != selectedGroupName {
			continue
		}
		names = append(names, inst.Name)
	}
	m.instrumentNames = names
	if len(m.instrumentNames) == 0 {
		m.selectedInstrument = 0
	} else if m.selectedInstrument >= len(m.instrumentNames) {
		m.selectedInstrument = len(m.instrumentNames) - 1
	}
}

func (m *editModel) refreshVariants() {
	m.variantNames = nil
	if len(m.instrumentNames) == 0 {
		m.selectedVariant = 0
		return
	}
	instName := m.instrumentNames[m.selectedInstrument]
	for _, inst := range m.file.Instruments {
		if inst.Name != instName {
			continue
		}
		for _, variant := range inst.Variants {
			m.variantNames = append(m.variantNames, variant.Name)
		}
		break
	}
	if len(m.variantNames) == 0 {
		m.selectedVariant = 0
	} else if m.selectedVariant >= len(m.variantNames) {
		m.selectedVariant = len(m.variantNames) - 1
	}
}

func (m *editModel) move(delta int) {
	switch m.focus {
	case paneGroups:
		m.selectedGroup = clamp(m.selectedGroup+delta, 0, len(m.groupNames)-1)
		m.refreshInstruments()
		m.refreshVariants()
		m.syncPreviewNoteToSelection()
	case paneInstruments:
		if len(m.instrumentNames) == 0 {
			return
		}
		m.selectedInstrument = clamp(m.selectedInstrument+delta, 0, len(m.instrumentNames)-1)
		m.refreshVariants()
		m.syncPreviewNoteToSelection()
	case paneVariants:
		if len(m.variantNames) == 0 {
			return
		}
		m.selectedVariant = clamp(m.selectedVariant+delta, 0, len(m.variantNames)-1)
		m.syncPreviewNoteToSelection()
	}
}

func (m *editModel) beginInput(mode, label string) {
	if mode != "export" && m.currentVariant() == nil {
		m.setStatus("No variant selected", true)
		return
	}
	m.inputMode = mode
	m.inputLabel = label
	m.inputValue = ""
	if mode == "export" {
		m.setStatus("Enter export path and press Enter", false)
	} else {
		m.setStatus(label, false)
	}
}

func (m editModel) handleInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.inputMode, m.inputLabel, m.inputValue = "", "", ""
		m.setStatus("Cancelled", false)
	case "enter":
		m.applyInput()
	case "backspace":
		if len(m.inputValue) > 0 {
			m.inputValue = m.inputValue[:len(m.inputValue)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.inputValue += msg.String()
		}
	}
	return m, nil
}

func (m *editModel) applyInput() {
	defer func() {
		m.inputMode, m.inputLabel, m.inputValue = "", "", ""
	}()
	if m.inputMode == "export" {
		m.applyExport(strings.TrimSpace(m.inputValue))
		return
	}
	if m.inputMode == "note" {
		note := strings.TrimSpace(m.inputValue)
		if note == "" {
			m.setStatus("Preview note cannot be empty", true)
			return
		}
		m.previewNote = note
		m.previewNoteManual = true
		m.setStatus("Preview note set to "+note, false)
		return
	}
	variant := m.currentVariant()
	if variant == nil {
		m.setStatus("No variant selected", true)
		return
	}
	value, err := strconv.Atoi(strings.TrimSpace(m.inputValue))
	if err != nil {
		m.setStatus(fmt.Sprintf("Invalid value: %v", err), true)
		return
	}
	switch m.inputMode {
	case "op2.l":
		if value < 0 || value > 63 {
			m.setStatus("Carrier level must be 0-63", true)
			return
		}
		variant.Op2.Level = uint8(value)
	case "op1.l":
		if value < 0 || value > 63 {
			m.setStatus("Modulator level must be 0-63", true)
			return
		}
		variant.Op1.Level = uint8(value)
	case "feedback":
		if value < 0 || value > 7 {
			m.setStatus("Feedback must be 0-7", true)
			return
		}
		variant.Feedback = uint8(value)
	case "connection":
		if value < 0 || value > 1 {
			m.setStatus("Connection must be 0 or 1", true)
			return
		}
		variant.Connection = uint8(value)
	}
	if err := variant.Validate(); err != nil {
		m.setStatus(err.Error(), true)
		return
	}
	m.setStatus("Updated field", false)
}

func (m *editModel) toggleFlag(flag string) {
	variant := m.currentVariant()
	if variant == nil {
		m.setStatus("No variant selected", true)
		return
	}
	switch flag {
	case "op2.t":
		variant.Op2.Tremolo = !variant.Op2.Tremolo
	case "op2.v":
		variant.Op2.Vibrato = !variant.Op2.Vibrato
	case "op2.kr":
		variant.Op2.KeyScaleRate = !variant.Op2.KeyScaleRate
	case "op2.su":
		variant.Op2.Sustaining = !variant.Op2.Sustaining
	}
	if err := variant.Validate(); err != nil {
		m.setStatus(err.Error(), true)
		return
	}
	m.setStatus("Toggled field", false)
}

func (m *editModel) previewSelected() tea.Cmd {
	instEntry, variant := m.currentSelection()
	if instEntry == nil || variant == nil {
		m.setStatus("No variant selected", true)
		return nil
	}
	inst := variant.ToInstrumentWithParent(instEntry.Name)
	m.previewSeq++
	seq := m.previewSeq
	note := m.previewNote
	if note == "" {
		note = defaultNoteForInstrument(instEntry)
	}
	stopPreview()
	m.setStatus("Previewing "+note+" for 3 seconds...", false)
	return func() tea.Msg {
		return previewFinishedMsg{seq: seq, err: previewInstrument(inst, note, 3*time.Second)}
	}
}

func (m *editModel) applyExport(path string) {
	if path == "" {
		m.setStatus("Export path cannot be empty", true)
		return
	}
	var b strings.Builder
	b.WriteString("package generated\n\n")
	b.WriteString("import \"github.com/jebbisson/spice-synth/voice\"\n\n")
	if path == "all.go" {
		for _, inst := range m.file.Instruments {
			for _, variant := range inst.Variants {
				resolved := variant.ToInstrumentWithParent(inst.Name)
				b.WriteString(instrumentCode(fullKey(&inst, &variant), resolved))
				b.WriteString("\n")
			}
		}
	} else {
		instEntry, variant := m.currentSelection()
		if instEntry == nil || variant == nil {
			m.setStatus("No variant selected", true)
			return
		}
		resolved := variant.ToInstrumentWithParent(instEntry.Name)
		b.WriteString(instrumentCode(fullKey(instEntry, variant), resolved))
	}
	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		m.setStatus(err.Error(), true)
		return
	}
	m.setStatus("Exported Go code to "+path, false)
}

func (m *editModel) currentSelection() (*instrument.FileInstrument, *instrument.InstrumentDef) {
	if len(m.instrumentNames) == 0 || len(m.variantNames) == 0 {
		return nil, nil
	}
	instName := m.instrumentNames[m.selectedInstrument]
	variantName := m.variantNames[m.selectedVariant]
	for i := range m.file.Instruments {
		if m.file.Instruments[i].Name != instName {
			continue
		}
		for j := range m.file.Instruments[i].Variants {
			if m.file.Instruments[i].Variants[j].Name == variantName {
				return &m.file.Instruments[i], &m.file.Instruments[i].Variants[j]
			}
		}
	}
	return nil, nil
}

func (m *editModel) currentVariant() *instrument.InstrumentDef {
	_, variant := m.currentSelection()
	return variant
}

func (m *editModel) syncPreviewNoteToSelection() {
	if m.previewNoteManual {
		return
	}
	instEntry, _ := m.currentSelection()
	m.previewNote = defaultNoteForInstrument(instEntry)
}

func (m *editModel) setStatus(text string, isError bool) {
	m.status = text
	m.statusIsError = isError
}

func (m editModel) renderToolbar() string {
	groupLabel := fmt.Sprintf("Group: %s", m.groupNames[m.selectedGroup])
	countLabel := fmt.Sprintf("Instruments: %d  Variants: %d", len(m.instrumentNames), len(m.variantNames))
	left := m.styles.title.Render(groupLabel)
	right := m.styles.muted.Render(countLabel)
	space := max(1, m.width-4-lipgloss.Width(left)-lipgloss.Width(right))
	return left + strings.Repeat(" ", space) + right
}

func (m editModel) renderBody() string {
	totalWidth := max(40, m.width-4)
	leftW := max(18, totalWidth/5)
	midW := max(24, totalWidth/4)
	rightW := max(28, totalWidth/4)
	detailW := totalWidth - leftW - midW - rightW - 6
	if detailW < 32 {
		detailW = 32
	}
	colHeight := max(8, m.height-12)

	groups := m.renderListPanel("Groups", m.groupNames, m.selectedGroup, paneGroups, leftW, colHeight)
	insts := m.renderListPanel("Instruments", m.instrumentNames, m.selectedInstrument, paneInstruments, midW, colHeight)
	variants := m.renderListPanel("Variants", m.variantNames, m.selectedVariant, paneVariants, rightW, colHeight)
	detail := m.renderDetailPanel(detailW, colHeight)

	return lipgloss.JoinHorizontal(lipgloss.Top, groups, "  ", insts, "  ", variants, "  ", detail)
}

func (m editModel) renderListPanel(title string, items []string, selected int, p pane, width, height int) string {
	style := m.styles.panel
	if m.focus == p {
		style = m.styles.focusPanel
	}
	contentHeight := max(1, height-2)
	start := scrollStart(selected, len(items), contentHeight)
	visible := make([]string, 0, contentHeight)
	for i := start; i < min(len(items), start+contentHeight); i++ {
		line := items[i]
		if line == "" {
			line = "(none)"
		}
		line = lipgloss.NewStyle().MaxWidth(width - 4).Render(line)
		if i == selected {
			line = m.styles.selected.Render(" " + line + " ")
		}
		visible = append(visible, line)
	}
	if len(visible) == 0 {
		visible = append(visible, m.styles.muted.Render("(empty)"))
	}
	body := strings.Join(visible, "\n")
	return style.Width(width).Height(height).Render(m.styles.title.Render(title) + "\n" + body)
}

func (m editModel) renderDetailPanel(width, height int) string {
	instEntry, variant := m.currentSelection()
	style := m.styles.detail.Width(width).Height(height)
	if instEntry == nil || variant == nil {
		return style.Render(m.styles.title.Render("Details") + "\n" + m.styles.muted.Render("No variant selected"))
	}
	lines := []string{
		m.styles.title.Render(fullKey(instEntry, variant)),
		"",
		fmt.Sprintf("Group       %s", instEntry.Group),
		fmt.Sprintf("Default     %s", defaultNoteForInstrument(instEntry)),
		fmt.Sprintf("Preview     %s", m.previewNote),
		fmt.Sprintf("Feedback    %d", variant.Feedback),
		fmt.Sprintf("Connection  %d", variant.Connection),
		"",
		"Op1",
		fmt.Sprintf("  a %2d  d %2d  s %2d  r %2d", variant.Op1.Attack, variant.Op1.Decay, variant.Op1.Sustain, variant.Op1.Release),
		fmt.Sprintf("  l %2d  m %2d  kl %2d  w %2d", variant.Op1.Level, variant.Op1.Multiply, variant.Op1.KeyScaleLevel, variant.Op1.Waveform),
		fmt.Sprintf("  kr %-5t  t %-5t  v %-5t  su %-5t", variant.Op1.KeyScaleRate, variant.Op1.Tremolo, variant.Op1.Vibrato, variant.Op1.Sustaining),
		"",
		"Op2",
		fmt.Sprintf("  a %2d  d %2d  s %2d  r %2d", variant.Op2.Attack, variant.Op2.Decay, variant.Op2.Sustain, variant.Op2.Release),
		fmt.Sprintf("  l %2d  m %2d  kl %2d  w %2d", variant.Op2.Level, variant.Op2.Multiply, variant.Op2.KeyScaleLevel, variant.Op2.Waveform),
		fmt.Sprintf("  kr %-5t  t %-5t  v %-5t  su %-5t", variant.Op2.KeyScaleRate, variant.Op2.Tremolo, variant.Op2.Vibrato, variant.Op2.Sustaining),
		"",
		m.styles.muted.Render("Enter/P preview  n note  x export  s save"),
		m.styles.muted.Render("e/E levels  f feedback  c connection"),
		m.styles.muted.Render("t/v/r/u toggle op2 flags"),
	}
	return style.Render(strings.Join(lines, "\n"))
}

func (m editModel) renderFooter() string {
	status := m.status
	if status == "" {
		status = "Ready"
	}
	statusStyle := m.styles.statusOK
	if m.statusIsError {
		statusStyle = m.styles.statusWarn
	}
	left := statusStyle.Render(status)
	right := m.styles.muted.Render("Arrows/hjkl move  Enter preview  n note  q quit")
	space := max(1, m.width-4-lipgloss.Width(left)-lipgloss.Width(right))
	return m.styles.footer.Width(max(0, m.width-4)).Render(left + strings.Repeat(" ", space) + right)
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func scrollStart(selected, total, visible int) int {
	if total <= visible {
		return 0
	}
	half := visible / 2
	start := selected - half
	if start < 0 {
		start = 0
	}
	if start > total-visible {
		start = total - visible
	}
	return start
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
