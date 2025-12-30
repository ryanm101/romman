package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/ryanm/romman-lib/config"
	"github.com/ryanm/romman-lib/db"
)

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

	if err := http.ListenAndServe(":"+port, server); err != nil {
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

func (s *Server) handleDashboard(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(dashboardHTML))
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>ROM Manager</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: #0d1117;
            color: #c9d1d9;
            min-height: 100vh;
        }
        .container { max-width: 1200px; margin: 0 auto; padding: 2rem; }
        header {
            display: flex; align-items: center; gap: 1rem;
            margin-bottom: 2rem; padding-bottom: 1rem;
            border-bottom: 1px solid #30363d;
        }
        header h1 { font-size: 1.5rem; }
        .stats {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1rem; margin-bottom: 2rem;
        }
        .stat-card {
            background: #161b22; border: 1px solid #30363d;
            border-radius: 8px; padding: 1.5rem;
        }
        .stat-card h3 { color: #8b949e; font-size: 0.875rem; margin-bottom: 0.5rem; }
        .stat-card .value { font-size: 2rem; font-weight: bold; color: #58a6ff; }
        .section { margin-bottom: 2rem; }
        .section h2 { margin-bottom: 1rem; font-size: 1.25rem; }
        table { width: 100%; border-collapse: collapse; }
        th, td { text-align: left; padding: 0.75rem; border-bottom: 1px solid #30363d; }
        th { color: #8b949e; font-weight: 500; }
        .progress { background: #30363d; border-radius: 4px; height: 8px; overflow: hidden; }
        .progress-bar { background: #58a6ff; height: 100%; }
    </style>
</head>
<body>
    <div class="container">
        <header><span style="font-size:2rem">🎮</span><h1>ROM Manager</h1></header>
        <div class="stats">
            <div class="stat-card"><h3>Systems</h3><div class="value" id="stat-systems">-</div></div>
            <div class="stat-card"><h3>Libraries</h3><div class="value" id="stat-libraries">-</div></div>
            <div class="stat-card"><h3>Releases</h3><div class="value" id="stat-releases">-</div></div>
        </div>
        <div class="section">
            <h2>Systems</h2>
            <table><thead><tr><th>Name</th><th>Releases</th><th>Preferred</th></tr></thead>
            <tbody id="systems-body"><tr><td colspan="3">Loading...</td></tr></tbody></table>
        </div>
        <div class="section">
            <h2>Libraries</h2>
            <table><thead><tr><th>Name</th><th>System</th><th>Progress</th></tr></thead>
            <tbody id="libs-body"><tr><td colspan="3">Loading...</td></tr></tbody></table>
        </div>
    </div>
    <script>
        fetch('/api/stats').then(r=>r.json()).then(d=>{
            document.getElementById('stat-systems').textContent=d.totalSystems;
            document.getElementById('stat-libraries').textContent=d.totalLibraries;
            document.getElementById('stat-releases').textContent=d.totalReleases;
        });
        fetch('/api/systems').then(r=>r.json()).then(d=>{
            const t=document.getElementById('systems-body');
            t.innerHTML=(d.systems||[]).map(s=>'<tr><td>'+s.name+'</td><td>'+s.releases+'</td><td>'+s.preferred+'</td></tr>').join('')||'<tr><td colspan="3">No systems</td></tr>';
        });
        fetch('/api/libraries').then(r=>r.json()).then(d=>{
            const t=document.getElementById('libs-body');
            t.innerHTML=(d.libraries||[]).map(l=>'<tr><td>'+l.name+'</td><td>'+l.system+'</td><td><div class="progress"><div class="progress-bar" style="width:'+l.matchPct+'%"></div></div>'+l.matchPct+'%</td></tr>').join('')||'<tr><td colspan="3">No libraries</td></tr>';
        });
    </script>
</body>
</html>`
