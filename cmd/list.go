package cmd

import (
	"fmt"
	"strconv"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"

	"github.com/prdlk/gh-commit/internal/ui"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured repositories",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		ui.MagentaBoldf("Repositories")
		ui.Println()

		repos, err := store.ListRepos()
		if err != nil {
			return err
		}
		if len(repos) == 0 {
			ui.Dimf("No repositories configured yet")
			ui.Println()
			ui.Infof("Run 'gh commit init' in a git repository to get started")
			return nil
		}

		t := table.New().
			Border(lipgloss.NormalBorder()).
			Headers("Name", "Path", "Scopes").
			StyleFunc(func(row, col int) lipgloss.Style {
				s := lipgloss.NewStyle().Padding(0, 1)
				if col == 2 {
					s = s.Align(lipgloss.Right)
				}
				if row == table.HeaderRow || col == 0 {
					s = s.Bold(true)
				}
				return s
			})
		for _, r := range repos {
			t.Row(r.Name, r.Path, strconv.Itoa(r.ScopeCount))
		}
		fmt.Println(t)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
