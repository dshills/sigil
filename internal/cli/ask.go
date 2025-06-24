package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// Ask command specific flags
	askFileFlag  string
	askDirFlag   string
	askLinesFlag string
	askStdinFlag bool
)

var askCmd = &cobra.Command{
	Use:   "ask [question]",
	Short: "Ask a question about code",
	Long: `Ask a question about code in files, directories, or from stdin.
The LLM will analyze the code and provide an answer.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runAsk,
}

func init() {
	askCmd.Flags().StringVarP(&askFileFlag, "file", "f", "", "File to analyze")
	askCmd.Flags().StringVarP(&askDirFlag, "dir", "d", "", "Directory to analyze")
	askCmd.Flags().StringVar(&askLinesFlag, "lines", "", "Line range (e.g., 10-20)")
	askCmd.Flags().BoolVar(&askStdinFlag, "stdin", false, "Read from stdin")
}

func runAsk(cmd *cobra.Command, args []string) error {
	question := args[0]

	// TODO: Implement ask functionality
	fmt.Printf("Ask command: %s\n", question)

	if askFileFlag != "" {
		fmt.Printf("Analyzing file: %s\n", askFileFlag)
	}

	if askDirFlag != "" {
		fmt.Printf("Analyzing directory: %s\n", askDirFlag)
	}

	return nil
}
