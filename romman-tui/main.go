package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ryanm/romman-lib/db"
	"github.com/ryanm/romman-lib/library"
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
	inDetail     bool
	detailFilter detailFilter
	detailItems  []detailItem
	detailCursor int
	selectedLib  string
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
}

type detailItem struct {
	Name      string
	Path      string
	MatchType string
	Flags     string
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
				if m.detailCursor < len(m.detailItems)-1 {
					m.detailCursor++
				}
			case "1", "m": // Matched
				m.detailFilter = filterMatched
				m.detailCursor = 0
				return m, loadDetail(m.selectedLib, m.detailFilter)
			case "2", "i": // mIssing
				m.detailFilter = filterMissing
				m.detailCursor = 0
				return m, loadDetail(m.selectedLib, m.detailFilter)
			case "3", "f": // Flagged
				m.detailFilter = filterFlagged
				m.detailCursor = 0
				return m, loadDetail(m.selectedLib, m.detailFilter)
			case "4", "u": // Unmatched
				m.detailFilter = filterUnmatched
				m.detailCursor = 0
				return m, loadDetail(m.selectedLib, m.detailFilter)
			case "5", "p": // Preferred
				m.detailFilter = filterPreferred
				m.detailCursor = 0
				return m, loadDetail(m.selectedLib, m.detailFilter)
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
		case "enter":
			if m.panel == panelLibraries && m.cursor < len(m.libraries) {
				m.inDetail = true
				m.selectedLib = m.libraries[m.cursor].Name
				m.detailFilter = filterMatched
				m.detailCursor = 0
				return m, loadDetail(m.selectedLib, m.detailFilter)
			}
		case "s":
			if m.panel == panelLibraries && m.cursor < len(m.libraries) {
				return m, scanLibrary(m.libraries[m.cursor].Name)
			}
		case "r":
			return m, tea.Batch(loadSystems, loadLibraries)
		}

	case systemsMsg:
		m.systems = msg.systems
		m.err = msg.err

	case librariesMsg:
		m.libraries = msg.libraries
		m.err = msg.err

	case scanCompleteMsg:
		return m, loadLibraries

	case detailMsg:
		m.detailItems = msg.items
	}

	return m, nil
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
		for i, s := range m.systems {
			line := fmt.Sprintf("%-8s %d games", s.Name, s.ReleaseCount)
			if m.panel == panelSystems && i == m.cursor {
				line = selectedStyle.Render(line)
			}
			systemsContent += line + "\n"
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
	if len(m.libraries) == 0 {
		libsContent += "No libraries configured.\nUse CLI: romman library add"
	} else {
		for i, lib := range m.libraries {
			pct := 0
			if lib.TotalGames > 0 {
				pct = lib.GamesInLib * 100 / lib.TotalGames
			}
			line := fmt.Sprintf("%-12s %3d%% (%d/%d)", lib.Name, pct, lib.GamesInLib, lib.TotalGames)
			if m.panel == panelLibraries && i == m.cursor {
				line = selectedStyle.Render(line)
			}
			libsContent += line + "\n"
		}
	}
	libsPanel := libsStyle.Render(libsContent)

	// Layout
	content := lipgloss.JoinHorizontal(lipgloss.Top, systemsPanel, libsPanel)

	// Help bar
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1)

	help := "Tab: switch | j/k: nav | Enter: details | s: scan | r: refresh | q: quit"

	return lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("🎮 ROM Manager"),
		content,
		helpStyle.Render(help),
	)
}

func (m model) viewDetail() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205"))

	// Filter tabs
	tabStyle := lipgloss.NewStyle().Padding(0, 2)
	activeTabStyle := tabStyle.Background(lipgloss.Color("205")).Foreground(lipgloss.Color("0"))

	tabs := []struct {
		name   string
		filter detailFilter
	}{
		{"[1] Matched", filterMatched},
		{"[2] Missing", filterMissing},
		{"[3] Flagged", filterFlagged},
		{"[4] Unmatched", filterUnmatched},
		{"[5] Preferred", filterPreferred},
	}

	var tabBar string
	for _, t := range tabs {
		style := tabStyle
		if t.filter == m.detailFilter {
			style = activeTabStyle
		}
		tabBar += style.Render(t.name) + " "
	}

	// Content
	contentStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Width(m.width - 4).
		Height(m.height - 8)

	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("57")).
		Foreground(lipgloss.Color("255"))

	var content string
	if len(m.detailItems) == 0 {
		content = "No items found."
	} else {
		maxShow := m.height - 12
		start := 0
		if m.detailCursor >= maxShow {
			start = m.detailCursor - maxShow + 1
		}
		end := start + maxShow
		if end > len(m.detailItems) {
			end = len(m.detailItems)
		}

		for i := start; i < end; i++ {
			item := m.detailItems[i]
			line := item.Name
			if item.Flags != "" {
				line += fmt.Sprintf(" [%s]", item.Flags)
			}
			if i == m.detailCursor {
				line = selectedStyle.Render(line)
			}
			content += line + "\n"
		}
		content += fmt.Sprintf("\n(%d/%d)", m.detailCursor+1, len(m.detailItems))
	}

	// Help
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1)
	help := "1-4: filter | j/k: nav | Esc: back | q: quit"

	return lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render(fmt.Sprintf("📁 %s", m.selectedLib)),
		tabBar,
		contentStyle.Render(content),
		helpStyle.Render(help),
	)
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

type scanCompleteMsg struct{}

type detailMsg struct {
	items []detailItem
}

// Commands
func loadSystems() tea.Msg {
	database, err := db.Open(getDBPath())
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
	database, err := db.Open(getDBPath())
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
			(SELECT COUNT(*) FROM releases WHERE system_id = l.system_id) as total_games
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
		if err := rows.Scan(&lib.Name, &lib.System, &lib.Path, &lib.SystemID, &lib.GamesInLib, &lib.TotalGames); err != nil {
			return librariesMsg{err: err}
		}
		libraries = append(libraries, lib)
	}

	return librariesMsg{libraries: libraries}
}

func loadDetail(libName string, filter detailFilter) tea.Cmd {
	return func() tea.Msg {
		database, err := db.Open(getDBPath())
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
			// Preferred releases for the system
			rows, err := database.Conn().Query(`
				SELECT r.name
				FROM releases r
				JOIN libraries l ON l.system_id = r.system_id
				WHERE l.name = ? AND r.is_preferred = 1
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
		}

		return detailMsg{items: items}
	}
}

func scanLibrary(name string) tea.Cmd {
	return func() tea.Msg {
		database, err := db.Open(getDBPath())
		if err != nil {
			return scanCompleteMsg{}
		}
		defer func() { _ = database.Close() }()

		scanner := library.NewScanner(database.Conn())
		_, _ = scanner.Scan(name)

		return scanCompleteMsg{}
	}
}

func getDBPath() string {
	if path := os.Getenv("ROMMAN_DB"); path != "" {
		return path
	}
	return "romman.db"
}
