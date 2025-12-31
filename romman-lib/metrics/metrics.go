package metrics

import (
	"database/sql"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Database Gauges
	SystemsTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "romman_systems_total",
		Help: "Total number of systems in the database.",
	})
	LibrariesTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "romman_libraries_total",
		Help: "Total number of configured libraries.",
	})
	ReleasesTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "romman_releases_total",
		Help: "Total number of releases in the database.",
	})
	RomsTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "romman_roms_total",
		Help: "Total number of ROM entries.",
	})
	MatchesTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "romman_matches_total",
		Help: "Total number of matched files.",
	})

	// Scan Performance
	ScanDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "romman_scan_duration_seconds",
		Help:    "Duration of library scans in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"library"})

	FilesProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "romman_files_processed_total",
		Help: "Total number of files processed during scans.",
	}, []string{"library", "status"}) // status: scanned, hashed, matched, skipped
)

// UpdateDBMetrics refreshes gauges that reflect the current state of the database.
func UpdateDBMetrics(db *sql.DB) error {
	var systems, libraries, releases, roms, matches int

	if err := db.QueryRow("SELECT COUNT(*) FROM systems").Scan(&systems); err != nil {
		return err
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM libraries").Scan(&libraries); err != nil {
		return err
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM releases").Scan(&releases); err != nil {
		return err
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM rom_entries").Scan(&roms); err != nil {
		return err
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM matches").Scan(&matches); err != nil {
		return err
	}

	SystemsTotal.Set(float64(systems))
	LibrariesTotal.Set(float64(libraries))
	ReleasesTotal.Set(float64(releases))
	RomsTotal.Set(float64(roms))
	MatchesTotal.Set(float64(matches))

	return nil
}

// RecordScanDuration records the time taken for a library scan.
func RecordScanDuration(library string, start time.Time) {
	ScanDuration.WithLabelValues(library).Observe(time.Since(start).Seconds())
}
