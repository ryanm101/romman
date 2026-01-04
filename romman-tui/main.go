package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ryanm101/romman-lib/db"
	"github.com/ryanm101/romman-lib/library"
)

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

// Model holds the application state
type model struct {
	systems   []systemInfo
	libraries []libraryInfo
	cursor    int
	panel     panel
	width     int
	height    int
	err       error

	// Detail view
	inDetail      bool
	detailFilter  detailFilter
	detailItems   []detailItem
	detailCounts  map[detailFilter]int
	detailCursor  int
	selectedLib   string
	loadingDetail bool

	// Search
	searching   bool
	searchQuery string

	// Status
	scanning  bool
	statusMsg string

	// Help overlay
	showHelp bool

	// Rename operation
	renaming    bool
	renameItems []renameAction
}

type panel int

const (
	panelSystems panel = iota
	panelLibraries
)

type detailFilter int

const (
	filterMatched detailFilter = iota
	filterMissing
	filterFlagged
	filterUnmatched
	filterPreferred
	filterDuplicates
)

type systemInfo struct {
	Name         string
	DatName      string
	ReleaseCount int
}

type libraryInfo struct {
	Name       string
	System     string
	Path       string
	SystemID   int64
	GamesInLib int
	TotalGames int
	LastScanAt string
}

type detailItem struct {
	Name      string
	Path      string
	MatchType string
	Flags     string
	DupGroup  int // For duplicate grouping
}

type renameAction struct {
	OldPath string
	NewPath string
	Status  string // pending, done, error
}

func initialModel() model {
	return model{
		panel: panelSystems,
	}
}

// Init loads initial data
func (m model) Init() tea.Cmd {
	return tea.Batch(
		loadSystems,
		loadLibraries,
	)
}

// Update handles messages
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		// Help overlay - always available
		if msg.String() == "?" {
			m.showHelp = !m.showHelp
			return m, nil
		}
		if m.showHelp {
			// Any key closes help
			m.showHelp = false
			return m, nil
		}

		// Search mode handling
		if m.searching {
			switch msg.String() {
			case "enter", "esc":
				m.searching = false
				return m, nil
			case "backspace":
				if len(m.searchQuery) > 0 {
					m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
				}
			default:
				if len(msg.String()) == 1 {
					m.searchQuery += msg.String()
				}
			}
			// Reset cursor on search change
			m.detailCursor = 0
			return m, nil
		}

		// Detail view keys
		if m.inDetail {
			switch msg.String() {
			case "q", "esc", "backspace":
				m.inDetail = false
				m.detailItems = nil
				return m, nil
			case "up", "k":
				if m.detailCursor > 0 {
					m.detailCursor--
				}
			case "down", "j":
				if m.detailCursor < len(m.getFilteredItems())-1 {
					m.detailCursor++
				}
			case "pgup":
				m.detailCursor -= 10
				if m.detailCursor < 0 {
					m.detailCursor = 0
				}
			case "pgdown":
				m.detailCursor += 10
				filtered := m.getFilteredItems()
				if m.detailCursor >= len(filtered) {
					m.detailCursor = len(filtered) - 1
				}
				if m.detailCursor < 0 {
					m.detailCursor = 0
				}
			case "/":
				m.searching = true
				m.searchQuery = ""
				return m, nil
			case "1", "m": // Matched
				m.detailFilter = filterMatched
				m.detailCursor = 0
				m.loadingDetail = true
				return m, loadDetail(m.selectedLib, m.detailFilter)
			case "2", "i": // mIssing
				m.detailFilter = filterMissing
				m.detailCursor = 0
				m.loadingDetail = true
				return m, loadDetail(m.selectedLib, m.detailFilter)
			case "3", "f": // Flagged
				m.detailFilter = filterFlagged
				m.detailCursor = 0
				m.loadingDetail = true
				return m, loadDetail(m.selectedLib, m.detailFilter)
			case "4", "u": // Unmatched
				m.detailFilter = filterUnmatched
				m.detailCursor = 0
				m.loadingDetail = true
				return m, loadDetail(m.selectedLib, m.detailFilter)
			case "5", "p": // Preferred
				m.detailFilter = filterPreferred
				m.detailCursor = 0
				m.loadingDetail = true
				return m, loadDetail(m.selectedLib, m.detailFilter)
			case "6", "d": // Duplicates
				m.detailFilter = filterDuplicates
				m.detailCursor = 0
				m.loadingDetail = true
				return m, loadDetail(m.selectedLib, m.detailFilter)
			case "R": // Rename files (shift+R)
				m.renaming = true
				m.statusMsg = fmt.Sprintf("Renaming files in %s...", m.selectedLib)
				return m, renameLibraryFiles(m.selectedLib)
			}
			return m, nil
		}

		// Main view keys
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			if m.panel == panelSystems {
				m.panel = panelLibraries
			} else {
				m.panel = panelSystems
			}
			m.cursor = 0
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			maxItems := m.maxItems()
			if m.cursor < maxItems-1 {
				m.cursor++
			}
		case "pgup":
			m.cursor -= 10
			if m.cursor < 0 {
				m.cursor = 0
			}
		case "pgdown":
			m.cursor += 10
			maxItems := m.maxItems()
			if m.cursor >= maxItems {
				m.cursor = maxItems - 1
			}
		case "enter":
			if m.panel == panelLibraries && m.cursor < len(m.libraries) {
				m.inDetail = true
				m.selectedLib = m.libraries[m.cursor].Name
				m.detailFilter = filterMatched
				m.detailCursor = 0
				m.loadingDetail = true
				return m, loadDetail(m.selectedLib, m.detailFilter)
			}
		case "s":
			if m.panel == panelLibraries && m.cursor < len(m.libraries) {
				m.scanning = true
				m.statusMsg = fmt.Sprintf("Scanning %s...", m.libraries[m.cursor].Name)
				return m, scanLibrary(m.libraries[m.cursor].Name)
			} else if m.panel == panelSystems && m.cursor < len(m.systems) {
				// Scan all libraries for this system
				systemName := m.systems[m.cursor].Name
				var libsToScan []string
				for _, lib := range m.libraries {
					if lib.System == systemName {
						libsToScan = append(libsToScan, lib.Name)
					}
				}
				if len(libsToScan) == 0 {
					m.statusMsg = fmt.Sprintf("No libraries for %s", systemName)
					return m, nil
				}
				m.scanning = true
				m.statusMsg = fmt.Sprintf("Scanning %d libraries for %s...", len(libsToScan), systemName)
				return m, scanLibraries(libsToScan)
			}
		case "r":
			m.statusMsg = "Refreshing..."
			return m, tea.Batch(loadSystems, loadLibraries)
		case "R": // Rename files (shift+R)
			if m.panel == panelLibraries && m.cursor < len(m.libraries) {
				m.renaming = true
				m.statusMsg = fmt.Sprintf("Renaming files in %s...", m.libraries[m.cursor].Name)
				return m, renameLibraryFiles(m.libraries[m.cursor].Name)
			}
		}

	case systemsMsg:
		m.systems = msg.systems
		m.err = msg.err

	case librariesMsg:
		m.libraries = msg.libraries
		m.err = msg.err
		if m.statusMsg == "Refreshing..." {
			m.statusMsg = ""
		}

	case scanCompleteMsg:
		m.scanning = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Scan failed: %v", msg.err)
		} else {
			m.statusMsg = "Scan complete!"
		}
		return m, loadLibraries

	case detailMsg:
		m.detailItems = msg.items
		m.detailCounts = msg.counts
		m.loadingDetail = false

	case renameMsg:
		m.renaming = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Rename failed: %v", msg.err)
		} else {
			m.statusMsg = fmt.Sprintf("Renamed %d files", msg.renamed)
		}
		// Refresh the detail view
		if m.inDetail {
			return m, loadDetail(m.selectedLib, m.detailFilter)
		}
	}

	return m, nil
}

func (m model) getFilteredItems() []detailItem {
	if m.searchQuery == "" {
		return m.detailItems
	}
	var filtered []detailItem
	for _, item := range m.detailItems {
		if strings.Contains(strings.ToLower(item.Name), strings.ToLower(m.searchQuery)) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (m model) maxItems() int {
	if m.panel == panelSystems {
		return len(m.systems)
	}
	return len(m.libraries)
}

// View renders the UI
func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	if m.showHelp {
		return m.viewHelp()
	}

	if m.inDetail {
		return m.viewDetail()
	}

	return m.viewMain()
}

func (m model) viewMain() string {
	// Styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		MarginBottom(1)

	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Width(m.width/2 - 4)

	activePanelStyle := panelStyle.
		BorderForeground(lipgloss.Color("205"))

	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("57")).
		Foreground(lipgloss.Color("255"))

	// Calculate max visible items based on terminal height
	// Panel chrome: border (2) + padding (2) + title (2) = 6 lines
	// Footer: help (2) + status (1) + title (2) = 5 lines
	maxVisibleItems := m.height - 15
	if maxVisibleItems < 5 {
		maxVisibleItems = 5
	}

	// Systems panel
	systemsStyle := panelStyle
	if m.panel == panelSystems {
		systemsStyle = activePanelStyle
	}

	var systemsContent string
	systemsContent += titleStyle.Render("📀 Systems") + "\n\n"
	if m.err != nil {
		systemsContent += fmt.Sprintf("Error: %v", m.err)
	} else if len(m.systems) == 0 {
		systemsContent += "No systems imported.\nUse CLI: romman dat import <file>"
	} else {
		// Calculate scroll window for systems
		sysStart := 0
		sysCursor := 0
		if m.panel == panelSystems {
			sysCursor = m.cursor
		}
		if sysCursor >= maxVisibleItems {
			sysStart = sysCursor - maxVisibleItems + 1
		}
		sysEnd := sysStart + maxVisibleItems
		if sysEnd > len(m.systems) {
			sysEnd = len(m.systems)
		}

		for i := sysStart; i < sysEnd; i++ {
			s := m.systems[i]
			line := fmt.Sprintf("%-8s %d games", s.Name, s.ReleaseCount)
			if m.panel == panelSystems && i == m.cursor {
				line = selectedStyle.Render(line)
			}
			systemsContent += line + "\n"
		}
		if len(m.systems) > maxVisibleItems {
			systemsContent += fmt.Sprintf("  (%d/%d)\n", sysCursor+1, len(m.systems))
		}
	}
	systemsPanel := systemsStyle.Render(systemsContent)

	// Libraries panel
	libsStyle := panelStyle
	if m.panel == panelLibraries {
		libsStyle = activePanelStyle
	}

	var libsContent string
	libsContent += titleStyle.Render("📁 Libraries") + "\n\n"
	if m.err != nil && len(m.systems) > 0 && len(m.libraries) == 0 {
		// Show error if systems loaded but libraries didn't (indicates library-specific error)
		libsContent += fmt.Sprintf("Error loading: %v", m.err)
	} else if len(m.libraries) == 0 {
		libsContent += "No libraries configured.\nUse CLI: romman library add"
	} else {
		// Calculate scroll window for libraries
		libStart := 0
		libCursor := 0
		if m.panel == panelLibraries {
			libCursor = m.cursor
		}
		if libCursor >= maxVisibleItems {
			libStart = libCursor - maxVisibleItems + 1
		}
		libEnd := libStart + maxVisibleItems
		if libEnd > len(m.libraries) {
			libEnd = len(m.libraries)
		}

		for i := libStart; i < libEnd; i++ {
			lib := m.libraries[i]
			pct := 0
			if lib.TotalGames > 0 {
				pct = lib.GamesInLib * 100 / lib.TotalGames
			}

			// Progress bar with color
			bar := renderProgressBar(pct, 15)
			line := fmt.Sprintf("%-10s %s %3d%%", lib.Name, bar, pct)

			if m.panel == panelLibraries && i == m.cursor {
				line = selectedStyle.Render(line)
			}
			libsContent += line + "\n"
		}
		if len(m.libraries) > maxVisibleItems {
			libsContent += fmt.Sprintf("  (%d/%d)\n", libCursor+1, len(m.libraries))
		}
	}
	libsPanel := libsStyle.Render(libsContent)

	// Layout
	content := lipgloss.JoinHorizontal(lipgloss.Top, systemsPanel, libsPanel)

	// Help bar
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1)

	help := "Tab: switch | j/k: nav | Enter: details | s: scan | r: refresh | R: rename | ?: help | q: quit"

	// Status bar
	statusStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("235")).
		Foreground(lipgloss.Color("241")).
		Width(m.width)
	status := fmt.Sprintf(" DB: %s", getDBPath())
	if m.statusMsg != "" {
		status = " " + m.statusMsg
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("🎮 ROM Manager"),
		content,
		helpStyle.Render(help),
		"\n",
		statusStyle.Render(status),
	)
}

func (m model) viewDetail() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205"))

	// Filter tabs
	tabStyle := lipgloss.NewStyle().Padding(0, 1)
	activeTabStyle := tabStyle.Background(lipgloss.Color("205")).Foreground(lipgloss.Color("0"))

	tabs := []struct {
		name   string
		filter detailFilter
	}{
		{"Matched", filterMatched},
		{"Missing", filterMissing},
		{"Flagged", filterFlagged},
		{"Unmatched", filterUnmatched},
		{"Preferred", filterPreferred},
		{"Duplicates", filterDuplicates},
	}

	var tabBar string
	for i, t := range tabs {
		style := tabStyle
		if t.filter == m.detailFilter {
			style = activeTabStyle
		}

		count := 0
		if m.detailCounts != nil {
			count = m.detailCounts[t.filter]
		}

		label := fmt.Sprintf("[%d] %s (%d)", i+1, t.name, count)
		tabBar += style.Render(label) + " "
	}

	// Content - let height be determined by content, which is controlled by maxShow
	contentStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(m.width - 4)

	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("57")).
		Foreground(lipgloss.Color("255"))

	// Item styles
	matchedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	missingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	flaggedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	unmatchedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	filteredItems := m.getFilteredItems()

	var content string
	if m.loadingDetail {
		content = "\n  Loading..."
	} else if len(filteredItems) == 0 {
		content = "\n  No items found."
	} else {
		// Calculate available lines, accounting for header/footer chrome
		// Reserve extra space for: selected item details (path+matchtype = 2 lines), counter line, padding
		availableHeight := m.height - 18
		if availableHeight < 5 {
			availableHeight = 5 // Minimum reasonable height
		}

		// Adjust for selected item's extra detail lines
		maxShow := availableHeight - 2 // Reserve 2 lines for selected item's path/matchtype

		start := 0
		if m.detailCursor >= maxShow {
			start = m.detailCursor - maxShow + 1
		}
		end := start + maxShow
		if end > len(filteredItems) {
			end = len(filteredItems)
		}
		if start < 0 {
			start = 0
		}

		for i := start; i < end; i++ {
			item := filteredItems[i]

			var style lipgloss.Style
			switch m.detailFilter {
			case filterMatched:
				style = matchedStyle
			case filterMissing:
				style = missingStyle
			case filterFlagged:
				style = flaggedStyle
			case filterUnmatched:
				style = unmatchedStyle
			case filterPreferred:
				if item.Path != "" {
					style = matchedStyle
				} else {
					style = missingStyle
				}
			}

			// Truncate long names to fit width
			maxNameLen := m.width - 10
			if maxNameLen < 20 {
				maxNameLen = 20
			}
			line := item.Name
			if len(line) > maxNameLen {
				line = line[:maxNameLen-3] + "..."
			}
			if item.Flags != "" {
				line += fmt.Sprintf(" [%s]", item.Flags)
			}

			if i == m.detailCursor {
				line = selectedStyle.Render("> " + line)
				if item.Path != "" {
					// Truncate path too
					pathDisplay := item.Path
					if len(pathDisplay) > maxNameLen {
						pathDisplay = "..." + pathDisplay[len(pathDisplay)-maxNameLen+3:]
					}
					line += "\n  " + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(pathDisplay)
				}
				if item.MatchType != "" {
					line += " " + lipgloss.NewStyle().Foreground(lipgloss.Color("57")).Render("("+item.MatchType+")")
				}
			} else {
				line = "  " + style.Render(line)
			}
			content += line + "\n"
		}
		content += fmt.Sprintf("\n  (%d/%d)", m.detailCursor+1, len(filteredItems))
	}

	// Status bar
	statusStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("235")).
		Foreground(lipgloss.Color("241")).
		Width(m.width)

	dbPath := getDBPath()
	lastScan := "Never"
	for _, lib := range m.libraries {
		if lib.Name == m.selectedLib {
			if lib.LastScanAt != "" {
				lastScan = lib.LastScanAt
			}
			break
		}
	}
	status := fmt.Sprintf(" DB: %s | Last Scan: %s", dbPath, lastScan)
	if m.searching {
		status = fmt.Sprintf(" SEARCH: %s", m.searchQuery)
	}

	// Help
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1)
	help := "1-5: filter | /: search | j/k: nav | Esc: back | q: quit"

	statsLine := ""
	if m.detailCounts != nil {
		statsLine = fmt.Sprintf("Matched: %d | Missing: %d | Flagged: %d",
			m.detailCounts[filterMatched],
			m.detailCounts[filterMissing],
			m.detailCounts[filterFlagged])
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render(fmt.Sprintf("📁 %s", m.selectedLib)),
		lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render(statsLine),
		"\n"+tabBar,
		contentStyle.Render(content),
		statusStyle.Render(status),
		helpStyle.Render(help),
	)
}

func (m model) viewHelp() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		MarginBottom(1)

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("212")).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	sectionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("243")).
		Bold(true).
		MarginTop(1)

	var lines []string
	lines = append(lines, titleStyle.Render("⌨️  Keyboard Shortcuts"))
	lines = append(lines, "")

	// Navigation
	lines = append(lines, sectionStyle.Render("Navigation"))
	lines = append(lines, keyStyle.Render("  j/↓")+"  "+descStyle.Render("Move down"))
	lines = append(lines, keyStyle.Render("  k/↑")+"  "+descStyle.Render("Move up"))
	lines = append(lines, keyStyle.Render("  PgUp")+"  "+descStyle.Render("Page up"))
	lines = append(lines, keyStyle.Render("  PgDn")+"  "+descStyle.Render("Page down"))
	lines = append(lines, keyStyle.Render("  Tab")+"  "+descStyle.Render("Switch panel"))
	lines = append(lines, keyStyle.Render("  Enter")+"  "+descStyle.Render("Open library details"))
	lines = append(lines, keyStyle.Render("  Esc")+"  "+descStyle.Render("Go back"))

	// Actions
	lines = append(lines, sectionStyle.Render("Actions"))
	lines = append(lines, keyStyle.Render("  s")+"  "+descStyle.Render("Scan selected library/system"))
	lines = append(lines, keyStyle.Render("  r")+"  "+descStyle.Render("Refresh data"))
	lines = append(lines, keyStyle.Render("  R")+"  "+descStyle.Render("Rename files to DAT names"))
	lines = append(lines, keyStyle.Render("  /")+"  "+descStyle.Render("Search in detail view"))

	// Detail View Filters
	lines = append(lines, sectionStyle.Render("Detail View Filters"))
	lines = append(lines, keyStyle.Render("  1/m")+"  "+descStyle.Render("Matched"))
	lines = append(lines, keyStyle.Render("  2/i")+"  "+descStyle.Render("Missing"))
	lines = append(lines, keyStyle.Render("  3/f")+"  "+descStyle.Render("Flagged"))
	lines = append(lines, keyStyle.Render("  4/u")+"  "+descStyle.Render("Unmatched"))
	lines = append(lines, keyStyle.Render("  5/p")+"  "+descStyle.Render("Preferred"))
	lines = append(lines, keyStyle.Render("  6/d")+"  "+descStyle.Render("Duplicates"))

	// General
	lines = append(lines, sectionStyle.Render("General"))
	lines = append(lines, keyStyle.Render("  ?")+"  "+descStyle.Render("Toggle this help"))
	lines = append(lines, keyStyle.Render("  q")+"  "+descStyle.Render("Quit"))

	lines = append(lines, "")
	lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("Press any key to close"))

	content := strings.Join(lines, "\n")

	// Center the help box
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Padding(1, 2).
		Width(50)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		boxStyle.Render(content))
}

// Messages
type systemsMsg struct {
	systems []systemInfo
	err     error
}

type librariesMsg struct {
	libraries []libraryInfo
	err       error
}

type scanCompleteMsg struct {
	err error
}

type detailMsg struct {
	items  []detailItem
	counts map[detailFilter]int
}

type renameMsg struct {
	renamed int
	err     error
}

// Commands
func loadSystems() tea.Msg {
	database, err := db.Open(context.Background(), getDBPath())
	if err != nil {
		return systemsMsg{err: err}
	}
	defer func() { _ = database.Close() }()

	rows, err := database.Conn().Query(`
		SELECT s.name, COALESCE(s.dat_name, ''), COUNT(r.id)
		FROM systems s
		LEFT JOIN releases r ON r.system_id = s.id
		GROUP BY s.id
		ORDER BY s.name
	`)
	if err != nil {
		return systemsMsg{err: err}
	}
	defer func() { _ = rows.Close() }()

	var systems []systemInfo
	for rows.Next() {
		var s systemInfo
		if err := rows.Scan(&s.Name, &s.DatName, &s.ReleaseCount); err != nil {
			return systemsMsg{err: err}
		}
		systems = append(systems, s)
	}

	return systemsMsg{systems: systems}
}

func loadLibraries() tea.Msg {
	database, err := db.Open(context.Background(), getDBPath())
	if err != nil {
		return librariesMsg{err: err}
	}
	defer func() { _ = database.Close() }()

	rows, err := database.Conn().Query(`
		SELECT l.name, s.name, l.root_path, l.system_id,
			(SELECT COUNT(DISTINCT re.release_id) 
			 FROM scanned_files sf 
			 JOIN matches m ON m.scanned_file_id = sf.id 
			 JOIN rom_entries re ON re.id = m.rom_entry_id
			 WHERE sf.library_id = l.id) as games_in_lib,
			(SELECT COUNT(*) FROM releases WHERE system_id = l.system_id) as total_games,
			COALESCE(l.last_scan_at, '')
		FROM libraries l
		JOIN systems s ON s.id = l.system_id
		ORDER BY l.name
	`)
	if err != nil {
		return librariesMsg{err: err}
	}
	defer func() { _ = rows.Close() }()

	var libraries []libraryInfo
	for rows.Next() {
		var lib libraryInfo
		if err := rows.Scan(&lib.Name, &lib.System, &lib.Path, &lib.SystemID, &lib.GamesInLib, &lib.TotalGames, &lib.LastScanAt); err != nil {
			return librariesMsg{err: err}
		}
		libraries = append(libraries, lib)
	}

	return librariesMsg{libraries: libraries}
}

func loadDetail(libName string, filter detailFilter) tea.Cmd {
	return func() tea.Msg {
		database, err := db.Open(context.Background(), getDBPath())
		if err != nil {
			return detailMsg{}
		}
		defer func() { _ = database.Close() }()

		var items []detailItem

		switch filter {
		case filterMatched:
			// Files that matched with sha1 or crc32
			rows, err := database.Conn().Query(`
				SELECT r.name, sf.path, m.match_type, COALESCE(m.flags, '')
				FROM scanned_files sf
				JOIN matches m ON m.scanned_file_id = sf.id
				JOIN rom_entries re ON re.id = m.rom_entry_id
				JOIN releases r ON r.id = re.release_id
				JOIN libraries l ON l.id = sf.library_id
				WHERE l.name = ? AND m.match_type IN ('sha1', 'crc32')
				ORDER BY r.name
			`, libName)
			if err == nil {
				defer func() { _ = rows.Close() }()
				for rows.Next() {
					var item detailItem
					_ = rows.Scan(&item.Name, &item.Path, &item.MatchType, &item.Flags)
					items = append(items, item)
				}
			}

		case filterMissing:
			// Releases in system that have no matched files
			rows, err := database.Conn().Query(`
				SELECT r.name
				FROM releases r
				JOIN libraries l ON l.system_id = r.system_id
				WHERE l.name = ?
				AND r.id NOT IN (
					SELECT DISTINCT re.release_id
					FROM scanned_files sf
					JOIN matches m ON m.scanned_file_id = sf.id
					JOIN rom_entries re ON re.id = m.rom_entry_id
					WHERE sf.library_id = l.id
				)
				ORDER BY r.name
			`, libName)
			if err == nil {
				defer func() { _ = rows.Close() }()
				for rows.Next() {
					var item detailItem
					_ = rows.Scan(&item.Name)
					items = append(items, item)
				}
			}

		case filterFlagged:
			// Files matched by name with flags (cracked, bad-dump, etc)
			rows, err := database.Conn().Query(`
				SELECT r.name, sf.path, m.match_type, m.flags
				FROM scanned_files sf
				JOIN matches m ON m.scanned_file_id = sf.id
				JOIN rom_entries re ON re.id = m.rom_entry_id
				JOIN releases r ON r.id = re.release_id
				JOIN libraries l ON l.id = sf.library_id
				WHERE l.name = ? AND m.flags IS NOT NULL AND m.flags != ''
				ORDER BY r.name
			`, libName)
			if err == nil {
				defer func() { _ = rows.Close() }()
				for rows.Next() {
					var item detailItem
					_ = rows.Scan(&item.Name, &item.Path, &item.MatchType, &item.Flags)
					items = append(items, item)
				}
			}

		case filterUnmatched:
			// Scanned files with no match
			rows, err := database.Conn().Query(`
				SELECT sf.path
				FROM scanned_files sf
				JOIN libraries l ON l.id = sf.library_id
				LEFT JOIN matches m ON m.scanned_file_id = sf.id
				WHERE l.name = ? AND m.id IS NULL
				ORDER BY sf.path
			`, libName)
			if err == nil {
				defer func() { _ = rows.Close() }()
				for rows.Next() {
					var item detailItem
					_ = rows.Scan(&item.Path)
					item.Name = item.Path
					items = append(items, item)
				}
			}

		case filterPreferred:
			// Preferred releases for the system with match status
			rows, err := database.Conn().Query(`
				SELECT r.name, 
					COALESCE((SELECT sf.path FROM scanned_files sf 
					          JOIN matches m ON m.scanned_file_id = sf.id 
							  JOIN rom_entries re ON re.id = m.rom_entry_id 
							  WHERE re.release_id = r.id AND sf.library_id = l.id LIMIT 1), ''),
					COALESCE((SELECT m.match_type FROM scanned_files sf 
					          JOIN matches m ON m.scanned_file_id = sf.id 
							  JOIN rom_entries re ON re.id = m.rom_entry_id 
							  WHERE re.release_id = r.id AND sf.library_id = l.id LIMIT 1), ''),
					'' as flags
				FROM releases r
				JOIN libraries l ON l.system_id = r.system_id
				WHERE l.name = ? AND r.is_preferred = 1
				ORDER BY r.name
			`, libName)
			if err == nil {
				defer func() { _ = rows.Close() }()
				for rows.Next() {
					var item detailItem
					_ = rows.Scan(&item.Name, &item.Path, &item.MatchType, &item.Flags)
					items = append(items, item)
				}
			}

		case filterDuplicates:
			// Find duplicate files (multiple files matching the same release)
			rows, err := database.Conn().Query(`
				SELECT r.name, sf.path, m.match_type, COALESCE(m.flags, ''), r.id as dup_group
				FROM scanned_files sf
				JOIN matches m ON m.scanned_file_id = sf.id
				JOIN rom_entries re ON re.id = m.rom_entry_id
				JOIN releases r ON r.id = re.release_id
				JOIN libraries l ON l.id = sf.library_id
				WHERE l.name = ?
				AND r.id IN (
					SELECT re2.release_id
					FROM scanned_files sf2
					JOIN matches m2 ON m2.scanned_file_id = sf2.id
					JOIN rom_entries re2 ON re2.id = m2.rom_entry_id
					JOIN libraries l2 ON l2.id = sf2.library_id
					WHERE l2.name = ?
					GROUP BY re2.release_id
					HAVING COUNT(DISTINCT sf2.id) > 1
				)
				ORDER BY r.name, sf.path
			`, libName, libName)
			if err == nil {
				defer func() { _ = rows.Close() }()
				for rows.Next() {
					var item detailItem
					_ = rows.Scan(&item.Name, &item.Path, &item.MatchType, &item.Flags, &item.DupGroup)
					items = append(items, item)
				}
			}
		}

		// Calculate counts for all filters
		counts := make(map[detailFilter]int)
		var c int

		// Matched
		_ = database.Conn().QueryRow(`
			SELECT COUNT(DISTINCT re.release_id)
			FROM scanned_files sf
			JOIN matches m ON m.scanned_file_id = sf.id
			JOIN rom_entries re ON re.id = m.rom_entry_id
			JOIN libraries l ON l.id = sf.library_id
			WHERE l.name = ?
		`, libName).Scan(&c)
		counts[filterMatched] = c

		// Missing
		_ = database.Conn().QueryRow(`
			SELECT COUNT(DISTINCT r.id)
			FROM releases r
			JOIN libraries l ON l.system_id = r.system_id
			WHERE l.name = ?
			AND r.id NOT IN (
				SELECT DISTINCT re.release_id
				FROM scanned_files sf
				JOIN matches m ON m.scanned_file_id = sf.id
				JOIN rom_entries re ON re.id = m.rom_entry_id
				WHERE sf.library_id = l.id
			)
		`, libName).Scan(&c)
		counts[filterMissing] = c

		// Flagged
		_ = database.Conn().QueryRow(`
			SELECT COUNT(DISTINCT re.release_id)
			FROM scanned_files sf
			JOIN matches m ON m.scanned_file_id = sf.id
			JOIN rom_entries re ON re.id = m.rom_entry_id
			JOIN libraries l ON l.id = sf.library_id
			WHERE l.name = ? AND m.flags IS NOT NULL AND m.flags != ''
		`, libName).Scan(&c)
		counts[filterFlagged] = c

		// Unmatched
		_ = database.Conn().QueryRow(`
			SELECT COUNT(*)
			FROM scanned_files sf
			JOIN libraries l ON l.id = sf.library_id
			LEFT JOIN matches m ON m.scanned_file_id = sf.id
			WHERE l.name = ? AND m.id IS NULL
		`, libName).Scan(&c)
		counts[filterUnmatched] = c

		// Preferred
		_ = database.Conn().QueryRow(`
			SELECT COUNT(*)
			FROM releases r
			JOIN libraries l ON l.system_id = r.system_id
			WHERE l.name = ? AND r.is_preferred = 1
		`, libName).Scan(&c)
		counts[filterPreferred] = c

		// Duplicates (count of releases with multiple files)
		_ = database.Conn().QueryRow(`
			SELECT COUNT(*)
			FROM (
				SELECT re.release_id
				FROM scanned_files sf
				JOIN matches m ON m.scanned_file_id = sf.id
				JOIN rom_entries re ON re.id = m.rom_entry_id
				JOIN libraries l ON l.id = sf.library_id
				WHERE l.name = ?
				GROUP BY re.release_id
				HAVING COUNT(DISTINCT sf.id) > 1
			)
		`, libName).Scan(&c)
		counts[filterDuplicates] = c

		return detailMsg{items: items, counts: counts}
	}
}

func scanLibrary(name string) tea.Cmd {
	return func() tea.Msg {
		database, err := db.Open(context.Background(), getDBPath())
		if err != nil {
			return scanCompleteMsg{err: err}
		}
		defer func() { _ = database.Close() }()

		scanner := library.NewScanner(database.Conn())
		_, err = scanner.Scan(context.Background(), name)

		return scanCompleteMsg{err: err}
	}
}

// scanLibraries scans multiple libraries sequentially
func scanLibraries(names []string) tea.Cmd {
	return func() tea.Msg {
		database, err := db.Open(context.Background(), getDBPath())
		if err != nil {
			return scanCompleteMsg{err: err}
		}
		defer func() { _ = database.Close() }()

		scanner := library.NewScanner(database.Conn())
		var lastErr error
		for _, name := range names {
			if _, err := scanner.Scan(context.Background(), name); err != nil {
				lastErr = err
			}
		}

		return scanCompleteMsg{err: lastErr}
	}
}

func getDBPath() string {
	if path := os.Getenv("ROMMAN_DB"); path != "" {
		return path
	}
	return "romman.db"
}

func renderProgressBar(pct int, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}

	fullChars := (pct * width) / 100
	emptyChars := width - fullChars

	filled := strings.Repeat("█", fullChars)
	empty := strings.Repeat("░", emptyChars)

	color := "2" // Green
	if pct < 33 {
		color = "1" // Red
	} else if pct < 66 {
		color = "3" // Yellow
	}

	barStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	return barStyle.Render(filled) + lipgloss.NewStyle().Foreground(lipgloss.Color("235")).Render(empty)
}

func renameLibraryFiles(libName string) tea.Cmd {
	return func() tea.Msg {
		database, err := db.Open(context.Background(), getDBPath())
		if err != nil {
			return renameMsg{err: err}
		}
		defer func() { _ = database.Close() }()

		manager := library.NewManager(database.Conn())
		renamer := library.NewRenamer(database.Conn(), manager)

		result, err := renamer.Rename(context.Background(), libName, false) // dryRun=false
		if err != nil {
			return renameMsg{err: err}
		}

		return renameMsg{renamed: result.Renamed}
	}
}
