package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestRecordScanDuration(t *testing.T) {
	start := time.Now().Add(-100 * time.Millisecond)

	RecordScanDuration("test-lib", start)

	// Verify histogram was updated - just ensure no panic
	// The histogram is recorded successfully if we get here
}

func TestFilesProcessed_Counter(t *testing.T) {
	// Record some file processing
	FilesProcessed.WithLabelValues("test-lib", "scanned").Inc()
	FilesProcessed.WithLabelValues("test-lib", "hashed").Inc()
	FilesProcessed.WithLabelValues("test-lib", "skipped").Inc()

	// Verify counters exist and are accessible
	scanned := testutil.ToFloat64(FilesProcessed.WithLabelValues("test-lib", "scanned"))
	assert.GreaterOrEqual(t, scanned, float64(1))

	hashed := testutil.ToFloat64(FilesProcessed.WithLabelValues("test-lib", "hashed"))
	assert.GreaterOrEqual(t, hashed, float64(1))

	skipped := testutil.ToFloat64(FilesProcessed.WithLabelValues("test-lib", "skipped"))
	assert.GreaterOrEqual(t, skipped, float64(1))
}

func TestGauges_Exist(t *testing.T) {
	// Verify all gauges are defined and accessible
	SystemsTotal.Set(10)
	assert.Equal(t, float64(10), testutil.ToFloat64(SystemsTotal))

	LibrariesTotal.Set(5)
	assert.Equal(t, float64(5), testutil.ToFloat64(LibrariesTotal))

	ReleasesTotal.Set(1000)
	assert.Equal(t, float64(1000), testutil.ToFloat64(ReleasesTotal))

	RomsTotal.Set(2000)
	assert.Equal(t, float64(2000), testutil.ToFloat64(RomsTotal))

	MatchesTotal.Set(500)
	assert.Equal(t, float64(500), testutil.ToFloat64(MatchesTotal))
}
