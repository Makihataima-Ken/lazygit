package authors

import (
	"testing"

	"github.com/gookit/color"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/xo/terminfo"
)

func TestGetInitials(t *testing.T) {
	for input, expectedOutput := range map[string]string{
		"Jesse Duffield":     "JD",
		"Jesse Duffield Man": "JD",
		"JesseDuffield":      "Je",
		"J":                  "J",
		"六书六書":               "六",
		"書":                  "書",
		"":                   "",
	} {
		output := getInitials(input)
		if output != expectedOutput {
			t.Errorf("Expected %s to be %s", output, expectedOutput)
		}
	}
}

func TestAuthorWithLength(t *testing.T) {
	scenarios := []struct {
		authorName     string
		length         int
		expectedOutput string
	}{
		{"Jesse Duffield", 0, ""},
		{"Jesse Duffield", 1, ""},
		{"Jesse Duffield", 2, "JD"},
		{"Jesse Duffield", 3, "Je…"},
		{"Jesse Duffield", 10, "Jesse Duf…"},
		{"Jesse Duffield", 14, "Jesse Duffield"},
	}
	for _, s := range scenarios {
		assert.Equal(t, s.expectedOutput, utils.Decolorise(AuthorWithLength(s.authorName, s.length)))
	}
}

func TestSetCustomAuthorsRecolorsRenderedAuthors(t *testing.T) {
	oldColorLevel := color.ForceSetColorLevel(terminfo.ColorLevelMillions)
	defer color.ForceSetColorLevel(oldColorLevel)
	defer SetCustomAuthors(nil)

	SetCustomAuthors(map[string]string{"*": "#00ff00"})
	assert.Equal(t, "\x1b[38;2;0;255;0mJS\x1b[0m", ShortAuthor("Jane Smith"))
	assert.Equal(t, "\x1b[38;2;0;255;0mJane Smith\x1b[0m", LongAuthor("Jane Smith", 10))

	SetCustomAuthors(map[string]string{"*": "#ff0000"})
	/* EXPECTED:
	assert.Equal(t, "\x1b[38;2;255;0;0mJS\x1b[0m", ShortAuthor("Jane Smith"))
	assert.Equal(t, "\x1b[38;2;255;0;0mJane Smith\x1b[0m", LongAuthor("Jane Smith", 10))
	ACTUAL: */
	assert.Equal(t, "\x1b[38;2;0;255;0mJS\x1b[0m", ShortAuthor("Jane Smith"))
	assert.Equal(t, "\x1b[38;2;0;255;0mJane Smith\x1b[0m", LongAuthor("Jane Smith", 10))
}
