package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
)

// Helper functions
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

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// truncateFromStart truncates a string from the beginning, keeping the end visible
func truncateFromStart(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return "..." + s[len(s)-(maxLen-3):]
}

// getRelativePath returns path relative to basePath
func getRelativePath(fullPath, basePath string) string {
	rel, err := filepath.Rel(basePath, fullPath)
	if err != nil {
		return fullPath
	}
	return rel
}

type Item struct {
	Path       string
	Size       int64
	IsSelected bool
}

type Items []Item

func (i Items) Len() int           { return len(i) }
func (i Items) Less(j, k int) bool { return i[j].Size > i[k].Size }
func (i Items) Swap(j, k int)      { i[j], i[k] = i[k], i[j] }

type model struct {
	files      Items
	folders    Items
	cursor     int
	viewMode   string // "files" or "folders"
	confirming bool
	err        error
	windowSize tea.WindowSizeMsg
	styles     styles
	offset     int    // for scrolling
	height     int    // visible height
	width      int    // screen width
	basePath   string // initial path to trim from display
}

type styles struct {
	title         lipgloss.Style
	header        lipgloss.Style
	selected      lipgloss.Style
	normal        lipgloss.Style
	size          lipgloss.Style
	helpText      lipgloss.Style
	errorText     lipgloss.Style
	confirmText   lipgloss.Style
	selectionMark lipgloss.Style
}

func initStyles() styles {
	return styles{
		title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFF")).
			Background(lipgloss.Color("#0366d6")).
			Padding(0, 1),
		header: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFF")).
			Background(lipgloss.Color("#2f363d")),
		selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFF")).
			Background(lipgloss.Color("#2ea043")),
		normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFF")),
		size: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#58a6ff")),
		helpText: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8b949e")),
		errorText: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f85149")),
		confirmText: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFF")).
			Background(lipgloss.Color("#da3633")).
			Padding(0, 1),
		selectionMark: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff0000")),
	}
}

func getDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

func scanDirectory(path string) (Items, Items, error) {
	var files, folders Items

	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if strings.HasPrefix(filepath.Base(path), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			size, err := getDirSize(path)
			if err != nil {
				return nil
			}
			folders = append(folders, Item{Path: path, Size: size})
		} else {
			files = append(files, Item{Path: path, Size: info.Size()})
		}
		return nil
	})

	sort.Sort(files)
	sort.Sort(folders)

	return files, folders, err
}

func initialModel(path string) (model, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return model{}, err
	}

	files, folders, err := scanDirectory(absPath)
	if err != nil {
		return model{}, err
	}

	return model{
		files:    files,
		folders:  folders,
		viewMode: "files",
		styles:   initStyles(),
		height:   10,  // Default height, will be updated on WindowSizeMsg
		width:    100, // Default width, will be updated on WindowSizeMsg
		basePath: absPath,
	}, nil
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
			}
		case "down", "j":
			items := m.files
			if m.viewMode == "folders" {
				items = m.folders
			}
			if m.cursor < len(items)-1 {
				m.cursor++
				if m.cursor >= m.offset+m.height-4 {
					m.offset = m.cursor - m.height + 5
				}
			}
		case "pageup":
			m.offset -= m.height - 4
			if m.offset < 0 {
				m.offset = 0
			}
			m.cursor -= m.height - 4
			if m.cursor < 0 {
				m.cursor = 0
			}
		case "pagedown":
			items := m.files
			if m.viewMode == "folders" {
				items = m.folders
			}
			m.offset += m.height - 4
			maxOffset := len(items) - (m.height - 4)
			if m.offset > maxOffset {
				m.offset = maxOffset
			}
			if m.offset < 0 {
				m.offset = 0
			}
			m.cursor += m.height - 4
			if m.cursor >= len(items) {
				m.cursor = len(items) - 1
			}
		case "home":
			m.cursor = 0
			m.offset = 0
		case "end":
			items := m.files
			if m.viewMode == "folders" {
				items = m.folders
			}
			m.cursor = len(items) - 1
			m.offset = len(items) - (m.height - 4)
			if m.offset < 0 {
				m.offset = 0
			}
		case "tab":
			m.viewMode = map[string]string{
				"files":   "folders",
				"folders": "files",
			}[m.viewMode]
			m.cursor = 0
			m.offset = 0
		case " ":
			if m.viewMode == "files" && m.cursor < len(m.files) {
				m.files[m.cursor].IsSelected = !m.files[m.cursor].IsSelected
			} else if m.viewMode == "folders" && m.cursor < len(m.folders) {
				m.folders[m.cursor].IsSelected = !m.folders[m.cursor].IsSelected
			}
		case "d":
			m.confirming = true
		case "y":
			if m.confirming {
				items := &m.files
				if m.viewMode == "folders" {
					items = &m.folders
				}
				for i, item := range *items {
					if item.IsSelected {
						err := os.Remove(item.Path)
						if err != nil {
							m.err = err
							break
						}
						(*items)[i].IsSelected = false
					}
				}
				m.confirming = false
			}
		case "n":
			if m.confirming {
				m.confirming = false
			}
		}
	case tea.WindowSizeMsg:
		m.windowSize = msg
		m.height = msg.Height
		m.width = msg.Width
	}
	return m, nil
}

func (m model) View() string {
	if m.err != nil {
		return m.styles.errorText.Render(fmt.Sprintf("Error: %v", m.err))
	}

	var s strings.Builder

	// Get current items list
	items := m.files
	if m.viewMode == "folders" {
		items = m.folders
	}

	// Title with item count
	title := fmt.Sprintf(" Disk Usage Analyzer - %s (%d/%d) ",
		strings.ToUpper(m.viewMode),
		min(m.cursor+1, max(len(items), 1)),
		len(items),
	)
	s.WriteString(m.styles.title.Render(title) + "\n\n")

	// Calculate widths based on screen size
	selectWidth := 3                                                  // Width for selection indicator (including brackets) [*]
	sizeWidth := 8                                                    // Fixed width for size column
	minPathWidth := 30                                                // Minimum width for path
	nameWidth := m.width - sizeWidth - selectWidth - minPathWidth - 6 // -6 for spacing

	// If we still have too much space, limit name column to something reasonable
	if nameWidth > 100 {
		nameWidth = 100
	}

	// Path gets whatever is left
	pathWidth := m.width - sizeWidth - nameWidth - selectWidth - 6

	// Header
	header := fmt.Sprintf("[%s] %*s %-*s %s",
		" ",
		sizeWidth, "SIZE",
		nameWidth, "NAME",
		"PATH",
	)
	s.WriteString(m.styles.header.Render(header) + "\n")

	// Handle empty list
	if len(items) == 0 {
		s.WriteString(m.styles.normal.Render("\nNo items found in this view"))
		s.WriteString(m.styles.helpText.Render("\n\nTab: Switch View • q: Quit"))
		return s.String()
	}

	// Calculate visible range and items
	visibleHeight := m.height - 4
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	// Update offset bounds
	if m.offset < 0 {
		m.offset = 0
	}
	maxOffset := len(items) - visibleHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.offset > maxOffset {
		m.offset = maxOffset
	}

	endIdx := m.offset + visibleHeight
	if endIdx > len(items) {
		endIdx = len(items)
	}

	visibleItems := items[m.offset:endIdx]

	// Items
	for i, item := range visibleItems {
		name := filepath.Base(item.Path)
		relPath := getRelativePath(filepath.Dir(item.Path), m.basePath)
		selected := " "
		if item.IsSelected {
			selected = m.styles.selectionMark.Render("*")
		}

		// Format line with selection at start
		line := fmt.Sprintf("[%s] %*s %-*s %s",
			selected,
			sizeWidth, m.styles.size.Render(humanize.Bytes(uint64(item.Size))),
			nameWidth, truncateString(name, nameWidth),
			truncateFromStart(relPath, pathWidth),
		)

		if i+m.offset == m.cursor {
			s.WriteString(m.styles.selected.Render(line))
		} else {
			s.WriteString(m.styles.normal.Render(line))
		}
		s.WriteString("\n")
	}

	// Confirmation dialog
	if m.confirming {
		s.WriteString("\n" + m.styles.confirmText.Render("Delete selected items? (y/n)"))
	}

	// Help
	help := "\n↑/↓: Navigate • PgUp/PgDn: Page • Home/End: Jump • Tab: Switch View • Space: Select • d: Delete • q: Quit"
	s.WriteString(m.styles.helpText.Render(help))

	return s.String()
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <directory_path>")
		os.Exit(1)
	}

	initialModel, err := initialModel(os.Args[1])
	if err != nil {
		fmt.Printf("Error initializing: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(
		initialModel,
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
