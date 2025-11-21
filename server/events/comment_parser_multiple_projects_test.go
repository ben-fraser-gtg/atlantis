// Copyright 2017 HootSuite Media Inc.
//
// Licensed under the Apache License, Version 2.0 (the License);
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an AS IS BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// Modified hereafter by contributors to runatlantis/atlantis.

package events_test

import (
	"testing"

	"github.com/runatlantis/atlantis/server/events/models"
	. "github.com/runatlantis/atlantis/testing"
)

func TestParse_MultipleProjects(t *testing.T) {
	cases := []struct {
		description string
		comment     string
		expProjects []string
	}{
		{
			description: "single project flag",
			comment:     "atlantis plan -p project1",
			expProjects: []string{"project1"},
		},
		{
			description: "two project flags",
			comment:     "atlantis plan -p project1 -p project2",
			expProjects: []string{"project1", "project2"},
		},
		{
			description: "three project flags",
			comment:     "atlantis plan -p project1 -p project2 -p project3",
			expProjects: []string{"project1", "project2", "project3"},
		},
		{
			description: "multiple projects with apply",
			comment:     "atlantis apply -p project1 -p project2",
			expProjects: []string{"project1", "project2"},
		},
		{
			description: "multiple projects with approve_policies",
			comment:     "atlantis approve_policies -p project1 -p project2",
			expProjects: []string{"project1", "project2"},
		},
		{
			description: "multiple projects with version",
			comment:     "atlantis version -p project1 -p project2",
			expProjects: []string{"project1", "project2"},
		},
		{
			description: "multiple projects with import",
			comment:     "atlantis import -p project1 -p project2 address id",
			expProjects: []string{"project1", "project2"},
		},
		{
			description: "multiple projects with state",
			comment:     "atlantis state rm -p project1 -p project2 address",
			expProjects: []string{"project1", "project2"},
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			r := commentParser.Parse(c.comment, models.Github)
			Assert(t, r.Command != nil, "expected command to be parsed")
			Equals(t, c.expProjects, r.Command.ProjectNames)
		})
	}
}

func TestParse_MultipleProjectsWithFlags(t *testing.T) {
	cases := []struct {
		description string
		comment     string
		expProjects []string
		expVerbose  bool
	}{
		{
			description: "multiple projects with verbose flag",
			comment:     "atlantis plan -p project1 -p project2 --verbose",
			expProjects: []string{"project1", "project2"},
			expVerbose:  true,
		},
		{
			description: "multiple projects with extra terraform flags",
			comment:     "atlantis plan -p project1 -p project2 -- -target=resource",
			expProjects: []string{"project1", "project2"},
			expVerbose:  false,
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			r := commentParser.Parse(c.comment, models.Github)
			Assert(t, r.Command != nil, "expected command to be parsed")
			Equals(t, c.expProjects, r.Command.ProjectNames)
			Equals(t, c.expVerbose, r.Command.Verbose)
		})
	}
}

func TestParse_MultipleProjectsError(t *testing.T) {
	cases := []struct {
		description string
		comment     string
		expError    string
	}{
		{
			description: "multiple projects with workspace flag",
			comment:     "atlantis plan -p project1 -p project2 -w workspace",
			expError:    "cannot use -p/--project at same time as -w/--workspace or -d/--dir",
		},
		{
			description: "multiple projects with dir flag",
			comment:     "atlantis plan -p project1 -p project2 -d dir",
			expError:    "cannot use -p/--project at same time as -w/--workspace or -d/--dir",
		},
		{
			description: "multiple projects with workspace and dir flags",
			comment:     "atlantis plan -p project1 -p project2 -w workspace -d dir",
			expError:    "cannot use -p/--project at same time as -w/--workspace or -d/--dir",
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			r := commentParser.Parse(c.comment, models.Github)
			Assert(t, r.CommentResponse != "", "expected error response")
			Assert(t, r.Command == nil, "expected no command")
		})
	}
}

func TestParse_MultipleProjectsBackwardsCompatibility(t *testing.T) {
	t.Run("old single project syntax still works", func(t *testing.T) {
		comment := "atlantis plan -p project1"
		r := commentParser.Parse(comment, models.Github)
		Assert(t, r.Command != nil, "expected command to be parsed")
		Equals(t, []string{"project1"}, r.Command.ProjectNames)
	})
}

func TestParse_MultipleProjectsIsForSpecificProject(t *testing.T) {
	cases := []struct {
		description   string
		comment       string
		expIsSpecific bool
	}{
		{
			description:   "single project is specific",
			comment:       "atlantis plan -p project1",
			expIsSpecific: true,
		},
		{
			description:   "multiple projects are specific",
			comment:       "atlantis plan -p project1 -p project2",
			expIsSpecific: true,
		},
		{
			description:   "no project is not specific",
			comment:       "atlantis plan",
			expIsSpecific: false,
		},
		{
			description:   "dir flag is specific",
			comment:       "atlantis plan -d mydir",
			expIsSpecific: true,
		},
		{
			description:   "workspace flag is specific",
			comment:       "atlantis plan -w myworkspace",
			expIsSpecific: true,
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			r := commentParser.Parse(c.comment, models.Github)
			Assert(t, r.Command != nil, "expected command to be parsed")
			Equals(t, c.expIsSpecific, r.Command.IsForSpecificProject())
		})
	}
}

func TestParse_MultipleProjectsStringRepresentation(t *testing.T) {
	t.Run("multiple projects are included in string representation", func(t *testing.T) {
		comment := "atlantis plan -p project1 -p project2 -p project3"
		r := commentParser.Parse(comment, models.Github)
		Assert(t, r.Command != nil, "expected command to be parsed")

		str := r.Command.String()
		Assert(t, str != "", "expected non-empty string representation")
		// Should contain the projects in some form
		Equals(t, "project1,project2,project3", r.Command.ProjectNames[0]+","+r.Command.ProjectNames[1]+","+r.Command.ProjectNames[2])
	})
}

func TestParse_AllCommandsMultipleProjects(t *testing.T) {
	commands := []struct {
		name    string
		comment string
	}{
		{name: "plan", comment: "atlantis plan -p project1 -p project2"},
		{name: "apply", comment: "atlantis apply -p project1 -p project2"},
		{name: "approve_policies", comment: "atlantis approve_policies -p project1 -p project2"},
		{name: "version", comment: "atlantis version -p project1 -p project2"},
		{name: "import", comment: "atlantis import -p project1 -p project2 addr id"},
		{name: "state", comment: "atlantis state rm -p project1 -p project2 addr"},
	}

	for _, cmd := range commands {
		t.Run(cmd.name+" supports multiple projects", func(t *testing.T) {
			r := commentParser.Parse(cmd.comment, models.Github)
			Assert(t, r.Command != nil, "expected command to be parsed for "+cmd.name)
			Equals(t, []string{"project1", "project2"}, r.Command.ProjectNames)
		})
	}
}

func TestParse_MultipleDirsAndWorkspaces(t *testing.T) {
	cases := []struct {
		description   string
		comment       string
		expDirs       []string
		expWorkspaces []string
	}{
		{
			description:   "multiple dirs",
			comment:       "atlantis plan -d dir1 -d dir2",
			expDirs:       []string{"dir1", "dir2"},
			expWorkspaces: []string{},
		},
		{
			description:   "multiple workspaces",
			comment:       "atlantis plan -w workspace1 -w workspace2",
			expDirs:       []string{},
			expWorkspaces: []string{"workspace1", "workspace2"},
		},
		{
			description:   "multiple dirs and workspaces combined",
			comment:       "atlantis plan -d dir1 -d dir2 -w staging -w prod",
			expDirs:       []string{"dir1", "dir2"},
			expWorkspaces: []string{"staging", "prod"},
		},
		{
			description:   "apply with multiple dirs",
			comment:       "atlantis apply -d apps/frontend -d apps/backend",
			expDirs:       []string{"apps/frontend", "apps/backend"},
			expWorkspaces: []string{},
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			r := commentParser.Parse(c.comment, models.Github)
			Assert(t, r.Command != nil, "expected command to be parsed")
			Equals(t, c.expDirs, r.Command.RepoRelDirs)
			Equals(t, c.expWorkspaces, r.Command.Workspaces)
		})
	}
}

func TestParse_CannotMixProjectsWithDirsWorkspaces(t *testing.T) {
	cases := []struct {
		description string
		comment     string
	}{
		{
			description: "projects with dirs",
			comment:     "atlantis plan -p project1 -d dir1",
		},
		{
			description: "projects with workspaces",
			comment:     "atlantis plan -p project1 -w workspace1",
		},
		{
			description: "projects with both dirs and workspaces",
			comment:     "atlantis plan -p project1 -d dir1 -w workspace1",
		},
		{
			description: "multiple projects with multiple dirs",
			comment:     "atlantis plan -p project1 -p project2 -d dir1 -d dir2",
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			r := commentParser.Parse(c.comment, models.Github)
			Assert(t, r.CommentResponse != "", "expected error response")
			Assert(t, r.Command == nil, "expected no command")
		})
	}
}
