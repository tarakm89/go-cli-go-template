package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func (a *app) newDocsCmd() *cobra.Command {
	var outputDir string

	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Generate Markdown documentation for this command tree",
		Long: "Writes one Markdown page per command.\n\n" +
			"`make docs` calls this alongside gomarkdoc, and the docs workflow\n" +
			"publishes the result to the gh-pages branch, so the published\n" +
			"reference can never drift from the flags the binary actually has.",
		Args:   cobra.NoArgs,
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := os.MkdirAll(outputDir, 0o755); err != nil {
				return fmt.Errorf("create %s: %w", outputDir, err)
			}

			root := cmd.Root()
			// Cobra stamps a generation date into each page by default, which
			// would make every docs build a spurious commit on gh-pages.
			return doc.GenMarkdownTreeCustom(root, outputDir, func(string) string { return "" }, linkHandler)
		},
	}

	cmd.Flags().StringVar(&outputDir, "output-dir", filepath.Join("docs", "cli"),
		"directory to write the Markdown pages into")

	return cmd
}

// linkHandler turns a command page name into the link the site expects.
func linkHandler(name string) string {
	return strings.ToLower(name)
}
