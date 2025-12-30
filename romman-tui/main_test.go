package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestInitialModel(t *testing.T) {
	m := initialModel()

	assert.Equal(t, panelSystems, m.panel, "initial panel should be systems")
	assert.Equal(t, 0, m.cursor, "initial cursor should be 0")
	assert.False(t, m.inDetail, "should not start in detail view")
	assert.Empty(t, m.systems, "systems should be empty initially")
	assert.Empty(t, m.libraries, "libraries should be empty initially")
}

func TestPanelNavigation(t *testing.T) {
	m := initialModel()

	// Tab switches to libraries
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab")})
	m = newM.(model)
	assert.Equal(t, panelLibraries, m.panel)
	assert.Equal(t, 0, m.cursor, "cursor should reset on panel switch")

	// Tab switches back to systems
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab")})
	m = newM.(model)
	assert.Equal(t, panelSystems, m.panel)
}

func TestCursorNavigation(t *testing.T) {
	m := initialModel()
	m.systems = []systemInfo{
		{Name: "md", ReleaseCount: 100},
		{Name: "snes", ReleaseCount: 200},
		{Name: "nes", ReleaseCount: 300},
	}

	// Down moves cursor
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = newM.(model)
	assert.Equal(t, 1, m.cursor)

	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = newM.(model)
	assert.Equal(t, 2, m.cursor)

	// Can't go past end
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = newM.(model)
	assert.Equal(t, 2, m.cursor, "cursor should stop at end")

	// Up moves cursor back
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = newM.(model)
	assert.Equal(t, 1, m.cursor)

	// Can't go before start
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = newM.(model)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = newM.(model)
	assert.Equal(t, 0, m.cursor, "cursor should stop at start")
}

func TestDetailViewEntry(t *testing.T) {
	m := initialModel()
	m.panel = panelLibraries
	m.libraries = []libraryInfo{
		{Name: "megadrive", System: "md", GamesInLib: 36, TotalGames: 2461},
	}

	// Enter opens detail view
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(model)

	assert.True(t, m.inDetail, "should be in detail view")
	assert.Equal(t, "megadrive", m.selectedLib)
	assert.Equal(t, filterMatched, m.detailFilter, "default filter should be matched")
	assert.NotNil(t, cmd, "should return command to load detail")
}

func TestDetailViewNavigation(t *testing.T) {
	m := initialModel()
	m.inDetail = true
	m.selectedLib = "megadrive"
	m.detailFilter = filterMatched
	m.detailItems = []detailItem{
		{Name: "Game 1"},
		{Name: "Game 2"},
		{Name: "Game 3"},
	}

	// Filter switching
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	m = newM.(model)
	assert.Equal(t, filterMissing, m.detailFilter)

	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	m = newM.(model)
	assert.Equal(t, filterFlagged, m.detailFilter)

	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
	m = newM.(model)
	assert.Equal(t, filterUnmatched, m.detailFilter)

	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
	m = newM.(model)
	assert.Equal(t, filterMatched, m.detailFilter)
}

func TestDetailViewExit(t *testing.T) {
	m := initialModel()
	m.inDetail = true
	m.selectedLib = "megadrive"
	m.detailItems = []detailItem{{Name: "Game 1"}}

	// Esc exits detail view
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newM.(model)

	assert.False(t, m.inDetail, "should exit detail view")
	assert.Empty(t, m.detailItems, "detail items should be cleared")
}

func TestQuitCommand(t *testing.T) {
	m := initialModel()

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	assert.NotNil(t, cmd, "should return quit command")
}

func TestWindowSizeUpdate(t *testing.T) {
	m := initialModel()

	newM, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = newM.(model)

	assert.Equal(t, 120, m.width)
	assert.Equal(t, 40, m.height)
}

func TestSystemsMessage(t *testing.T) {
	m := initialModel()

	systems := []systemInfo{
		{Name: "md", DatName: "Sega Mega Drive", ReleaseCount: 2461},
	}

	newM, _ := m.Update(systemsMsg{systems: systems})
	m = newM.(model)

	assert.Len(t, m.systems, 1)
	assert.Equal(t, "md", m.systems[0].Name)
	assert.Equal(t, 2461, m.systems[0].ReleaseCount)
}

func TestLibrariesMessage(t *testing.T) {
	m := initialModel()

	libs := []libraryInfo{
		{Name: "megadrive", System: "md", GamesInLib: 36, TotalGames: 2461},
	}

	newM, _ := m.Update(librariesMsg{libraries: libs})
	m = newM.(model)

	assert.Len(t, m.libraries, 1)
	assert.Equal(t, "megadrive", m.libraries[0].Name)
	assert.Equal(t, 36, m.libraries[0].GamesInLib)
}

func TestViewLoading(t *testing.T) {
	m := initialModel()
	// width 0 means not yet sized

	view := m.View()
	assert.Equal(t, "Loading...", view)
}

func TestMaxItems(t *testing.T) {
	m := initialModel()
	m.systems = []systemInfo{{}, {}, {}}
	m.libraries = []libraryInfo{{}, {}}

	m.panel = panelSystems
	assert.Equal(t, 3, m.maxItems())

	m.panel = panelLibraries
	assert.Equal(t, 2, m.maxItems())
}
