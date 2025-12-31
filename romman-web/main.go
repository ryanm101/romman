package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/ryanm101/romman-lib/config"
	"github.com/ryanm101/romman-lib/db"
	"github.com/ryanm101/romman-lib/library"
	"github.com/ryanm101/romman-lib/metrics"
)

//go:embed assets/*
var assets embed.FS

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Printf("Warning: failed to load config: %v", err)
		cfg = config.DefaultConfig()
	}

	database, err := db.Open(cfg.GetDBPath())
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer func() { _ = database.Close() }()

	server := NewServer(database.Conn())

	port := os.Getenv("ROMMAN_PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("🌐 ROM Manager Web UI\n")
	fmt.Printf("   http://localhost:%s\n\n", port)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      server,
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
	db  *sql.DB
	mux *http.ServeMux
}

// NewServer creates a new web server.
func NewServer(conn *sql.DB) *Server {
	s := &Server{
		db:  conn,
		mux: http.NewServeMux(),
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
	s.mux.HandleFunc("/api/details", s.handleDetails)
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/metrics", s.handleMetrics)
	s.mux.HandleFunc("/", s.handleDashboard)
}

func (s *Server) handleStats(w http.ResponseWriter, _ *http.Request) {
	var systems, libraries, releases int

	_ = s.db.QueryRow("SELECT COUNT(*) FROM systems").Scan(&systems)
	_ = s.db.QueryRow("SELECT COUNT(*) FROM libraries").Scan(&libraries)
	_ = s.db.QueryRow("SELECT COUNT(*) FROM releases").Scan(&releases)

	data := map[string]int{
		"totalSystems":   systems,
		"totalLibraries": libraries,
		"totalReleases":  releases,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

func (s *Server) handleSystems(w http.ResponseWriter, _ *http.Request) {
	rows, err := s.db.Query(`
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

func (s *Server) handleLibraries(w http.ResponseWriter, _ *http.Request) {
	rows, err := s.db.Query(`
		SELECT l.name, s.name as system,
			COUNT(DISTINCT CASE WHEN m.id IS NOT NULL THEN re.release_id END) as matched,
			COUNT(DISTINCT r.id) as total
		FROM libraries l
		JOIN systems s ON s.id = l.system_id
		LEFT JOIN releases r ON r.system_id = l.system_id
		LEFT JOIN rom_entries re ON re.release_id = r.id
		LEFT JOIN matches m ON m.rom_entry_id = re.id
			AND m.scanned_file_id IN (SELECT id FROM scanned_files WHERE library_id = l.id)
		GROUP BY l.id ORDER BY l.name
	`)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer func() { _ = rows.Close() }()

	var libs []map[string]interface{}
	for rows.Next() {
		var name, system string
		var matched, total int
		if err := rows.Scan(&name, &system, &matched, &total); err != nil {
			continue
		}
		pct := 0
		if total > 0 {
			pct = matched * 100 / total
		}
		libs = append(libs, map[string]interface{}{
			"name":     name,
			"system":   system,
			"matched":  matched,
			"total":    total,
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
		rows, err := s.db.Query(`
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
				var name, path, matchType, flags string
				_ = rows.Scan(&name, &path, &matchType, &flags)
				items = append(items, map[string]string{"name": name, "path": path, "matchType": matchType, "flags": flags, "status": "matched"})
			}
		}
	case "missing":
		rows, err := s.db.Query(`
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
		rows, err := s.db.Query(`
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
		rows, err := s.db.Query(`
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
		rows, err := s.db.Query(`
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
