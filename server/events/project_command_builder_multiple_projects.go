package events

import (
	"fmt"
	"strings"

	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/utils"
)

// buildMultipleProjectPlanCommands builds plan commands for multiple projects, dirs, and/or workspaces.
// It uses a unified filtering approach that works for any combination of filters.
func (p *DefaultProjectCommandBuilder) buildMultipleProjectPlanCommands(ctx *command.Context, cmd *CommentCommand) ([]command.ProjectContext, error) {
	ctx.Log.Info("Building plan commands with filters: projects=%v, dirs=%v, workspaces=%v",
		cmd.ProjectNames, cmd.RepoRelDirs, cmd.Workspaces)

	// Get all available project contexts
	allContexts, err := p.buildAllCommandsByCfg(ctx, cmd.CommandName(), cmd.SubName, cmd.Flags, cmd.Verbose)
	if err != nil {
		return nil, fmt.Errorf("error building project contexts: %w", err)
	}

	ctx.Log.Info("Found %d total project contexts, applying filters", len(allContexts))

	// Apply unified filtering
	filteredContexts, err := filterProjectContexts(allContexts, cmd.ProjectNames, cmd.RepoRelDirs, cmd.Workspaces)
	if err != nil {
		// Enhance error with suggestions
		return nil, enhanceFilterError(err, allContexts, cmd.ProjectNames, cmd.RepoRelDirs, cmd.Workspaces)
	}

	ctx.Log.Info("Successfully built commands for %d filtered projects", len(filteredContexts))
	return filteredContexts, nil
}

// buildMultipleProjectCommands builds commands for multiple projects, dirs, and/or workspaces.
// This is used for apply, approve_policies, version, import, and state commands.
func (p *DefaultProjectCommandBuilder) buildMultipleProjectCommands(ctx *command.Context, cmd *CommentCommand) ([]command.ProjectContext, error) {
	ctx.Log.Info("Building commands with filters: projects=%v, dirs=%v, workspaces=%v",
		cmd.ProjectNames, cmd.RepoRelDirs, cmd.Workspaces)

	// Get all available contexts - method depends on command type
	var allContexts []command.ProjectContext
	var err error

	switch cmd.Name {
	case command.Apply, command.ApprovePolicies, command.Version:
		// These commands operate on existing plans
		allContexts, err = p.buildAllProjectCommandsByPlan(ctx, cmd)
	case command.Import, command.State:
		// These commands don't need existing plans
		allContexts, err = p.buildAllCommandsByCfg(ctx, cmd.CommandName(), cmd.SubName, cmd.Flags, cmd.Verbose)
	default:
		return nil, fmt.Errorf("unsupported command: %s", cmd.Name.String())
	}

	if err != nil {
		return nil, fmt.Errorf("error building project contexts: %w", err)
	}

	ctx.Log.Info("Found %d total project contexts, applying filters", len(allContexts))

	// Apply unified filtering
	filteredContexts, err := filterProjectContexts(allContexts, cmd.ProjectNames, cmd.RepoRelDirs, cmd.Workspaces)
	if err != nil {
		// Enhance error with suggestions
		return nil, enhanceFilterError(err, allContexts, cmd.ProjectNames, cmd.RepoRelDirs, cmd.Workspaces)
	}

	ctx.Log.Info("Successfully built commands for %d filtered projects", len(filteredContexts))
	return filteredContexts, nil
}

// buildMultipleDirWorkspacePlanCommands is an alias for buildMultipleProjectPlanCommands
// for backward compatibility with routing logic.
func (p *DefaultProjectCommandBuilder) buildMultipleDirWorkspacePlanCommands(ctx *command.Context, cmd *CommentCommand) ([]command.ProjectContext, error) {
	return p.buildMultipleProjectPlanCommands(ctx, cmd)
}

// buildMultipleDirWorkspaceCommands is an alias for buildMultipleProjectCommands
// for backward compatibility with routing logic.
func (p *DefaultProjectCommandBuilder) buildMultipleDirWorkspaceCommands(ctx *command.Context, cmd *CommentCommand) ([]command.ProjectContext, error) {
	return p.buildMultipleProjectCommands(ctx, cmd)
}

// buildMultipleDirWorkspaceCommandsByCfg is an alias for buildMultipleProjectCommands
// for backward compatibility with routing logic.
func (p *DefaultProjectCommandBuilder) buildMultipleDirWorkspaceCommandsByCfg(ctx *command.Context, cmd *CommentCommand) ([]command.ProjectContext, error) {
	return p.buildMultipleProjectCommands(ctx, cmd)
}

// filterProjectContexts applies all filters (projects, dirs, workspaces) to the contexts.
// Returns filtered contexts or an error if no matches found.
func filterProjectContexts(contexts []command.ProjectContext, projects, dirs, workspaces []string) ([]command.ProjectContext, error) {
	if len(contexts) == 0 {
		return nil, fmt.Errorf("no project contexts available to filter")
	}

	// If no filters specified, return all contexts
	if len(projects) == 0 && len(dirs) == 0 && len(workspaces) == 0 {
		return contexts, nil
	}

	var filtered []command.ProjectContext

	// Create filter maps for efficient lookup
	projectMap := makeStringMap(projects)
	dirMap := makeStringMap(dirs)
	workspaceMap := makeStringMap(workspaces)

	for _, ctx := range contexts {
		matchesProject := len(projects) == 0 || projectMap[ctx.ProjectName]
		matchesDir := len(dirs) == 0 || dirMap[ctx.RepoRelDir]
		matchesWorkspace := len(workspaces) == 0 || workspaceMap[ctx.Workspace]

		// Context must match ALL specified filters
		if matchesProject && matchesDir && matchesWorkspace {
			filtered = append(filtered, ctx)
		}
	}

	if len(filtered) == 0 {
		return nil, fmt.Errorf("no projects matched the specified filters")
	}

	return filtered, nil
}

// enhanceFilterError adds helpful information to filter errors.
func enhanceFilterError(err error, allContexts []command.ProjectContext, projects, dirs, workspaces []string) error {
	var errorParts []string
	errorParts = append(errorParts, err.Error())

	// Add what was requested
	var requested []string
	if len(projects) > 0 {
		requested = append(requested, fmt.Sprintf("projects: %s", strings.Join(projects, ", ")))
	}
	if len(dirs) > 0 {
		requested = append(requested, fmt.Sprintf("directories: %s", strings.Join(dirs, ", ")))
	}
	if len(workspaces) > 0 {
		requested = append(requested, fmt.Sprintf("workspaces: %s", strings.Join(workspaces, ", ")))
	}
	if len(requested) > 0 {
		errorParts = append(errorParts, fmt.Sprintf("\nRequested filters:\n  %s", strings.Join(requested, "\n  ")))
	}

	// Add fuzzy matching suggestions for projects
	if len(projects) > 0 {
		availableProjects := getProjectNamesFromContexts(allContexts)
		suggestions := findSimilarProjects(projects, availableProjects)
		if len(suggestions) > 0 {
			errorParts = append(errorParts, fmt.Sprintf("\nDid you mean?\n  %s", strings.Join(suggestions, "\n  ")))
		}
		errorParts = append(errorParts, fmt.Sprintf("\nAvailable projects: %s", strings.Join(availableProjects, ", ")))
	}

	// Add available dirs if filtering by dirs
	if len(dirs) > 0 {
		availableDirs := getUniqueDirsFromContexts(allContexts)
		errorParts = append(errorParts, fmt.Sprintf("\nAvailable directories: %s", strings.Join(availableDirs, ", ")))
	}

	// Add available workspaces if filtering by workspaces
	if len(workspaces) > 0 {
		availableWorkspaces := getUniqueWorkspacesFromContexts(allContexts)
		errorParts = append(errorParts, fmt.Sprintf("\nAvailable workspaces: %s", strings.Join(availableWorkspaces, ", ")))
	}

	return fmt.Errorf("%s", strings.Join(errorParts, ""))
}

// makeStringMap creates a map from a string slice for O(1) lookup.
func makeStringMap(items []string) map[string]bool {
	m := make(map[string]bool)
	for _, item := range items {
		m[item] = true
	}
	return m
}

// getProjectNamesFromContexts extracts unique project names from contexts.
func getProjectNamesFromContexts(contexts []command.ProjectContext) []string {
	names := make([]string, 0)
	seen := make(map[string]bool)

	for _, ctx := range contexts {
		if ctx.ProjectName != "" && !seen[ctx.ProjectName] {
			names = append(names, ctx.ProjectName)
			seen[ctx.ProjectName] = true
		}
	}

	return names
}

// getUniqueDirsFromContexts extracts unique directories from contexts.
func getUniqueDirsFromContexts(contexts []command.ProjectContext) []string {
	dirs := make([]string, 0)
	seen := make(map[string]bool)

	for _, ctx := range contexts {
		if !seen[ctx.RepoRelDir] {
			dirs = append(dirs, ctx.RepoRelDir)
			seen[ctx.RepoRelDir] = true
		}
	}

	return dirs
}

// getUniqueWorkspacesFromContexts extracts unique workspaces from contexts.
func getUniqueWorkspacesFromContexts(contexts []command.ProjectContext) []string {
	workspaces := make([]string, 0)
	seen := make(map[string]bool)

	for _, ctx := range contexts {
		if !seen[ctx.Workspace] {
			workspaces = append(workspaces, ctx.Workspace)
			seen[ctx.Workspace] = true
		}
	}

	return workspaces
}

// findSimilarProjects finds project names that are similar to the requested ones.
func findSimilarProjects(requested []string, available []string) []string {
	var suggestions []string
	seen := make(map[string]bool)

	for _, req := range requested {
		for _, avail := range available {
			if !seen[avail] && isSimilar(req, avail) {
				suggestions = append(suggestions, fmt.Sprintf("%s -> %s", req, avail))
				seen[avail] = true
			}
		}
	}

	return suggestions
}

// isSimilar checks if two strings are similar using Levenshtein distance.
func isSimilar(a, b string) bool {
	if a == b {
		return false
	}
	return utils.IsSimilarWord(a, b)
}

// getProjectNameFromCommand extracts the project name from a CommentCommand.
// Returns empty string if no project is specified, or the single project name if exactly one is specified.
func getProjectNameFromCommand(cmd *CommentCommand) string {
	if cmd.ProjectName != "" {
		return cmd.ProjectName
	}
	if len(cmd.ProjectNames) == 1 {
		return cmd.ProjectNames[0]
	}
	return ""
}
