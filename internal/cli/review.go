package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// Review command specific flags
	reviewFileFlag   string
	reviewDirFlag    string
	reviewGitFlag    bool
	reviewStagedFlag bool
)

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "AI-based code review",
	Long: `Perform an AI-based code review on files, directories, or git changes.
The LLM will analyze the code and provide feedback.`,
	RunE: runReview,
}

func init() {
	reviewCmd.Flags().StringVarP(&reviewFileFlag, "file", "f", "", "File to review")
	reviewCmd.Flags().StringVarP(&reviewDirFlag, "dir", "d", "", "Directory to review")
	reviewCmd.Flags().BoolVar(&reviewGitFlag, "git", false, "Review git diff")
	reviewCmd.Flags().BoolVar(&reviewStagedFlag, "staged", false, "Review staged changes")
}

func runReview(cmd *cobra.Command, args []string) error {
	// TODO: Implement review functionality
	fmt.Println("Review command")

	return nil
}
