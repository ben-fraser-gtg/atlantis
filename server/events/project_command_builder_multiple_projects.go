package events

import (
	"fmt"
	"strings"

	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/utils"
)

// buildMultipleProjectPlanCommands builds plan commands for multiple projects by filtering
// from all available projects in the repo config. This leverages the existing project
// discovery and configuration logic instead of trying to build each project individually.
func (p *DefaultProjectCommandBuilder) buildMultipleProjectPlanCommands(ctx *command.Context, cmd *CommentCommand) ([]command.ProjectContext, error) {
	if len(cmd.ProjectNames) == 0 {
		return nil, fmt.Errorf("no projects specified")
	}

	ctx.Log.Info("Building plan commands for specified projects: %v", cmd.ProjectNames)

	// Use the existing buildAllCommandsByCfg logic to get all project contexts,
	// then filter to only the ones the user specified
	allContexts, err := p.buildAllCommandsByCfg(ctx, cmd.CommandName(), cmd.SubName, cmd.Flags, cmd.Verbose)
	if err != nil {
		return nil, fmt.Errorf("error building project contexts: %w", err)
	}

	ctx.Log.Info("Found %d total project contexts, filtering to requested projects", len(allContexts))

	// Filter to only the projects that were explicitly requested
	filteredContexts := filterProjectContextsByNames(allContexts, cmd.ProjectNames)

	if len(filteredContexts) == 0 {
		// Check if any of the requested projects exist in all contexts
		availableProjects := getProjectNamesFromContexts(allContexts)

		// Provide helpful suggestions for similar project names
		suggestions := findSimilarProjects(cmd.ProjectNames, availableProjects)
		if len(suggestions) > 0 {
			return nil, fmt.Errorf("none of the specified projects were found. Requested: %s.\n\nDid you mean one of these?\n  %s\n\nAll available projects: %s",
				strings.Join(cmd.ProjectNames, ", "),
				strings.Join(suggestions, "\n  "),
				strings.Join(availableProjects, ", "))
		}

		return nil, fmt.Errorf("none of the specified projects were found. Requested: %s. Available projects in config: %s",
			strings.Join(cmd.ProjectNames, ", "),
			strings.Join(availableProjects, ", "))
	}

	// Check if some requested projects were not found
	foundProjects := getProjectNamesFromContexts(filteredContexts)
	missingProjects := findMissingProjects(cmd.ProjectNames, foundProjects)
	if len(missingProjects) > 0 {
		ctx.Log.Warn("Some requested projects were not found: %s", strings.Join(missingProjects, ", "))
	}

	ctx.Log.Info("Successfully built commands for %d projects: %v", len(filteredContexts), foundProjects)
	return filteredContexts, nil
}

// buildMultipleProjectCommands builds commands for multiple projects by filtering from
// all available projects. This is used for apply, approve_policies, version, import, and state commands.
func (p *DefaultProjectCommandBuilder) buildMultipleProjectCommands(ctx *command.Context, cmd *CommentCommand) ([]command.ProjectContext, error) {
	if len(cmd.ProjectNames) == 0 {
		return nil, fmt.Errorf("no projects specified")
	}

	ctx.Log.Info("Building commands for specified projects: %v", cmd.ProjectNames)

	// For apply/approve_policies/version, use buildAllProjectCommandsByPlan which finds existing plans
	// For import/state, use buildAllCommandsByCfg since they don't need existing plans
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

	ctx.Log.Info("Found %d total project contexts, filtering to requested projects", len(allContexts))

	// Filter to only the projects that were explicitly requested
	filteredContexts := filterProjectContextsByNames(allContexts, cmd.ProjectNames)

	if len(filteredContexts) == 0 {
		// Check if any of the requested projects exist in all contexts
		availableProjects := getProjectNamesFromContexts(allContexts)

		// Provide helpful suggestions for similar project names
		suggestions := findSimilarProjects(cmd.ProjectNames, availableProjects)
		if len(suggestions) > 0 {
			return nil, fmt.Errorf("none of the specified projects were found. Requested: %s.\n\nDid you mean one of these?\n  %s\n\nAll available projects: %s",
				strings.Join(cmd.ProjectNames, ", "),
				strings.Join(suggestions, "\n  "),
				strings.Join(availableProjects, ", "))
		}

		return nil, fmt.Errorf("none of the specified projects were found. Requested: %s. Available projects: %s",
			strings.Join(cmd.ProjectNames, ", "),
			strings.Join(availableProjects, ", "))
	}

	// Check if some requested projects were not found
	foundProjects := getProjectNamesFromContexts(filteredContexts)
	missingProjects := findMissingProjects(cmd.ProjectNames, foundProjects)
	if len(missingProjects) > 0 {
		ctx.Log.Warn("Some requested projects were not found: %s", strings.Join(missingProjects, ", "))
	}

	ctx.Log.Info("Successfully built commands for %d projects: %v", len(filteredContexts), foundProjects)
	return filteredContexts, nil
}

// filterProjectContextsByNames filters a slice of project contexts to only include
// those whose ProjectName matches one of the specified names.
func filterProjectContextsByNames(contexts []command.ProjectContext, projectNames []string) []command.ProjectContext {
	var filtered []command.ProjectContext

	// Create a map for faster lookup
	nameMap := make(map[string]bool)
	for _, name := range projectNames {
		nameMap[name] = true
	}

	for _, ctx := range contexts {
		if nameMap[ctx.ProjectName] {
			filtered = append(filtered, ctx)
		}
	}

	return filtered
}

// getProjectNamesFromContexts extracts the project names from a slice of project contexts.
func getProjectNamesFromContexts(contexts []command.ProjectContext) []string {
	names := make([]string, 0, len(contexts))
	seen := make(map[string]bool)

	for _, ctx := range contexts {
		if ctx.ProjectName != "" && !seen[ctx.ProjectName] {
			names = append(names, ctx.ProjectName)
			seen[ctx.ProjectName] = true
		}
	}

	return names
}

// findMissingProjects returns the project names that were requested but not found.
func findMissingProjects(requested []string, found []string) []string {
	foundMap := make(map[string]bool)
	for _, name := range found {
		foundMap[name] = true
	}

	var missing []string
	for _, name := range requested {
		if !foundMap[name] {
			missing = append(missing, name)
		}
	}

	return missing
}

// findSimilarProjects finds project names that are similar to the requested ones.
// Uses Levenshtein distance to find typos and similar names.
func findSimilarProjects(requested []string, available []string) []string {
	var suggestions []string
	seen := make(map[string]bool)

	for _, req := range requested {
		for _, avail := range available {
			// Calculate similarity - use a simple approach
			if !seen[avail] && isSimilar(req, avail) {
				suggestions = append(suggestions, fmt.Sprintf("%s -> %s", req, avail))
				seen[avail] = true
			}
		}
	}

	return suggestions
}

// isSimilar checks if two strings are similar enough to suggest.
// Returns true if they differ by only a few characters or have similar structure.
func isSimilar(a, b string) bool {
	// If they're equal, they're not "similar" - they're exact (shouldn't happen here)
	if a == b {
		return false
	}

	// Use the existing similarity function from utils package
	return utils.IsSimilarWord(a, b)
}

// getProjectNameFromCommand extracts the project name from a CommentCommand.
// It handles both the legacy ProjectName field and the new ProjectNames slice.
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
