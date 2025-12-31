package library

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSavePlan(t *testing.T) {
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plan.json")

	plan := &CleanupPlan{
		LibraryName: "test-lib",
		LibraryPath: "/roms",
		SystemName:  "nes",
		Actions: []CleanupAction{
			{Action: ActionMove, SourcePath: "/roms/a.rom", DestPath: "/quarantine/a.rom"},
			{Action: ActionIgnore, SourcePath: "/roms/b.rom", Reason: "preferred"},
		},
		Summary: PlanSummary{
			TotalActions: 2,
			MoveCount:    1,
			IgnoreCount:  1,
		},
	}

	err := SavePlan(plan, planPath)
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(planPath)
	assert.NoError(t, err)
}

func TestLoadPlan(t *testing.T) {
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plan.json")

	// Create a plan file
	plan := &CleanupPlan{
		LibraryName: "test-lib",
		SystemName:  "nes",
		Actions: []CleanupAction{
			{Action: ActionMove, SourcePath: "/a.rom"},
		},
	}
	err := SavePlan(plan, planPath)
	require.NoError(t, err)

	// Load it back
	loaded, err := LoadPlan(planPath)
	require.NoError(t, err)

	assert.Equal(t, "test-lib", loaded.LibraryName)
	assert.Equal(t, "nes", loaded.SystemName)
	assert.Len(t, loaded.Actions, 1)
}

func TestLoadPlan_NotFound(t *testing.T) {
	_, err := LoadPlan("/nonexistent/plan.json")
	assert.Error(t, err)
}

func TestLoadPlan_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "bad.json")

	err := os.WriteFile(planPath, []byte("not json"), 0644) // #nosec G306
	require.NoError(t, err)

	_, err = LoadPlan(planPath)
	assert.Error(t, err)
}

func TestExecutePlan_DryRun(t *testing.T) {
	plan := &CleanupPlan{
		Actions: []CleanupAction{
			{Action: ActionIgnore, SourcePath: "/a.rom"},
			{Action: ActionMove, SourcePath: "/b.rom", DestPath: "/quarantine/b.rom"},
			{Action: ActionDelete, SourcePath: "/c.rom"},
		},
	}

	result, err := ExecutePlan(plan, true)
	require.NoError(t, err)

	assert.True(t, result.DryRun)
	assert.Equal(t, 3, result.Succeeded)
	assert.Equal(t, 0, result.Failed)
}

func TestExecutePlan_ActionIgnoreAlwaysSucceeds(t *testing.T) {
	plan := &CleanupPlan{
		Actions: []CleanupAction{
			{Action: ActionIgnore, SourcePath: "/nonexistent.rom"},
		},
	}

	result, err := ExecutePlan(plan, false)
	require.NoError(t, err)

	assert.Equal(t, 1, result.Succeeded)
}

func TestActionTypes(t *testing.T) {
	assert.Equal(t, ActionType("delete"), ActionDelete)
	assert.Equal(t, ActionType("move"), ActionMove)
	assert.Equal(t, ActionType("ignore"), ActionIgnore)
}
