package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// Explain command specific flags
	explainFileFlag  string
	explainLinesFlag string
)

var explainCmd = &cobra.Command{
	Use:   "explain",
	Short: "Explain selected code",
	Long: `Explain what the selected code does in natural language.
The LLM will analyze the code and provide a detailed explanation.`,
	RunE: runExplain,
}

func init() {
	explainCmd.Flags().StringVarP(&explainFileFlag, "file", "f", "", "File to explain")
	explainCmd.Flags().StringVar(&explainLinesFlag, "lines", "", "Line range (e.g., 10-20)")
}

func runExplain(cmd *cobra.Command, args []string) error {
	// TODO: Implement explain functionality
	fmt.Println("Explain command")

	if explainFileFlag != "" {
		fmt.Printf("Explaining file: %s\n", explainFileFlag)
	}

	return nil
}
