package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

const (
	allowedThemeKeysMessage = "may only set gui.theme, gui.authorColors and gui.branchColorPatterns"
	emptyThemeValueMessage  = "none of its values may be empty"
)

func TestValidateThemeFileContent(t *testing.T) {
	scenarios := []struct {
		name           string
		content        string
		expectedErrors []string
	}{
		{
			name:    "All allowed keys",
			content: "gui: {theme: {activeBorderColor: ['#ff00ff', bold], defaultFgColor: [white]}, authorColors: {'*': '#b4befe'}, branchColorPatterns: {'^feature/': green}}",
		},
		{
			name:    "Empty file",
			content: "",
		},
		{
			name:    "Only a comment",
			content: "# no colors yet\n",
		},
		{
			name:    "Gui without a value",
			content: "gui:\n",
		},
		{
			name:    "Theme without a value",
			content: "gui:\n  theme:\n",
		},
		{
			name:    "Anchors and aliases within the allowed keys",
			content: "gui: {theme: {activeBorderColor: &accent ['#ff00ff', bold], searchingActiveBorderColor: *accent}}",
		},
		{
			name:    "Merge keys within the allowed keys",
			content: "gui: {authorColors: &palette {'*': '#b4befe'}, branchColorPatterns: {<<: *palette, '^feature/': green}}",
		},
		{
			// yaml skips the entry whose key is null, so this only sets John's
			// color; checking the values must not follow the alias forever
			name:    "Alias inside its own anchor",
			content: "gui: {authorColors: &authors {John: red, ~: *authors}}",
		},
		{
			name:           "Unknown top-level key",
			content:        "git: {autoFetch: false}",
			expectedErrors: []string{allowedThemeKeysMessage, "field git not found"},
		},
		{
			name:           "Gui key that isn't about colors",
			content:        "gui: {nerdFontsVersion: '3'}",
			expectedErrors: []string{allowedThemeKeysMessage, "field nerdFontsVersion not found"},
		},
		{
			name:           "Deprecated branchColors",
			content:        "gui: {branchColors: {feature: green}}",
			expectedErrors: []string{allowedThemeKeysMessage, "field branchColors not found"},
		},
		{
			name:           "Unknown theme key",
			content:        "gui: {theme: {selectedRangeBgColor: [blue]}}",
			expectedErrors: []string{allowedThemeKeysMessage, "field selectedRangeBgColor not found"},
		},
		{
			name:           "Theme keys without gui",
			content:        "theme: {activeBorderColor: [red]}",
			expectedErrors: []string{allowedThemeKeysMessage, "field theme not found"},
		},
		{
			name:           "Invalid yaml",
			content:        "gui: {theme: [",
			expectedErrors: []string{allowedThemeKeysMessage, "yaml:"},
		},
		{
			name:           "Author colors without a value",
			content:        "gui:\n  authorColors:\n",
			expectedErrors: []string{emptyThemeValueMessage, "gui.authorColors has no value"},
		},
		{
			name:           "Border color set to null",
			content:        "gui:\n  theme:\n    activeBorderColor: null\n",
			expectedErrors: []string{emptyThemeValueMessage, "gui.theme.activeBorderColor has no value"},
		},
		{
			name:           "Empty list of colors",
			content:        "gui: {theme: {activeBorderColor: []}}",
			expectedErrors: []string{emptyThemeValueMessage, "gui.theme.activeBorderColor is an empty list"},
		},
		{
			name:           "Empty map of branch colors",
			content:        "gui: {branchColorPatterns: {}}",
			expectedErrors: []string{emptyThemeValueMessage, "gui.branchColorPatterns is an empty map"},
		},
		{
			name:           "Null in a list of colors",
			content:        "gui: {theme: {activeBorderColor: ['#ff00ff', ~]}}",
			expectedErrors: []string{emptyThemeValueMessage, "gui.theme.activeBorderColor[1] has no value"},
		},
		{
			name:           "Author without a color",
			content:        "gui: {authorColors: {John: ~}}",
			expectedErrors: []string{emptyThemeValueMessage, `gui.authorColors["John"] has no value`},
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			err := validateThemeFileContent([]byte(s.content))
			if len(s.expectedErrors) == 0 {
				assert.NoError(t, err)
				return
			}
			for _, expectedError := range s.expectedErrors {
				assert.ErrorContains(t, err, expectedError)
			}
		})
	}
}

func TestIsValidThemeName(t *testing.T) {
	scenarios := []struct {
		name     string
		expected bool
	}{
		{name: "dark", expected: true},
		{name: "Catppuccin Mocha", expected: true},
		{name: "", expected: false},
		{name: ".", expected: false},
		{name: "..", expected: false},
		{name: ".hidden", expected: false},
		{name: "../dark", expected: false},
		{name: "sub/dark", expected: false},
	}

	for _, s := range scenarios {
		assert.Equal(t, s.expected, isValidThemeName(s.name), "theme name %q", s.name)
	}
}

func TestSelectedThemeFilePath(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("CONFIG_DIR", stateDir)

	path, err := selectedThemeFilePath()

	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(stateDir, selectedThemeFileName), path)
}

func TestLoadSelectedThemeName(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("CONFIG_DIR", stateDir)
	path := filepath.Join(stateDir, selectedThemeFileName)

	name, err := loadSelectedThemeName()
	assert.NoError(t, err)
	assert.Equal(t, "", name, "missing file")

	writeThemeTestFile(t, path, "")
	name, err = loadSelectedThemeName()
	assert.NoError(t, err)
	assert.Equal(t, "", name, "empty file")

	writeThemeTestFile(t, path, "name: dark\n")
	name, err = loadSelectedThemeName()
	assert.NoError(t, err)
	assert.Equal(t, "dark", name)

	writeThemeTestFile(t, path, "name: [dark\n")
	_, err = loadSelectedThemeName()
	assert.ErrorContains(t, err, path)
}

func TestGetThemesDir(t *testing.T) {
	assert.Equal(t, "", NewDummyAppConfig().GetThemesDir())
	assert.Nil(t, NewDummyAppConfig().themeConfigFile("dark"))

	appConfig, configDir := newThemeTestAppConfig(t, "")
	assert.Equal(t, filepath.Join(configDir, "themes"), appConfig.GetThemesDir())
}

func TestListThemes(t *testing.T) {
	themes, err := NewDummyAppConfig().ListThemes()
	assert.NoError(t, err)
	assert.Nil(t, themes, "no config dir")

	appConfig, configDir := newThemeTestAppConfig(t, "")

	themes, err = appConfig.ListThemes()
	assert.NoError(t, err)
	assert.Nil(t, themes, "missing themes folder")

	themesDir := filepath.Join(configDir, "themes")
	for _, fileName := range []string{
		"zed.yml",
		"Dark.yml",
		"alpha beta.yml",
		"light.yaml",
		"shout.YML",
		".hidden.yml",
		".yml",
		"notes.txt",
	} {
		writeThemeTestFile(t, filepath.Join(themesDir, fileName), "")
	}
	assert.NoError(t, os.Mkdir(filepath.Join(themesDir, "folder.yml"), 0o755))

	themes, err = appConfig.ListThemes()
	assert.NoError(t, err)
	assert.Equal(t, []string{"alpha beta", "Dark", "zed"}, themes)
}

func TestListThemesFollowsLinks(t *testing.T) {
	appConfig, configDir := newThemeTestAppConfig(t, "")
	themesDir := filepath.Join(configDir, "themes")
	assert.NoError(t, os.MkdirAll(themesDir, 0o755))
	target := filepath.Join(t.TempDir(), "mocha.yml")
	writeThemeTestFile(t, target, "")
	if err := os.Symlink(target, filepath.Join(themesDir, "mocha.yml")); err != nil {
		t.Skipf("can't create symlinks here: %v", err)
	}
	assert.NoError(t, os.Symlink(filepath.Join(themesDir, "missing.yml"), filepath.Join(themesDir, "broken.yml")))

	themes, err := appConfig.ListThemes()

	assert.NoError(t, err)
	assert.Equal(t, []string{"mocha"}, themes)
}

func TestReloadUserConfigForRepoAppliesThemeBetweenGlobalAndRepoConfig(t *testing.T) {
	appConfig, configDir := newThemeTestAppConfig(t, `gui:
  theme:
    activeBorderColor:
      - red
    inactiveBorderColor:
      - blue
  authorColors:
    John: red
    '*': white
`)
	writeThemeTestFile(t, filepath.Join(configDir, "themes", "pink.yml"), `gui:
  theme:
    activeBorderColor:
      - '#ff00ff'
    optionsTextColor:
      - '#00ff00'
  authorColors:
    '*': '#ff00ff'
`)
	selectThemeForTest(t, configDir, "pink")
	repoConfigPath := filepath.Join(t.TempDir(), "lazygit.yml")
	writeThemeTestFile(t, repoConfigPath, `gui:
  theme:
    optionsTextColor:
      - yellow
`)
	repoConfigFiles := []*ConfigFile{{Path: repoConfigPath, Policy: ConfigFilePolicySkipIfMissing}}

	err := appConfig.ReloadUserConfigForRepo(repoConfigFiles)

	assert.NoError(t, err)
	assert.NoError(t, appConfig.GetThemeLoadError())
	assert.Equal(t, "pink", appConfig.GetSelectedTheme())
	assert.Equal(t, "pink", appConfig.GetAppliedTheme())
	themeConfig := appConfig.GetUserConfig().Gui.Theme
	assert.Equal(t, []string{"#ff00ff"}, themeConfig.ActiveBorderColor, "the theme overrides the global config")
	assert.Equal(t, []string{"blue"}, themeConfig.InactiveBorderColor, "the global config stays where the theme is silent")
	assert.Equal(t, []string{"yellow"}, themeConfig.OptionsTextColor, "the repo config overrides the theme")
	assert.Equal(t,
		map[string]string{"John": "red", "*": "#ff00ff"},
		appConfig.GetUserConfig().Gui.AuthorColors,
		"maps are merged key by key",
	)
	assert.Equal(t, repoConfigFiles, appConfig.repoUserConfigFiles)
	assert.Len(t, appConfig.globalUserConfigFiles, 1)
}

func TestReloadUserConfigForRepoWithoutSelectedTheme(t *testing.T) {
	appConfig, configDir := newThemeTestAppConfig(t, "")
	writeThemeTestFile(t, filepath.Join(configDir, "themes", "pink.yml"), `gui:
  branchColorPatterns:
    master: '#ff00ff'
`)

	assert.NoError(t, appConfig.ReloadUserConfigForRepo(nil))

	assert.Equal(t, "", appConfig.GetSelectedTheme())
	assert.Equal(t, "", appConfig.GetAppliedTheme())
	assert.NoError(t, appConfig.GetThemeLoadError())
	assert.Empty(t, appConfig.GetUserConfig().Gui.BranchColorPatterns)
	assert.False(t, lo.SomeBy(appConfig.userConfigFiles, func(f *ConfigFile) bool { return f.isTheme }))
}

func TestReloadUserConfigForRepoFallsBackToNoThemeWhenThemeFileIsInvalid(t *testing.T) {
	appConfig, configDir := newThemeTestAppConfig(t, `gui:
  branchColorPatterns:
    main: red
`)
	themePath := filepath.Join(configDir, "themes", "broken.yml")
	writeThemeTestFile(t, themePath, `gui:
  nerdFontsVersion: "3"
  branchColorPatterns:
    master: '#ff00ff'
`)
	selectThemeForTest(t, configDir, "broken")

	err := appConfig.ReloadUserConfigForRepo(nil)

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"main": "red"}, appConfig.GetUserConfig().Gui.BranchColorPatterns)
	assert.Equal(t, "broken", appConfig.GetSelectedTheme())
	assert.Equal(t, "", appConfig.GetAppliedTheme())
	assert.False(t, lo.SomeBy(appConfig.userConfigFiles, func(f *ConfigFile) bool { return f.isTheme }))

	themeLoadError := appConfig.GetThemeLoadError()
	var themeFileError *ThemeFileError
	if assert.ErrorAs(t, themeLoadError, &themeFileError) {
		assert.Equal(t, themePath, themeFileError.Path)
	}
	assert.ErrorContains(t, themeLoadError, "The theme file `"+themePath+"` couldn't be loaded.")
	assert.ErrorContains(t, themeLoadError, "field nerdFontsVersion not found")
	assert.ErrorContains(t, themeLoadError, allowedThemeKeysMessage)
	assert.Equal(t, themeLoadError, appConfig.GetThemeLoadError(), "reading the error doesn't clear it")
}

func TestThemeLoadErrorIsClearedOnceTheThemeLoads(t *testing.T) {
	appConfig, configDir := newThemeTestAppConfig(t, "")
	themePath := filepath.Join(configDir, "themes", "pink.yml")
	writeThemeTestFile(t, themePath, `gui:
  nerdFontsVersion: "3"
`)
	selectThemeForTest(t, configDir, "pink")

	assert.NoError(t, appConfig.ReloadUserConfigForRepo(nil))
	assert.Error(t, appConfig.GetThemeLoadError())

	// The next reload (e.g. a repo switch) reads the still broken file again
	assert.NoError(t, appConfig.ReloadUserConfigForRepo(nil))
	assert.Error(t, appConfig.GetThemeLoadError())

	writeThemeTestFile(t, themePath, `gui:
  branchColorPatterns:
    master: '#ff00ff'
`)
	assert.NoError(t, appConfig.ReloadUserConfigForRepo(nil))

	assert.NoError(t, appConfig.GetThemeLoadError())
	assert.Equal(t, "pink", appConfig.GetAppliedTheme())
	assert.Equal(t, map[string]string{"master": "#ff00ff"}, appConfig.GetUserConfig().Gui.BranchColorPatterns)
}

func TestReloadUserConfigForRepoFallsBackWhenThemeFileIsUnreadable(t *testing.T) {
	appConfig, configDir := newThemeTestAppConfig(t, "")
	// A folder can be stat'ed but not read as a file
	themePath := filepath.Join(configDir, "themes", "pink.yml")
	assert.NoError(t, os.MkdirAll(themePath, 0o755))
	selectThemeForTest(t, configDir, "pink")

	assert.NoError(t, appConfig.ReloadUserConfigForRepo(nil))

	assert.Equal(t, "", appConfig.GetAppliedTheme())
	var themeFileError *ThemeFileError
	if assert.ErrorAs(t, appConfig.GetThemeLoadError(), &themeFileError) {
		assert.Equal(t, themePath, themeFileError.Path)
	}
}

func TestReloadUserConfigForRepoSurvivesUnreachableThemeFile(t *testing.T) {
	appConfig, configDir := newThemeTestAppConfig(t, `gui:
  theme:
    activeBorderColor:
      - red
`)
	// With a file where the themes folder should be, stat'ing the theme file
	// fails with an error other than "doesn't exist" on Linux and macOS, while
	// Windows reports the theme file as missing. Either way lazygit starts
	// without the theme.
	writeThemeTestFile(t, filepath.Join(configDir, "themes"), "")
	selectThemeForTest(t, configDir, "pink")

	assert.NoError(t, appConfig.ReloadUserConfigForRepo(nil))
	assert.Equal(t, []string{"red"}, appConfig.GetUserConfig().Gui.Theme.ActiveBorderColor)
	assert.Equal(t, "pink", appConfig.GetSelectedTheme())
	assert.Equal(t, "", appConfig.GetAppliedTheme())
}

func TestMissingThemeFileIsAppliedOnceItAppears(t *testing.T) {
	appConfig, configDir := newThemeTestAppConfig(t, `gui:
  theme:
    activeBorderColor:
      - red
`)
	selectThemeForTest(t, configDir, "later")

	assert.NoError(t, appConfig.ReloadUserConfigForRepo(nil))
	assert.NoError(t, appConfig.GetThemeLoadError())
	assert.Equal(t, "later", appConfig.GetSelectedTheme())
	assert.Equal(t, "", appConfig.GetAppliedTheme(), "the theme file is missing")
	assert.Equal(t, []string{"red"}, appConfig.GetUserConfig().Gui.Theme.ActiveBorderColor)

	writeThemeTestFile(t, filepath.Join(configDir, "themes", "later.yml"), `gui:
  theme:
    activeBorderColor:
      - '#ff00ff'
`)
	err, didChange := appConfig.ReloadChangedUserConfigFiles()

	assert.NoError(t, err)
	assert.True(t, didChange)
	assert.Equal(t, "later", appConfig.GetAppliedTheme())
	assert.Equal(t, []string{"#ff00ff"}, appConfig.GetUserConfig().Gui.Theme.ActiveBorderColor)
}

func TestAppliedThemeIsKeptWhenReloadingTheThemeFileFails(t *testing.T) {
	appConfig, configDir := newThemeTestAppConfig(t, "")
	selectThemeForTest(t, configDir, "later")
	assert.NoError(t, appConfig.ReloadUserConfigForRepo(nil))
	assert.Equal(t, "", appConfig.GetAppliedTheme(), "the theme file is missing")

	themePath := filepath.Join(configDir, "themes", "later.yml")
	rewriteThemeTestFile(t, themePath, `gui:
  nerdFontsVersion: "3"
  branchColorPatterns:
    master: '#ff00ff'
`)
	err, didChange := appConfig.ReloadChangedUserConfigFiles()

	var themeFileError *ThemeFileError
	if assert.ErrorAs(t, err, &themeFileError) {
		assert.Equal(t, themePath, themeFileError.Path)
	}
	assert.False(t, didChange)
	// The config from before the reload stays in use, and it has no theme
	assert.Empty(t, appConfig.GetUserConfig().Gui.BranchColorPatterns)
	assert.Equal(t, "", appConfig.GetAppliedTheme())
	assert.NoError(t, appConfig.GetThemeLoadError(), "the reload on focus reports its error itself")
}

func TestAppliedThemeIsClearedWhenTheThemeFileIsDeleted(t *testing.T) {
	appConfig, configDir := newThemeTestAppConfig(t, "")
	themePath := filepath.Join(configDir, "themes", "pink.yml")
	writeThemeTestFile(t, themePath, `gui:
  branchColorPatterns:
    master: '#ff00ff'
`)
	selectThemeForTest(t, configDir, "pink")
	assert.NoError(t, appConfig.ReloadUserConfigForRepo(nil))
	assert.Equal(t, "pink", appConfig.GetAppliedTheme())

	assert.NoError(t, os.Remove(themePath))
	err, didChange := appConfig.ReloadChangedUserConfigFiles()

	assert.NoError(t, err)
	assert.True(t, didChange)
	assert.Equal(t, "pink", appConfig.GetSelectedTheme(), "the selection is kept")
	assert.Equal(t, "", appConfig.GetAppliedTheme())
	assert.Empty(t, appConfig.GetUserConfig().Gui.BranchColorPatterns)
}

func TestReloadUserConfigForRepoIgnoresInvalidThemeName(t *testing.T) {
	appConfig, configDir := newThemeTestAppConfig(t, "")
	// This is the file that "../x" would point to if it were used as a path
	writeThemeTestFile(t, filepath.Join(configDir, "x.yml"), `gui:
  branchColorPatterns:
    master: '#ff00ff'
`)
	selectThemeForTest(t, configDir, "../x")

	assert.NoError(t, appConfig.ReloadUserConfigForRepo(nil))
	assert.NoError(t, appConfig.GetThemeLoadError())
	assert.Equal(t, "../x", appConfig.GetSelectedTheme())
	assert.Equal(t, "", appConfig.GetAppliedTheme())
	assert.Empty(t, appConfig.GetUserConfig().Gui.BranchColorPatterns)
}

func TestReloadUserConfigForRepoMatchesThemeNameIgnoringCase(t *testing.T) {
	appConfig, configDir := newThemeTestAppConfig(t, "")
	themePath := filepath.Join(configDir, "themes", "pink.yml")
	writeThemeTestFile(t, themePath, `gui:
  branchColorPatterns:
    master: '#ff00ff'
`)
	// As if edited by hand
	selectThemeForTest(t, configDir, "Pink")

	assert.NoError(t, appConfig.ReloadUserConfigForRepo(nil))

	assert.NoError(t, appConfig.GetThemeLoadError())
	assert.Equal(t, "pink", appConfig.GetSelectedTheme(), "the name is spelled as listed")
	assert.Equal(t, "pink", appConfig.GetAppliedTheme())
	assert.Equal(t, map[string]string{"master": "#ff00ff"}, appConfig.GetUserConfig().Gui.BranchColorPatterns)
	loadedThemeFile, found := lo.Find(appConfig.userConfigFiles, func(f *ConfigFile) bool { return f.isTheme })
	if assert.True(t, found) {
		assert.Equal(t, themePath, loadedThemeFile.Path)
	}
}

func TestReloadUserConfigForRepoPrefersThemeNameWithSameCase(t *testing.T) {
	appConfig, configDir := newThemeTestAppConfig(t, "")
	themesDir := filepath.Join(configDir, "themes")
	writeThemeTestFile(t, filepath.Join(themesDir, "Pink.yml"), `gui:
  branchColorPatterns:
    master: '#00ff00'
`)
	writeThemeTestFile(t, filepath.Join(themesDir, "pink.yml"), `gui:
  branchColorPatterns:
    master: '#ff00ff'
`)
	if themes, _ := appConfig.ListThemes(); len(themes) != 2 {
		t.Skip("the file system ignores case, so both names are the same file")
	}
	selectThemeForTest(t, configDir, "pink")

	assert.NoError(t, appConfig.ReloadUserConfigForRepo(nil))

	assert.Equal(t, "pink", appConfig.GetSelectedTheme())
	assert.Equal(t, map[string]string{"master": "#ff00ff"}, appConfig.GetUserConfig().Gui.BranchColorPatterns)
}

func TestReloadUserConfigForRepoReportsUnparsableThemeSelection(t *testing.T) {
	appConfig, configDir := newThemeTestAppConfig(t, "")
	selectionPath := filepath.Join(configDir, selectedThemeFileName)
	writeThemeTestFile(t, selectionPath, "name: [unclosed\n")

	assert.NoError(t, appConfig.ReloadUserConfigForRepo(nil))
	assert.Equal(t, "", appConfig.GetSelectedTheme())
	assert.Equal(t, "", appConfig.GetAppliedTheme())
	assert.ErrorContains(t, appConfig.GetThemeLoadError(), selectionPath)

	writeThemeTestFile(t, selectionPath, "")
	assert.NoError(t, appConfig.ReloadUserConfigForRepo(nil))
	assert.NoError(t, appConfig.GetThemeLoadError(), "the next reload clears the error")
}

func TestThemeFileIsWatchedButNotOfferedForEditing(t *testing.T) {
	appConfig, configDir := newThemeTestAppConfig(t, "")
	themePath := filepath.Join(configDir, "themes", "pink.yml")
	writeThemeTestFile(t, themePath, `gui:
  theme:
    activeBorderColor:
      - '#ff00ff'
`)
	selectThemeForTest(t, configDir, "pink")
	assert.NoError(t, appConfig.ReloadUserConfigForRepo(nil))

	assert.Equal(t, []string{filepath.Join(configDir, ConfigFilename)}, appConfig.GetUserConfigPaths())

	rewriteThemeTestFile(t, themePath, `gui:
  theme:
    activeBorderColor:
      - '#00ff00'
`)
	err, didChange := appConfig.ReloadChangedUserConfigFiles()

	assert.NoError(t, err)
	assert.True(t, didChange)
	assert.Equal(t, []string{"#00ff00"}, appConfig.GetUserConfig().Gui.Theme.ActiveBorderColor)
}

func TestLoadingThemeFileDoesNotRewriteIt(t *testing.T) {
	appConfig, configDir := newThemeTestAppConfig(t, "")
	themePath := filepath.Join(configDir, "themes", "palette.yml")
	// Migrations can't handle aliases, so this also fails if the theme is migrated
	themeContent := `# One accent color for two keys
gui:
  theme:
    activeBorderColor: &accent
      - '#ff00ff'
      - bold
    searchingActiveBorderColor: *accent
`
	writeThemeTestFile(t, themePath, themeContent)
	selectThemeForTest(t, configDir, "palette")

	assert.NoError(t, appConfig.ReloadUserConfigForRepo(nil))

	assert.NoError(t, appConfig.GetThemeLoadError())
	assert.Equal(t, []string{"#ff00ff", "bold"}, appConfig.GetUserConfig().Gui.Theme.SearchingActiveBorderColor)
	actualContent, err := os.ReadFile(themePath)
	assert.NoError(t, err)
	assert.Equal(t, themeContent, string(actualContent))
}

// newThemeTestAppConfig creates an AppConfig whose config dir, which is also
// its state dir, is a fresh temp dir containing the given config.yml.
func newThemeTestAppConfig(t *testing.T, globalConfig string) (*AppConfig, string) {
	t.Helper()

	configDir := t.TempDir()
	t.Setenv("CONFIG_DIR", configDir)
	t.Setenv("LG_CONFIG_FILE", "")
	writeThemeTestFile(t, filepath.Join(configDir, ConfigFilename), globalConfig)

	appConfig, err := NewAppConfig("lazygit", "unversioned", "", "", "", false, t.TempDir())
	assert.NoError(t, err)
	return appConfig, configDir
}

func writeThemeTestFile(t *testing.T, path string, content string) {
	t.Helper()

	assert.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	assert.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// rewriteThemeTestFile writes a file after an AppConfig has loaded the config,
// like an edit made while lazygit runs. It sets the modification time well
// past the one recorded when loading, so that the change is noticed even where
// timestamps are coarse.
func rewriteThemeTestFile(t *testing.T, path string, content string) {
	t.Helper()

	writeThemeTestFile(t, path, content)
	later := time.Now().Add(time.Hour)
	assert.NoError(t, os.Chtimes(path, later, later))
}

func selectThemeForTest(t *testing.T, configDir string, name string) {
	t.Helper()

	writeThemeTestFile(t, filepath.Join(configDir, selectedThemeFileName), "name: "+name+"\n")
}

const pinkThemeTestContent = "gui:\n  branchColorPatterns:\n    master: '#ff00ff'\n"

func TestSelectThemeUpdatesOnlyTheThemeSettings(t *testing.T) {
	appConfig, configDir := newThemeTestAppConfig(t, "git:\n  autoFetch: false\n")
	writeThemeTestFile(t, filepath.Join(configDir, "themes", "pink.yml"),
		"gui:\n  branchColorPatterns:\n    master: '#ff00ff'\n  theme:\n    activeBorderColor:\n      - '#ff00ff'\n")
	assert.NoError(t, appConfig.ReloadUserConfigForRepo(nil))
	// A setting changed at runtime, like the sort order picked from its menu,
	appConfig.GetUserConfig().Git.LocalBranchSortOrder = "alphabetical"
	// and an edit of config.yml that hasn't been reloaded yet
	rewriteThemeTestFile(t, filepath.Join(configDir, ConfigFilename), "git:\n  autoFetch: true\n")
	previousUserConfig := appConfig.GetUserConfig()

	assert.NoError(t, appConfig.SelectTheme("pink"))

	userConfig := appConfig.GetUserConfig()
	assert.Equal(t, map[string]string{"master": "#ff00ff"}, userConfig.Gui.BranchColorPatterns)
	assert.Equal(t, []string{"#ff00ff"}, userConfig.Gui.Theme.ActiveBorderColor)
	assert.Equal(t, "alphabetical", userConfig.Git.LocalBranchSortOrder)
	assert.False(t, userConfig.Git.AutoFetch)
	assert.Nil(t, previousUserConfig.Gui.BranchColorPatterns, "the previous config isn't changed in place")

	// The edit of config.yml is still picked up by the next reload
	err, didChange := appConfig.ReloadChangedUserConfigFiles()
	assert.NoError(t, err)
	assert.True(t, didChange)
	assert.True(t, appConfig.GetUserConfig().Git.AutoFetch)
	assert.Equal(t, map[string]string{"master": "#ff00ff"}, appConfig.GetUserConfig().Gui.BranchColorPatterns)
}

func TestSelectThemeRemembersTheChoice(t *testing.T) {
	appConfig, configDir := newThemeTestAppConfig(t, "")
	writeThemeTestFile(t, filepath.Join(configDir, "themes", "pink.yml"), pinkThemeTestContent)
	assert.NoError(t, appConfig.ReloadUserConfigForRepo(nil))

	assert.NoError(t, appConfig.SelectTheme("pink"))

	assert.Equal(t, "pink", appConfig.GetSelectedTheme())
	assert.Equal(t, "pink", appConfig.GetAppliedTheme())
	content, err := os.ReadFile(filepath.Join(configDir, selectedThemeFileName))
	assert.NoError(t, err)
	assert.Equal(t, "name: pink\n", string(content))

	// The next start of lazygit applies it again
	restartedAppConfig, err := NewAppConfig("lazygit", "unversioned", "", "", "", false, t.TempDir())
	assert.NoError(t, err)
	assert.NoError(t, restartedAppConfig.ReloadUserConfigForRepo(nil))
	assert.Equal(t, "pink", restartedAppConfig.GetAppliedTheme())
	assert.Equal(t, map[string]string{"master": "#ff00ff"}, restartedAppConfig.GetUserConfig().Gui.BranchColorPatterns)
}

func TestSelectThemeFailsWhenTheChoiceCantBeSaved(t *testing.T) {
	appConfig, configDir := newThemeTestAppConfig(t, "")
	writeThemeTestFile(t, filepath.Join(configDir, "themes", "pink.yml"), pinkThemeTestContent)
	writeThemeTestFile(t, filepath.Join(configDir, "themes", "blue.yml"),
		"gui:\n  branchColorPatterns:\n    master: '#0000ff'\n")
	assert.NoError(t, appConfig.ReloadUserConfigForRepo(nil))
	assert.NoError(t, appConfig.SelectTheme("pink"))
	selectionPath := filepath.Join(configDir, selectedThemeFileName)
	assert.NoError(t, os.Chmod(selectionPath, 0o444))
	if file, err := os.OpenFile(selectionPath, os.O_WRONLY, 0); err == nil {
		file.Close()
		t.Skip("a read-only file can still be written here, e.g. when running as root")
	}
	userConfig := appConfig.GetUserConfig()

	err := appConfig.SelectTheme("blue")

	assert.ErrorIs(t, err, os.ErrPermission)
	assert.Same(t, userConfig, appConfig.GetUserConfig())
	assert.Equal(t, "pink", appConfig.GetSelectedTheme())
	assert.Equal(t, "pink", appConfig.GetAppliedTheme())
	assert.NoError(t, appConfig.GetThemeLoadError())
	savedName, err := loadSelectedThemeName()
	assert.NoError(t, err)
	assert.Equal(t, "pink", savedName)

	// Once the file can be written again, so can the choice
	assert.NoError(t, os.Chmod(selectionPath, 0o644))
	assert.NoError(t, appConfig.SelectTheme("blue"))
	assert.Equal(t, "blue", appConfig.GetAppliedTheme())
	assert.Equal(t, map[string]string{"master": "#0000ff"}, appConfig.GetUserConfig().Gui.BranchColorPatterns)
}

func TestSelectThemeWithEmptyNameDeselectsTheTheme(t *testing.T) {
	appConfig, configDir := newThemeTestAppConfig(t, "")
	themePath := filepath.Join(configDir, "themes", "pink.yml")
	writeThemeTestFile(t, themePath, pinkThemeTestContent)
	assert.NoError(t, appConfig.ReloadUserConfigForRepo(nil))
	assert.NoError(t, appConfig.SelectTheme("pink"))

	assert.NoError(t, appConfig.SelectTheme(""))

	assert.Equal(t, "", appConfig.GetSelectedTheme())
	assert.Equal(t, "", appConfig.GetAppliedTheme())
	assert.Nil(t, appConfig.GetUserConfig().Gui.BranchColorPatterns)
	savedName, err := loadSelectedThemeName()
	assert.NoError(t, err)
	assert.Equal(t, "", savedName)

	// The file of the theme that was selected before is no longer watched
	rewriteThemeTestFile(t, themePath, "gui:\n  branchColorPatterns:\n    master: '#00ff00'\n")
	err, didChange := appConfig.ReloadChangedUserConfigFiles()
	assert.NoError(t, err)
	assert.False(t, didChange)
}

func TestSelectThemeRejectsNamesThatAreNotListed(t *testing.T) {
	for _, name := range []string{"nope", "Pink", "pink.yml", ".pink", filepath.Join("..", themesDirName, "pink")} {
		t.Run(name, func(t *testing.T) {
			appConfig, configDir := newThemeTestAppConfig(t, "")
			writeThemeTestFile(t, filepath.Join(configDir, "themes", "pink.yml"), pinkThemeTestContent)
			writeThemeTestFile(t, filepath.Join(configDir, "themes", ".pink.yml"), pinkThemeTestContent)
			assert.NoError(t, appConfig.ReloadUserConfigForRepo(nil))
			userConfig := appConfig.GetUserConfig()

			err := appConfig.SelectTheme(name)

			assert.ErrorIs(t, err, ErrThemeNotFound)
			assert.Same(t, userConfig, appConfig.GetUserConfig())
			assert.Equal(t, "", appConfig.GetSelectedTheme())
			assert.NoFileExists(t, filepath.Join(configDir, selectedThemeFileName))
		})
	}
}

func TestSelectBrokenThemeChangesNothing(t *testing.T) {
	scenarios := []struct {
		name    string
		content string
	}{
		{name: "setting that isn't a theme setting", content: "gui:\n  branchColorPatterns:\n    master: '#00ff00'\ngit:\n  autoFetch: false\n"},
		{name: "empty value", content: "gui:\n  authorColors:\n"},
		{name: "invalid yaml", content: "gui: [\n"},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			appConfig, configDir := newThemeTestAppConfig(t, "")
			writeThemeTestFile(t, filepath.Join(configDir, "themes", "pink.yml"), pinkThemeTestContent)
			brokenPath := filepath.Join(configDir, "themes", "broken.yml")
			writeThemeTestFile(t, brokenPath, s.content)
			assert.NoError(t, appConfig.ReloadUserConfigForRepo(nil))
			assert.NoError(t, appConfig.SelectTheme("pink"))
			userConfig := appConfig.GetUserConfig()
			// An edit of config.yml that hasn't been reloaded yet
			rewriteThemeTestFile(t, filepath.Join(configDir, ConfigFilename), "git:\n  autoFetch: false\n")

			err := appConfig.SelectTheme("broken")

			var themeFileError *ThemeFileError
			if assert.ErrorAs(t, err, &themeFileError) {
				assert.Equal(t, brokenPath, themeFileError.Path)
			}
			assert.Same(t, userConfig, appConfig.GetUserConfig())
			assert.Equal(t, "pink", appConfig.GetSelectedTheme())
			assert.Equal(t, "pink", appConfig.GetAppliedTheme())
			assert.NoError(t, appConfig.GetThemeLoadError(), "broken isn't the selected theme")
			savedName, err := loadSelectedThemeName()
			assert.NoError(t, err)
			assert.Equal(t, "pink", savedName)

			// The edit of config.yml is still picked up by the next reload, and
			// the previous theme is still in place
			err, didChange := appConfig.ReloadChangedUserConfigFiles()
			assert.NoError(t, err)
			assert.True(t, didChange)
			assert.False(t, appConfig.GetUserConfig().Git.AutoFetch)
			assert.Equal(t, map[string]string{"master": "#ff00ff"}, appConfig.GetUserConfig().Gui.BranchColorPatterns)
		})
	}
}

func TestSelectListedThemeReportsThemeFileThatDisappeared(t *testing.T) {
	appConfig, configDir := newThemeTestAppConfig(t, "")
	assert.NoError(t, appConfig.ReloadUserConfigForRepo(nil))
	userConfig := appConfig.GetUserConfig()

	// SelectTheme has listed the theme, but its file is gone by the time it is
	// loaded
	err := appConfig.selectListedTheme("gone")

	assert.ErrorIs(t, err, ErrThemeNotFound)
	assert.Same(t, userConfig, appConfig.GetUserConfig())
	assert.Equal(t, "", appConfig.GetSelectedTheme())
	assert.NoFileExists(t, filepath.Join(configDir, selectedThemeFileName))
	assert.False(t, lo.SomeBy(appConfig.userConfigFiles, func(f *ConfigFile) bool { return f.isTheme }))
}

func TestSelectThemeClearsThemeLoadError(t *testing.T) {
	appConfig, configDir := newThemeTestAppConfig(t, "")
	themePath := filepath.Join(configDir, "themes", "pink.yml")
	writeThemeTestFile(t, themePath, "gui:\n  nerdFontsVersion: \"3\"\n")
	selectThemeForTest(t, configDir, "pink")
	assert.NoError(t, appConfig.ReloadUserConfigForRepo(nil))
	assert.Error(t, appConfig.GetThemeLoadError())

	writeThemeTestFile(t, themePath, pinkThemeTestContent)
	assert.NoError(t, appConfig.SelectTheme("pink"))

	assert.NoError(t, appConfig.GetThemeLoadError())
	assert.Equal(t, "pink", appConfig.GetAppliedTheme())
	assert.Equal(t, map[string]string{"master": "#ff00ff"}, appConfig.GetUserConfig().Gui.BranchColorPatterns)
}

func TestFailedReselectReplacesThemeLoadError(t *testing.T) {
	appConfig, configDir := newThemeTestAppConfig(t, "")
	themePath := filepath.Join(configDir, "themes", "pink.yml")
	writeThemeTestFile(t, themePath, "gui:\n  nerdFontsVersion: \"3\"\n")
	writeThemeTestFile(t, filepath.Join(configDir, "themes", "other.yml"), "gui:\n  branchColors:\n    master: green\n")
	selectThemeForTest(t, configDir, "pink")
	assert.NoError(t, appConfig.ReloadUserConfigForRepo(nil))
	startupError := appConfig.GetThemeLoadError()
	assert.ErrorContains(t, startupError, "field nerdFontsVersion not found")
	userConfig := appConfig.GetUserConfig()

	// A theme other than the selected one that fails to load says nothing
	// about why the selected one isn't applied
	var themeFileError *ThemeFileError
	assert.ErrorAs(t, appConfig.SelectTheme("other"), &themeFileError)
	assert.Same(t, startupError, appConfig.GetThemeLoadError())

	// Choosing the selected theme again loads its file again, which is now
	// broken in another way
	writeThemeTestFile(t, themePath, "gui:\n  authorColors:\n")
	err := appConfig.SelectTheme("pink")

	assert.ErrorAs(t, err, &themeFileError)
	assert.ErrorContains(t, err, "gui.authorColors has no value")
	assert.Same(t, err, appConfig.GetThemeLoadError())
	assert.Same(t, userConfig, appConfig.GetUserConfig())
	assert.Equal(t, "pink", appConfig.GetSelectedTheme())
	assert.Equal(t, "", appConfig.GetAppliedTheme())
	savedName, err := loadSelectedThemeName()
	assert.NoError(t, err)
	assert.Equal(t, "pink", savedName)
}

func TestThemeLoadErrorIsClearedWhenAReloadOnFocusLoadsTheTheme(t *testing.T) {
	appConfig, configDir := newThemeTestAppConfig(t, "")
	selectThemeForTest(t, configDir, "later")
	assert.NoError(t, appConfig.ReloadUserConfigForRepo(nil))
	// The missing theme file appears, but broken, so choosing the theme again
	// fails
	themePath := filepath.Join(configDir, "themes", "later.yml")
	writeThemeTestFile(t, themePath, "gui:\n  nerdFontsVersion: \"3\"\n")
	err := appConfig.SelectTheme("later")
	assert.Error(t, err)
	assert.Same(t, err, appConfig.GetThemeLoadError())

	// The file is watched because it was missing at the last load, so fixing
	// it gets it loaded
	rewriteThemeTestFile(t, themePath, pinkThemeTestContent)
	err, didChange := appConfig.ReloadChangedUserConfigFiles()

	assert.NoError(t, err)
	assert.True(t, didChange)
	assert.Equal(t, "later", appConfig.GetAppliedTheme())
	assert.Equal(t, map[string]string{"master": "#ff00ff"}, appConfig.GetUserConfig().Gui.BranchColorPatterns)
	assert.NoError(t, appConfig.GetThemeLoadError())
}

func TestSelectingNoThemeClearsThemeLoadError(t *testing.T) {
	appConfig, configDir := newThemeTestAppConfig(t, "")
	writeThemeTestFile(t, filepath.Join(configDir, "themes", "pink.yml"), "gui:\n  nerdFontsVersion: \"3\"\n")
	selectThemeForTest(t, configDir, "pink")
	assert.NoError(t, appConfig.ReloadUserConfigForRepo(nil))
	assert.Error(t, appConfig.GetThemeLoadError())

	assert.NoError(t, appConfig.SelectTheme(""))

	assert.NoError(t, appConfig.GetThemeLoadError())
	assert.Equal(t, "", appConfig.GetSelectedTheme())
}

func TestSelectThemeReplacesUnparsableThemeSelection(t *testing.T) {
	appConfig, configDir := newThemeTestAppConfig(t, "")
	writeThemeTestFile(t, filepath.Join(configDir, "themes", "pink.yml"), pinkThemeTestContent)
	writeThemeTestFile(t, filepath.Join(configDir, selectedThemeFileName), "name: [pink\n")
	assert.NoError(t, appConfig.ReloadUserConfigForRepo(nil))
	assert.Error(t, appConfig.GetThemeLoadError())

	assert.NoError(t, appConfig.SelectTheme("pink"))

	assert.NoError(t, appConfig.GetThemeLoadError())
	assert.Equal(t, "pink", appConfig.GetAppliedTheme())
	savedName, err := loadSelectedThemeName()
	assert.NoError(t, err)
	assert.Equal(t, "pink", savedName)
}

func TestSelectThemeKeepsRepoConfigOnTop(t *testing.T) {
	appConfig, configDir := newThemeTestAppConfig(t, "")
	writeThemeTestFile(t, filepath.Join(configDir, "themes", "pink.yml"),
		"gui:\n  branchColorPatterns:\n    master: '#ff00ff'\n    other: '#ff00ff'\n")
	repoConfigPath := filepath.Join(t.TempDir(), "lazygit.yml")
	writeThemeTestFile(t, repoConfigPath, "gui:\n  branchColorPatterns:\n    master: '#00ff00'\n")
	repoConfigFiles := []*ConfigFile{{Path: repoConfigPath, Policy: ConfigFilePolicySkipIfMissing}}
	assert.NoError(t, appConfig.ReloadUserConfigForRepo(repoConfigFiles))

	assert.NoError(t, appConfig.SelectTheme("pink"))

	assert.Equal(t,
		map[string]string{"master": "#00ff00", "other": "#ff00ff"},
		appConfig.GetUserConfig().Gui.BranchColorPatterns,
	)

	// The repo config file is still watched
	rewriteThemeTestFile(t, repoConfigPath, "gui:\n  branchColorPatterns:\n    master: '#0000ff'\n")
	err, didChange := appConfig.ReloadChangedUserConfigFiles()
	assert.NoError(t, err)
	assert.True(t, didChange)
	assert.Equal(t,
		map[string]string{"master": "#0000ff", "other": "#ff00ff"},
		appConfig.GetUserConfig().Gui.BranchColorPatterns,
	)
}

func TestThemeFileSelectedAtRuntimeIsReloadedWhenChanged(t *testing.T) {
	appConfig, configDir := newThemeTestAppConfig(t, "")
	themePath := filepath.Join(configDir, "themes", "pink.yml")
	writeThemeTestFile(t, themePath, pinkThemeTestContent)
	assert.NoError(t, appConfig.ReloadUserConfigForRepo(nil))
	assert.NoError(t, appConfig.SelectTheme("pink"))

	// Right after selecting, no file counts as changed
	err, didChange := appConfig.ReloadChangedUserConfigFiles()
	assert.NoError(t, err)
	assert.False(t, didChange)

	rewriteThemeTestFile(t, themePath, "gui:\n  branchColorPatterns:\n    master: '#00ff00'\n")
	err, didChange = appConfig.ReloadChangedUserConfigFiles()
	assert.NoError(t, err)
	assert.True(t, didChange)
	assert.Equal(t, map[string]string{"master": "#00ff00"}, appConfig.GetUserConfig().Gui.BranchColorPatterns)
}

func TestCopyThemeFieldsCopiesEverySettingThatAThemeMayContain(t *testing.T) {
	from := GetDefaultConfig()
	assert.NoError(t, yaml.Unmarshal([]byte(`
gui:
  theme:
    activeBorderColor:
      - '#ff00ff'
  authorColors:
    John: '#ff00ff'
  branchColorPatterns:
    master: '#ff00ff'
  nerdFontsVersion: "3"
`), from))
	to := GetDefaultConfig()

	themeFields := reflect.VisibleFields(reflect.TypeFor[themeGuiConfig]())
	for _, field := range themeFields {
		assert.NotEqual(t, themeTestGuiField(from, field.Name), themeTestGuiField(to, field.Name),
			"the test must set %s so that copying it is verified", field.Name)
	}

	copyThemeFields(to, from)

	for _, field := range themeFields {
		assert.Equal(t, themeTestGuiField(from, field.Name), themeTestGuiField(to, field.Name), field.Name)
	}
	assert.Equal(t, "", to.Gui.NerdFontsVersion)
}

// themeTestGuiField returns the value of the GuiConfig field with the given
// name.
func themeTestGuiField(userConfig *UserConfig, name string) any {
	return reflect.ValueOf(userConfig.Gui).FieldByName(name).Interface()
}
