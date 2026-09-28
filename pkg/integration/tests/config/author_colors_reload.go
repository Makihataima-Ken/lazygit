package config

import (
	"github.com/jesseduffield/lazygit/pkg/config"
	. "github.com/jesseduffield/lazygit/pkg/integration/components"
)

var AuthorColorsReload = NewIntegrationTest(NewIntegrationTestArgs{
	Description:  "Editing the author colors and refocusing the window recolors the authors and the graph in the commits view",
	ExtraCmdArgs: []string{},
	Skip:         false,
	SetupConfig: func(cfg *config.AppConfig) {
		cfg.GetUserConfig().Git.Log.ShowGraph = "always"
	},
	SetupRepo: func(shell *Shell) {
		shell.EmptyCommit("one")
		shell.EmptyCommit("two")
		shell.CreateFile(".git/lazygit.yml", `
gui:
  authorColors:
    '*': '#00ff00'`)
	},
	Run: func(t *TestDriver, keys config.KeybindingConfig) {
		t.Views().Commits().
			ContainsColoredText("#00ff00", "CI").
			ContainsColoredText("#00ff00", "○")

		t.Shell().UpdateFile(".git/lazygit.yml", `
gui:
  authorColors:
    '*': '#ff0000'`)
		t.FocusIn()

		t.Views().Commits().
			ContainsColoredText("#ff0000", "CI").
			ContainsColoredText("#ff0000", "○")
	},
})
