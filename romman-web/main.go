package main

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/ryanm101/romman-lib/config"
	"github.com/ryanm101/romman-lib/db"
	"github.com/ryanm101/romman-lib/library"
	"github.com/ryanm101/romman-lib/metrics"
	"github.com/ryanm101/romman-lib/pack"
	"github.com/ryanm101/romman-lib/tracing"
)

//go:embed assets/*
var assets embed.FS

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Printf("Warning: failed to load config: %v", err)
		cfg = config.DefaultConfig()
	}

	// Setup Tracing context early for database operations
	ctx := context.Background()

	database, err := db.Open(ctx, cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer func() { _ = database.Close() }()

	server := NewServer(database.Conn())

	port := os.Getenv("ROMMAN_PORT")
	if port == "" {
		port = "8080"
	}

	// Setup Tracing
	shutdown, err := tracing.Setup(ctx, tracing.Config{
		Enabled:  os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "",
		Endpoint: os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
	})
	if err != nil {
		log.Printf("Warning: failed to setup tracing: %v", err)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			log.Printf("Error shutting down tracer: %v", err)
		}
	}()

	fmt.Printf("🌐 ROM Manager Web UI\n")
	fmt.Printf("   http://localhost:%s\n\n", port)

	// Wrap handler with otelhttp
	handler := otelhttp.NewHandler(server, "romman-web",
		otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
			return fmt.Sprintf("%s %s", r.Method, r.URL.Path)
		}),
	)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}

// Server handles HTTP requests.
type Server struct {
	db        *sql.DB
	mux       *http.ServeMux
	mediaRoot string
}

// NewServer creates a new web server.
func NewServer(conn *sql.DB) *Server {
	home, _ := os.UserHomeDir()
	s := &Server{
		db:        conn,
		mux:       http.NewServeMux(),
		mediaRoot: fmt.Sprintf("%s/.romman/media", home),
	}
	s.setupRoutes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) setupRoutes() {
	s.mux.HandleFunc("/api/systems", s.handleSystems)
	s.mux.HandleFunc("/api/libraries", s.handleLibraries)
	s.mux.HandleFunc("/api/stats", s.handleStats)
	s.mux.HandleFunc("/api/scan", s.handleScan)
	s.mux.HandleFunc("/api/scan-all", s.handleScanAll)
	s.mux.HandleFunc("/api/details", s.handleDetails)
	s.mux.HandleFunc("/api/counts", s.handleCounts)
	s.mux.HandleFunc("/api/media/", s.handleMedia) // Note trailing slash for prefix matching
	s.mux.HandleFunc("/api/packs/games", s.handlePackGames)
	s.mux.HandleFunc("/api/packs/generate", s.handlePackGenerate)
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/metrics", s.handleMetrics)
	s.mux.HandleFunc("/", s.handleDashboard)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	var systems, libraries, releases int

	_ = s.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM systems").Scan(&systems)
	_ = s.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM libraries").Scan(&libraries)
	_ = s.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM releases").Scan(&releases)

	data := map[string]int{
		"totalSystems":   systems,
		"totalLibraries": libraries,
		"totalReleases":  releases,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

func (s *Server) handleSystems(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT s.name, COUNT(r.id) as releases,
			COUNT(CASE WHEN r.is_preferred = 1 THEN 1 END) as preferred
		FROM systems s
		LEFT JOIN releases r ON r.system_id = s.id
		GROUP BY s.id ORDER BY s.name
	`)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer func() { _ = rows.Close() }()

	var systems []map[string]interface{}
	for rows.Next() {
		var name string
		var releases, preferred int
		if err := rows.Scan(&name, &releases, &preferred); err != nil {
			continue
		}
		systems = append(systems, map[string]interface{}{
			"name":      name,
			"releases":  releases,
			"preferred": preferred,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"systems": systems})
}

func (s *Server) handleLibraries(w http.ResponseWriter, r *http.Request) {
	// Get library info
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT l.id, l.name, s.name as system,
			(SELECT COUNT(*) FROM releases WHERE system_id = l.system_id) as total
		FROM libraries l
		JOIN systems s ON s.id = l.system_id
		ORDER BY l.name
	`)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer func() { _ = rows.Close() }()

	type libInfo struct {
		id      int64
		name    string
		system  string
		matched int
		total   int
	}
	var libList []libInfo
	for rows.Next() {
		var l libInfo
		if err := rows.Scan(&l.id, &l.name, &l.system, &l.total); err != nil {
			continue
		}
		libList = append(libList, l)
	}

	// Get matched counts per library (separate query is faster)
	for i := range libList {
		_ = s.db.QueryRowContext(r.Context(), `
			SELECT COUNT(DISTINCT re.release_id)
			FROM scanned_files sf
			JOIN matches m ON m.scanned_file_id = sf.id
			JOIN rom_entries re ON re.id = m.rom_entry_id
			WHERE sf.library_id = ?
		`, libList[i].id).Scan(&libList[i].matched)
	}

	var libs []map[string]interface{}
	for _, l := range libList {
		pct := 0
		if l.total > 0 {
			pct = l.matched * 100 / l.total
		}
		libs = append(libs, map[string]interface{}{
			"name":     l.name,
			"system":   l.system,
			"matched":  l.matched,
			"total":    l.total,
			"matchPct": pct,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"libraries": libs})
}

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.URL.Query().Get("library")
	if name == "" {
		http.Error(w, "Missing library parameter", http.StatusBadRequest)
		return
	}

	scanner := library.NewScanner(s.db)
	_, err := scanner.Scan(r.Context(), name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleScanAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	system := r.URL.Query().Get("system")
	if system == "" {
		http.Error(w, "Missing system parameter", http.StatusBadRequest)
		return
	}

	// Get all libraries for this system
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT l.name FROM libraries l
		JOIN systems s ON s.id = l.system_id
		WHERE s.name = ?
	`, system)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = rows.Close() }()

	var libNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			libNames = append(libNames, name)
		}
	}

	if len(libNames) == 0 {
		http.Error(w, "No libraries found for system", http.StatusNotFound)
		return
	}

	scanner := library.NewScanner(s.db)
	var scanned int
	for _, name := range libNames {
		if _, err := scanner.Scan(r.Context(), name); err == nil {
			scanned++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"scanned": scanned,
		"total":   len(libNames),
	})
}

func (s *Server) handleCounts(w http.ResponseWriter, r *http.Request) {
	libName := r.URL.Query().Get("library")
	if libName == "" {
		http.Error(w, "Missing library parameter", http.StatusBadRequest)
		return
	}

	var matched, missing, flagged, unmatched, preferred int

	// Matched count
	_ = s.db.QueryRowContext(r.Context(), `
		SELECT COUNT(DISTINCT r.id)
		FROM scanned_files sf
		JOIN matches m ON m.scanned_file_id = sf.id
		JOIN rom_entries re ON re.id = m.rom_entry_id
		JOIN releases r ON r.id = re.release_id
		JOIN libraries l ON l.id = sf.library_id
		WHERE l.name = ? AND m.match_type IN ('sha1', 'crc32')
	`, libName).Scan(&matched)

	// Missing count
	_ = s.db.QueryRowContext(r.Context(), `
		SELECT COUNT(*)
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
	`, libName).Scan(&missing)

	// Flagged count
	_ = s.db.QueryRowContext(r.Context(), `
		SELECT COUNT(DISTINCT r.id)
		FROM scanned_files sf
		JOIN matches m ON m.scanned_file_id = sf.id
		JOIN rom_entries re ON re.id = m.rom_entry_id
		JOIN releases r ON r.id = re.release_id
		JOIN libraries l ON l.id = sf.library_id
		WHERE l.name = ? AND m.flags IS NOT NULL AND m.flags != ''
	`, libName).Scan(&flagged)

	// Unmatched count
	_ = s.db.QueryRowContext(r.Context(), `
		SELECT COUNT(*)
		FROM scanned_files sf
		JOIN libraries l ON l.id = sf.library_id
		LEFT JOIN matches m ON m.scanned_file_id = sf.id
		WHERE l.name = ? AND m.id IS NULL
	`, libName).Scan(&unmatched)

	// Preferred count
	_ = s.db.QueryRowContext(r.Context(), `
		SELECT COUNT(*)
		FROM releases r
		JOIN libraries l ON l.system_id = r.system_id
		WHERE l.name = ? AND r.is_preferred = 1
	`, libName).Scan(&preferred)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int{
		"matched":   matched,
		"missing":   missing,
		"flagged":   flagged,
		"unmatched": unmatched,
		"preferred": preferred,
	})
}

func (s *Server) handleDetails(w http.ResponseWriter, r *http.Request) {
	libName := r.URL.Query().Get("library")
	filter := r.URL.Query().Get("filter")
	if libName == "" {
		http.Error(w, "Missing library parameter", http.StatusBadRequest)
		return
	}

	var items []map[string]string

	switch filter {
	case "matched":
		rows, err := s.db.QueryContext(r.Context(), `
			SELECT r.name, sf.path, m.match_type, COALESCE(m.flags, ''),
				COALESCE(gm.local_path, ''), COALESCE(gmd.description, '')
			FROM scanned_files sf
			JOIN matches m ON m.scanned_file_id = sf.id
			JOIN rom_entries re ON re.id = m.rom_entry_id
			JOIN releases r ON r.id = re.release_id
			JOIN libraries l ON l.id = sf.library_id
			LEFT JOIN game_media gm ON gm.release_id = r.id AND gm.type = 'boxart'
			LEFT JOIN game_metadata gmd ON gmd.release_id = r.id
			WHERE l.name = ? AND m.match_type IN ('sha1', 'crc32')
			ORDER BY r.name
		`, libName)
		if err == nil {
			defer func() { _ = rows.Close() }()
			for rows.Next() {
				var name, path, matchType, flags, mediaPath, desc string
				_ = rows.Scan(&name, &path, &matchType, &flags, &mediaPath, &desc)

				item := map[string]string{
					"name": name, "path": path, "matchType": matchType,
					"flags": flags, "status": "matched",
					"description": desc,
				}
				if mediaPath != "" {
					if rel, err := filepath.Rel(s.mediaRoot, mediaPath); err == nil {
						item["boxart"] = "/api/media/" + rel
					}
				}
				items = append(items, item)
			}
		}
	case "missing":
		rows, err := s.db.QueryContext(r.Context(), `
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
				var name string
				_ = rows.Scan(&name)
				items = append(items, map[string]string{"name": name, "status": "missing"})
			}
		}
	case "flagged":
		rows, err := s.db.QueryContext(r.Context(), `
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
				var name, path, matchType, flags string
				_ = rows.Scan(&name, &path, &matchType, &flags)
				items = append(items, map[string]string{"name": name, "path": path, "matchType": matchType, "flags": flags, "status": "flagged"})
			}
		}
	case "unmatched":
		rows, err := s.db.QueryContext(r.Context(), `
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
				var path string
				_ = rows.Scan(&path)
				items = append(items, map[string]string{"name": path, "path": path, "status": "unmatched"})
			}
		}
	case "preferred":
		rows, err := s.db.QueryContext(r.Context(), `
			SELECT r.name, 
				COALESCE((SELECT sf.path FROM scanned_files sf 
						  JOIN matches m ON m.scanned_file_id = sf.id 
						  JOIN rom_entries re ON re.id = m.rom_entry_id 
						  WHERE re.release_id = r.id AND sf.library_id = (SELECT id FROM libraries WHERE name = ?) LIMIT 1), ''),
				COALESCE((SELECT m.match_type FROM scanned_files sf 
						  JOIN matches m ON m.scanned_file_id = sf.id 
						  JOIN rom_entries re ON re.id = m.rom_entry_id 
						  WHERE re.release_id = r.id AND sf.library_id = (SELECT id FROM libraries WHERE name = ?) LIMIT 1), '')
			FROM releases r
			JOIN libraries l ON l.system_id = r.system_id
			WHERE l.name = ? AND r.is_preferred = 1
			ORDER BY r.name
		`, libName, libName, libName)
		if err == nil {
			defer func() { _ = rows.Close() }()
			for rows.Next() {
				var name, path, matchType string
				_ = rows.Scan(&name, &path, &matchType)
				status := "missing"
				if path != "" {
					status = "matched"
				}
				items = append(items, map[string]string{"name": name, "path": path, "matchType": matchType, "status": status})
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"items": items})
}

func (s *Server) handleMedia(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Path: /api/media/snes/123-boxart.jpg
	// Strip prefix
	relPath := r.URL.Path[len("/api/media/"):]
	if relPath == "" || strings.Contains(relPath, "..") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	fullPath := filepath.Join(s.mediaRoot, relPath)
	http.ServeFile(w, r, fullPath)
}

func (s *Server) handleDashboard(w http.ResponseWriter, _ *http.Request) {
	content, err := assets.ReadFile("assets/index.html")
	if err != nil {
		http.Error(w, "Dashboard not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(content)
}
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if err := metrics.UpdateDBMetrics(s.db); err != nil {
		log.Printf("Error updating metrics: %v", err)
	}
	promhttp.Handler().ServeHTTP(w, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	err := s.db.Ping()
	status := "healthy"
	statusCode := http.StatusOK

	if err != nil {
		status = "unhealthy"
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": status,
		"db":     fmt.Sprintf("%v", err == nil),
	})
}

// handlePackGames returns games available for packing, grouped by system.
func (s *Server) handlePackGames(w http.ResponseWriter, r *http.Request) {
	// Query matched games with file paths
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT 
			s.name as system_id,
			COALESCE(s.dat_name, s.name) as system_name,
			r.id as release_id,
			r.name as game_name,
			sf.path as file_path,
			COALESCE(sf.archive_path, '') as entry_name,
			sf.size
		FROM systems s
		JOIN releases r ON r.system_id = s.id
		JOIN rom_entries re ON re.release_id = r.id
		JOIN matches m ON m.rom_entry_id = re.id
		JOIN scanned_files sf ON m.scanned_file_id = sf.id
		WHERE r.is_preferred = 1 OR NOT EXISTS (
			SELECT 1 FROM releases r2 WHERE r2.system_id = s.id AND r2.is_preferred = 1
		)
		GROUP BY r.id
		ORDER BY s.name, r.name
	`)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer func() { _ = rows.Close() }()

	type gameInfo struct {
		ID       int64  `json:"id"`
		Name     string `json:"name"`
		FilePath string `json:"filePath"`
		Size     int64  `json:"size"`
	}
	systemGames := make(map[string][]gameInfo)
	systemNames := make(map[string]string)

	for rows.Next() {
		var systemID, systemName, gameName, filePath, entryName string
		var releaseID, size int64
		if err := rows.Scan(&systemID, &systemName, &releaseID, &gameName, &filePath, &entryName, &size); err != nil {
			continue
		}
		systemNames[systemID] = systemName
		systemGames[systemID] = append(systemGames[systemID], gameInfo{
			ID:       releaseID,
			Name:     gameName,
			FilePath: filePath,
			Size:     size,
		})
	}

	// Build response
	type systemInfo struct {
		ID    string     `json:"id"`
		Name  string     `json:"name"`
		Games []gameInfo `json:"games"`
	}
	systems := make([]systemInfo, 0, len(systemGames))
	for sysID, games := range systemGames {
		systems = append(systems, systemInfo{
			ID:    sysID,
			Name:  systemNames[sysID],
			Games: games,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"systems": systems})
}

// PackGenerateRequest is the request body for pack generation.
type PackGenerateRequest struct {
	GameIDs []int64 `json:"gameIds"`
	Format  string  `json:"format"`
	Name    string  `json:"name"`
}

// handlePackGenerate streams a zip file containing the requested games.
func (s *Server) handlePackGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PackGenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if len(req.GameIDs) == 0 {
		http.Error(w, "No games specified", http.StatusBadRequest)
		return
	}

	// Lookup game details from database
	games := make([]pack.Game, 0, len(req.GameIDs))
	for _, id := range req.GameIDs {
		var game pack.Game
		err := s.db.QueryRowContext(r.Context(), `
			SELECT 
				r.id, r.name, s.name, COALESCE(s.dat_name, s.name),
				sf.path, COALESCE(re.name, 'rom.bin'), sf.size
			FROM releases r
			JOIN systems s ON s.id = r.system_id
			JOIN rom_entries re ON re.release_id = r.id
			JOIN matches m ON m.rom_entry_id = re.id
			JOIN scanned_files sf ON m.scanned_file_id = sf.id
			WHERE r.id = ?
			LIMIT 1
		`, id).Scan(&game.ID, &game.Name, &game.System, &game.SystemName,
			&game.FilePath, &game.FileName, &game.Size)
		if err != nil {
			continue // Skip games not found
		}
		games = append(games, game)
	}

	if len(games) == 0 {
		http.Error(w, "No valid games found", http.StatusBadRequest)
		return
	}

	// Determine format
	format := pack.FormatSimple
	switch req.Format {
	case "retroarch":
		format = pack.FormatRetroArch
	case "emulationstation":
		format = pack.FormatEmulationStation
	case "simple":
		format = pack.FormatSimple
	case "arkos":
		format = pack.FormatArkOS
	}

	// Set headers for zip download
	packName := req.Name
	if packName == "" {
		packName = "gamepack"
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.zip\"", packName))

	// Stream zip directly to response
	generator := pack.NewGenerator()
	_, err := generator.Generate(pack.Request{
		Name:   packName,
		Format: format,
		Games:  games,
	}, w)

	if err != nil {
		// Can't send error to client since we've already started writing
		log.Printf("Error generating pack: %v", err)
	}
}
