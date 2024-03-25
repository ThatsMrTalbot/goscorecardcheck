package command

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/thatsmrtalbot/goscorecardcheck"
	"github.com/thatsmrtalbot/goscorecardcheck/internal/filesearch"
	"github.com/thatsmrtalbot/goscorecardcheck/internal/reporters"
	"gopkg.in/yaml.v3"
)

const configFileName = ".goscorecardcheck.yaml"

func NewScoreCardCheckCommand() *cobra.Command {
	// Define flag options
	var (
		noTests        = false
		configFilePath = ""
		reportFilePath = "-"
		reporter       reporters.Reporter
		reporterValue  = NewEnumValue(&reporter, "default", map[string]reporters.Reporter{
			"default":    reporters.Default,
			"checkstyle": reporters.Checkstyle,
		})
	)

	// Define the command
	cmd := &cobra.Command{
		Use:  "goscorecardcheck",
		Long: "Lints each import using the scorecard API to ensure all dependencies meet pre-defined policy.",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			// Load the config file
			config, err := getConfigFile(configFilePath)
			if err != nil {
				return err
			}

			// Get current working dir, then find all files based on provided
			// arguments
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			files, err := filesearch.Find(cwd, args, noTests)
			if err != nil {
				return err
			}

			// Create processor and process files
			processor, err := goscorecardcheck.NewProcessor(&config)
			if err != nil {
				return err
			}

			issues := processor.ProcessFiles(cmd.Context(), files)

			// Open the specified file for output
			reportFile := os.Stdout
			if reportFilePath != "-" {
				reportFile, err = os.OpenFile(reportFilePath, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)
				if err != nil {
					return err
				}

				defer reportFile.Close()
			}

			// Write the report
			if err := reporter.Write(reportFile, issues); err != nil {
				return err
			}

			return nil
		},
	}

	// Add flags to the command
	cmd.Flags().BoolVar(&noTests, "no-tests", false, "skip test files when linting")
	cmd.Flags().StringVarP(&configFilePath, "config", "f", configFilePath, "scorecard config file")
	cmd.Flags().StringVarP(&reportFilePath, "report", "o", reportFilePath, "file to output report to")
	cmd.Flags().Var(reporterValue, "format", "set the format of the report")

	return cmd
}

func getConfigFile(configFilePath string) (config goscorecardcheck.Configuration, err error) {
	path, err := getConfigFilePath(configFilePath)
	if err != nil {
		return config, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return config, err
	}

	err = yaml.Unmarshal(data, &config)
	return
}

func getConfigFilePath(configFilePath string) (string, error) {
	// Use user specified config if provided
	if configFilePath != "" {
		return configFilePath, nil
	}

	// Get the home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Search for a config file, first in the local folder, then the home dir
	return getFirstFileThatExists(
		configFileName,
		filepath.Join(homeDir, configFileName),
	)
}

func getFirstFileThatExists(paths ...string) (string, error) {
	for _, path := range paths {
		// Stat the file, to check if it exists
		_, err := os.Stat(path)
		switch {
		case os.IsNotExist(err):
			continue
		case err != nil:
			return "", err
		}

		// Return the file
		return path, nil
	}

	return "", fmt.Errorf("no config file found, searched %s", strings.Join(paths, ", "))
}
