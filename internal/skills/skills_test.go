package skills

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseContent(t *testing.T) {
	raw := `---
name: demo-skill
description: Demo skill for tests
user-invocable: true
---
Use this skill when asked to do demo work.
`

	skill, err := ParseContent([]byte(raw))
	require.NoError(t, err)
	require.Equal(t, "demo-skill", skill.Name)
	require.Equal(t, "Demo skill for tests", skill.Description)
	require.True(t, skill.UserInvocable)
	require.Equal(t, "Use this skill when asked to do demo work.", skill.Instructions)
}

func TestParseContentMissingFrontmatter(t *testing.T) {
	_, err := ParseContent([]byte("no frontmatter"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "no YAML frontmatter")
}

func TestValidate(t *testing.T) {
	skill := &Skill{
		Name:        "my-skill",
		Description: "valid",
		Path:        filepath.Join("/tmp", "my-skill"),
	}
	require.NoError(t, skill.Validate())

	invalid := &Skill{Name: "bad_name", Description: ""}
	err := invalid.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "description is required")
	require.Contains(t, err.Error(), "name must be alphanumeric")
}

func TestDiscoverWithStates(t *testing.T) {
	root := t.TempDir()

	goodDir := filepath.Join(root, "good-skill")
	require.NoError(t, os.MkdirAll(goodDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(goodDir, SkillFileName), []byte(`---
name: good-skill
description: Good skill
---
Instructions
`), 0o644))

	badDir := filepath.Join(root, "bad-skill")
	require.NoError(t, os.MkdirAll(badDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(badDir, SkillFileName), []byte(`---
name: bad_skill
description: Bad name format
---
Instructions
`), 0o644))

	skills, states := DiscoverWithStates([]string{root})
	require.Len(t, skills, 1)
	require.Equal(t, "good-skill", skills[0].Name)

	require.Len(t, states, 2)
	hasErrorState := false
	hasNormalState := false
	for _, state := range states {
		if state.State == StateError {
			hasErrorState = true
		}
		if state.State == StateNormal {
			hasNormalState = true
		}
	}
	require.True(t, hasErrorState)
	require.True(t, hasNormalState)
}

func TestDefaultPaths(t *testing.T) {
	paths, err := DefaultPaths("/workspace/project")
	require.NoError(t, err)
	require.Len(t, paths, 2)
	require.Contains(t, paths[0], ".opencode/skills")
	require.Equal(t, filepath.Join("/workspace/project", ".opencode", "skills"), paths[1])
}

func TestToPromptXMLSkipsDisabledInvocation(t *testing.T) {
	skills := []*Skill{
		{Name: "keep", Description: "keep me", SkillFilePath: "/tmp/keep/SKILL.md"},
		{Name: "skip", Description: "skip me", SkillFilePath: "/tmp/skip/SKILL.md", DisableModelInvocation: true},
	}

	xml := ToPromptXML(skills)
	require.Contains(t, xml, "<name>keep</name>")
	require.NotContains(t, xml, "<name>skip</name>")
}

func TestBuildPromptSection(t *testing.T) {
	root := t.TempDir()
	goodDir := filepath.Join(root, "good-skill")
	require.NoError(t, os.MkdirAll(goodDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(goodDir, SkillFileName), []byte(`---
name: good-skill
description: Good skill
---
Instructions
`), 0o644))

	section := BuildPromptSection([]string{root})
	require.Contains(t, section, "# Skills Information")
	require.Contains(t, section, "<available_skills>")
	require.Contains(t, section, "good-skill")
}
