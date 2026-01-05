package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ryanm101/romman-lib/library"
)

func handleCleanupCommand(ctx context.Context, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: romman cleanup <command>")
		os.Exit(1)
	}

	switch args[0] {
	case "plan":
		if len(args) < 3 {
			fmt.Println("Usage: romman cleanup plan <library> <quarantine-dir>")
			os.Exit(1)
		}
		generateCleanupPlan(ctx, args[1], args[2])
	case "exec":
		if len(args) < 2 {
			fmt.Println("Usage: romman cleanup exec <plan-file> [--dry-run]")
			os.Exit(1)
		}
		dryRun := len(args) > 2 && args[2] == "--dry-run"
		executeCleanupPlan(ctx, args[1], dryRun)
	default:
		fmt.Printf("Unknown cleanup command: %s\n", args[0])
		os.Exit(1)
	}
}

func generateCleanupPlan(ctx context.Context, libraryName, quarantineDir string) {
	database, err := openDB(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	manager := library.NewManager(database.Conn())
	finder := library.NewDuplicateFinder(database.Conn())
	planner := library.NewCleanupPlanner(finder, manager)

	absQuarantine, err := filepath.Abs(quarantineDir)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error resolving path: %v\n", err)
		os.Exit(1)
	}

	plan, err := planner.GeneratePlan(context.Background(), libraryName, absQuarantine)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error generating plan: %v\n", err)
		os.Exit(1)
	}

	planFile := fmt.Sprintf("cleanup-%s-%s.json", libraryName, plan.CreatedAt.Format("20060102-150405"))
	if err := library.SavePlan(plan, planFile); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error saving plan: %v\n", err)
		os.Exit(1)
	}

	if outputCfg.JSON {
		PrintResult(plan)
		return
	}

	fmt.Printf("Cleanup plan generated: %s\n\n", planFile)
	fmt.Printf("Library: %s\n", plan.LibraryName)
	fmt.Printf("Quarantine: %s\n\n", plan.QuarantineDir)
	fmt.Printf("Summary:\n")
	fmt.Printf("  Total actions: %d\n", plan.Summary.TotalActions)
	fmt.Printf("  Keep (ignore): %d\n", plan.Summary.IgnoreCount)
	fmt.Printf("  Move to quarantine: %d\n", plan.Summary.MoveCount)
	fmt.Printf("  Space to reclaim: %.2f MB\n", float64(plan.Summary.SpaceReclaimed)/1024/1024)
	fmt.Println()
	fmt.Printf("To execute: romman cleanup exec %s [--dry-run]\n", planFile)
}

func executeCleanupPlan(ctx context.Context, planFile string, dryRun bool) {
	_ = ctx // May be used for operations in future
	plan, err := library.LoadPlan(planFile)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error loading plan: %v\n", err)
		os.Exit(1)
	}

	mode := "LIVE"
	if dryRun {
		mode = "DRY-RUN"
	}

	fmt.Printf("Executing cleanup plan (%s): %s\n\n", mode, planFile)
	fmt.Printf("Library: %s\n", plan.LibraryName)
	fmt.Printf("Actions: %d\n\n", plan.Summary.TotalActions)

	if !dryRun && !outputCfg.JSON && !outputCfg.Quiet {
		fmt.Print("This will move files to quarantine. Continue? [y/N] ")
		var response string
		_, _ = fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Aborted.")
			return
		}
	}

	result, err := library.ExecutePlan(plan, dryRun)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error executing plan: %v\n", err)
		os.Exit(1)
	}

	if outputCfg.JSON {
		PrintResult(result)
		return
	}

	fmt.Printf("\nResults:\n")
	fmt.Printf("  Succeeded: %d\n", result.Succeeded)
	fmt.Printf("  Failed: %d\n", result.Failed)

	if len(result.Errors) > 0 {
		fmt.Println("\nErrors:")
		for _, e := range result.Errors {
			fmt.Printf("  %s: %s\n", e.Action.SourcePath, e.Error)
		}
	}

	if dryRun {
		fmt.Println("\n(Dry run - no files were modified)")
	}
}
