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
}

type panel int

const (
	panelSystems panel = iota
	panelLibraries
)

type systemInfo struct {
	Name         string
	DatName      string
	ReleaseCount int
}

type libraryInfo struct {
	Name    string
	System  string
	Path    string
	Matched int
	Total   int
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
			line := fmt.Sprintf("%-8s %d releases", s.Name, s.ReleaseCount)
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
			if lib.Total > 0 {
				pct = lib.Matched * 100 / lib.Total
			}
			line := fmt.Sprintf("%-12s %3d%% (%d/%d)", lib.Name, pct, lib.Matched, lib.Total)
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

	help := "Tab: switch panel | j/k: navigate | s: scan | r: refresh | q: quit"

	return lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("🎮 ROM Manager"),
		content,
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
		SELECT l.name, s.name, l.root_path,
			(SELECT COUNT(*) FROM scanned_files sf 
			 JOIN matches m ON m.scanned_file_id = sf.id 
			 WHERE sf.library_id = l.id) as matched,
			(SELECT COUNT(*) FROM scanned_files WHERE library_id = l.id) as total
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
		if err := rows.Scan(&lib.Name, &lib.System, &lib.Path, &lib.Matched, &lib.Total); err != nil {
			return librariesMsg{err: err}
		}
		libraries = append(libraries, lib)
	}

	return librariesMsg{libraries: libraries}
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
