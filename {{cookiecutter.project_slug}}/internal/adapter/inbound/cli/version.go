package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func (a *app) newVersionCmd() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print build information",
		Long: "Prints the version, commit and build date of this binary.\n\n" +
			"Use --json when a pipeline needs to record exactly which build ran.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if asJSON {
				encoder := json.NewEncoder(a.opts.Stdout)
				encoder.SetIndent("", "  ")
				return encoder.Encode(a.opts.Build)
			}
			_, err := fmt.Fprintln(a.opts.Stdout, a.opts.Build.String())
			return err
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "print build information as JSON")

	return cmd
}
