package library

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ryanm101/romman-lib/tracing"
	"go.opentelemetry.io/otel/attribute"
)

// ActionType represents what to do with a file.
type ActionType string

const (
	ActionDelete ActionType = "delete"
	ActionMove   ActionType = "move"
	ActionIgnore ActionType = "ignore"
)

// CleanupAction represents a single file operation in a cleanup plan.
type CleanupAction struct {
	Action     ActionType `json:"action"`
	SourcePath string     `json:"source_path"`
	DestPath   string     `json:"dest_path,omitempty"` // For move actions
	Reason     string     `json:"reason"`
	FileID     int64      `json:"file_id"`
	DupType    string     `json:"duplicate_type"`
	MatchType  string     `json:"match_type,omitempty"`
	Flags      string     `json:"flags,omitempty"`
}

// CleanupPlan is a set of actions to clean up a library.
type CleanupPlan struct {
	LibraryName   string          `json:"library_name"`
	LibraryPath   string          `json:"library_path"`
	SystemName    string          `json:"system_name"`
	CreatedAt     time.Time       `json:"created_at"`
	QuarantineDir string          `json:"quarantine_dir"`
	Actions       []CleanupAction `json:"actions"`
	Summary       PlanSummary     `json:"summary"`
}

// PlanSummary summarizes the plan.
type PlanSummary struct {
	TotalActions   int   `json:"total_actions"`
	DeleteCount    int   `json:"delete_count"`
	MoveCount      int   `json:"move_count"`
	IgnoreCount    int   `json:"ignore_count"`
	SpaceReclaimed int64 `json:"space_reclaimed_bytes"`
}

// ExecutionResult is the result of executing a cleanup plan.
type ExecutionResult struct {
	Plan       *CleanupPlan  `json:"plan"`
	ExecutedAt time.Time     `json:"executed_at"`
	DryRun     bool          `json:"dry_run"`
	Succeeded  int           `json:"succeeded"`
	Failed     int           `json:"failed"`
	Errors     []ActionError `json:"errors,omitempty"`
}

// ActionError records a failed action.
type ActionError struct {
	Action CleanupAction `json:"action"`
	Error  string        `json:"error"`
}

// CleanupPlanner generates cleanup plans.
type CleanupPlanner struct {
	finder  *DuplicateFinder
	manager *Manager
}

// NewCleanupPlanner creates a new planner.
func NewCleanupPlanner(finder *DuplicateFinder, manager *Manager) *CleanupPlanner {
	return &CleanupPlanner{
		finder:  finder,
		manager: manager,
	}
}

// GeneratePlan creates a cleanup plan for a library's duplicates.
func (p *CleanupPlanner) GeneratePlan(ctx context.Context, libraryName string, quarantineBase string) (*CleanupPlan, error) {
	ctx, span := tracing.StartSpan(ctx, "library.CleanupPlan",
		tracing.WithAttributes(
			attribute.String("library.name", libraryName),
			attribute.String("quarantine.base", quarantineBase),
		),
	)
	defer span.End()

	lib, err := p.manager.Get(libraryName)
	if err != nil {
		tracing.RecordError(span, err)
		return nil, err
	}

	duplicates, err := p.finder.FindAllDuplicates(ctx, lib.ID)
	if err != nil {
		tracing.RecordError(span, err)
		return nil, err
	}

	// Build quarantine path for this system
	quarantineDir := filepath.Join(quarantineBase, lib.SystemName)

	plan := &CleanupPlan{
		LibraryName:   lib.Name,
		LibraryPath:   lib.RootPath,
		SystemName:    lib.SystemName,
		CreatedAt:     time.Now(),
		QuarantineDir: quarantineDir,
	}

	// Track files we've already added to avoid duplicates
	// A file may appear in multiple duplicate groups (exact, variant, package)
	seenFiles := make(map[string]*CleanupAction)

	for _, dup := range duplicates {
		for _, file := range dup.Files {
			// Check if we've already seen this file
			if existing, ok := seenFiles[file.Path]; ok {
				// If we already have this file as ignore (keep), don't change it
				// If we already have it as move but now it's preferred, upgrade to ignore
				if file.IsPreferred && existing.Action == ActionMove {
					existing.Action = ActionIgnore
					existing.Reason = "preferred copy"
					existing.DestPath = ""
					plan.Summary.MoveCount--
					plan.Summary.IgnoreCount++
				}
				continue
			}

			action := CleanupAction{
				FileID:     file.ScannedFileID,
				SourcePath: file.Path,
				DupType:    string(dup.Type),
				MatchType:  file.MatchType,
				Flags:      file.Flags,
			}

			if file.IsPreferred {
				// Keep preferred files
				action.Action = ActionIgnore
				action.Reason = "preferred copy"
				plan.Summary.IgnoreCount++
			} else {
				// Move non-preferred to quarantine
				action.Action = ActionMove
				relPath, _ := filepath.Rel(lib.RootPath, file.Path)
				action.DestPath = filepath.Join(quarantineDir, relPath)
				action.Reason = fmt.Sprintf("duplicate of preferred (%s)", dup.Type)
				plan.Summary.MoveCount++
			}

			seenFiles[file.Path] = &action
			plan.Actions = append(plan.Actions, action)
		}
	}

	// Calculate space reclaimed from move actions
	var totalSpace int64
	for _, action := range plan.Actions {
		if action.Action == ActionMove {
			// Get file size from duplicates
			for _, dup := range duplicates {
				for _, file := range dup.Files {
					if file.Path == action.SourcePath {
						totalSpace += file.Size
						break
					}
				}
			}
		}
	}

	plan.Summary.TotalActions = len(plan.Actions)
	plan.Summary.SpaceReclaimed = totalSpace

	tracing.AddSpanAttributes(span,
		attribute.Int("result.total_actions", plan.Summary.TotalActions),
		attribute.Int("result.move_count", plan.Summary.MoveCount),
		attribute.Int64("result.space_reclaimed", plan.Summary.SpaceReclaimed),
	)

	return plan, nil
}

// SavePlan saves a plan to a JSON file.
func SavePlan(plan *CleanupPlan, path string) error {
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal plan: %w", err)
	}

	// #nosec G306
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write plan: %w", err)
	}

	return nil
}

// LoadPlan loads a plan from a JSON file.
func LoadPlan(path string) (*CleanupPlan, error) {
	data, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		return nil, fmt.Errorf("failed to read plan: %w", err)
	}

	var plan CleanupPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan: %w", err)
	}

	return &plan, nil
}

// ExecutePlan executes a cleanup plan.
func ExecutePlan(plan *CleanupPlan, dryRun bool) (*ExecutionResult, error) {
	result := &ExecutionResult{
		Plan:       plan,
		ExecutedAt: time.Now(),
		DryRun:     dryRun,
	}

	for _, action := range plan.Actions {
		if action.Action == ActionIgnore {
			result.Succeeded++
			continue
		}

		var err error
		switch action.Action {
		case ActionDelete:
			if !dryRun {
				err = os.Remove(action.SourcePath)
			}
		case ActionMove:
			if !dryRun {
				err = moveFile(action.SourcePath, action.DestPath)
			}
		}

		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, ActionError{
				Action: action,
				Error:  err.Error(),
			})
		} else {
			result.Succeeded++
		}
	}

	return result, nil
}

func moveFile(src, dst string) error {
	// Ensure destination directory exists
	// #nosec G301
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Try rename first (fast, same filesystem)
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// Fall back to copy + delete for cross-filesystem moves
	srcFile, err := os.Open(src) // #nosec G304
	if err != nil {
		return fmt.Errorf("failed to open source: %w", err)
	}
	defer func() { _ = srcFile.Close() }()

	dstFile, err := os.Create(dst) // #nosec G304
	if err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}
	defer func() { _ = dstFile.Close() }()

	if _, err := srcFile.WriteTo(dstFile); err != nil {
		return fmt.Errorf("failed to copy: %w", err)
	}

	// Sync to ensure data is written
	if err := dstFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync: %w", err)
	}

	// Remove source
	if err := os.Remove(src); err != nil {
		return fmt.Errorf("failed to remove source: %w", err)
	}

	return nil
}
