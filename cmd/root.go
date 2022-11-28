/*
Copyright © 2022 Aditya Sirish A Yelgundhalli
*/
package cmd

import (
	"os"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	PreRunE: preRoot,
	Use:     "gittuf",
	Short:   "Making Git repositories more TUF",
	Long: `gittuf embeds TUF's access control semantics in a Git repository.
The tool serves as a wrapper around Git, and a gittuf repository is compatible
with existing Git tooling.`,
}

var verbosity string

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.gittuf.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	rootCmd.PersistentFlags().StringVarP(
		&verbosity,
		"verbose",
		"v",
		logrus.DebugLevel.String(),
		"Verbosity level (debug, info, warn, error, fatal, panic)",
	)
}

func preRoot(cmd *cobra.Command, args []string) error {
	logrus.SetOutput(os.Stdout)
	level, err := logrus.ParseLevel(verbosity)
	if err != nil {
		return err
	}
	logrus.SetLevel(level)
	return nil
}

// Borrowed from go-tuf
func parseExpires(e string) (time.Time, error) {
	days, err := strconv.Atoi(e)
	if err != nil {
		return time.Time{}, err
	}
	return time.Now().AddDate(0, 0, days).UTC(), nil
}
