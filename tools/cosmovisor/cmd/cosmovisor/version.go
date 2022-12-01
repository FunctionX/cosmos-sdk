package main

import (
	"encoding/json"
	"fmt"
	"runtime/debug"
	"strings"

	"cosmossdk.io/tools/cosmovisor"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

var version = ""

func init() {
	versionCmd.Flags().StringP(OutputFlag, "o", "text", "Output format (text|json)")
	rootCmd.AddCommand(versionCmd)
}

// OutputFlag defines the output format flag
var OutputFlag = "output"

var versionCmd = &cobra.Command{
	Use:          "version",
	Short:        "Prints the version of Cosmovisor.",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := cmd.Context().Value(cosmovisor.LoggerKey).(*zerolog.Logger)

		if val, err := cmd.Flags().GetString(OutputFlag); val == "json" && err == nil {
			return printVersionJSON(logger, args)
		}

		return printVersion(logger, args)
	},
}

func getVersion() string {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		panic("failed to get cosmovisor version")
	}

	// default version
	mainVersion := strings.TrimSpace(buildInfo.Main.Version)
	if !strings.EqualFold(mainVersion, "(devel)") {
		return mainVersion
	}

	// ldflags version
	if version != "" {
		return version
	}

	// no version
	commit := "unknown"
	for _, setting := range buildInfo.Settings {
		if setting.Key == "vcs.revision" {
			commit = strings.TrimSpace(setting.Value)
			break
		}
	}
	return fmt.Sprintf("devel-%s", commit)
}

func printVersion(logger *zerolog.Logger, args []string) error {
	fmt.Printf("cosmovisor version: %s\n", getVersion())

	// disable logger
	l := logger.Level(zerolog.Disabled)
	logger = &l

	buf := new(strings.Builder)
	if err := Run(logger, append([]string{"version"}, args...), StdOutRunOption(buf)); err != nil {
		return fmt.Errorf("failed to run version command: %w", err)
	}
	fmt.Printf("app version: %s", buf.String())
	return nil
}

func printVersionJSON(logger *zerolog.Logger, args []string) error {
	buf := new(strings.Builder)

	// disable logger
	l := logger.Level(zerolog.Disabled)
	logger = &l

	if err := Run(
		logger,
		[]string{"version", "--long", "--output", "json"},
		StdOutRunOption(buf),
	); err != nil {
		return fmt.Errorf("failed to run version command: %w", err)
	}

	out, err := json.Marshal(struct {
		Version    string          `json:"cosmovisor_version"`
		AppVersion json.RawMessage `json:"app_version"`
	}{
		Version:    getVersion(),
		AppVersion: json.RawMessage(buf.String()),
	})
	if err != nil {
		l := logger.Level(zerolog.TraceLevel)
		logger = &l
		return fmt.Errorf("can't print version output, expected valid json from APP, got: %s - %w", buf.String(), err)
	}

	fmt.Println(string(out))
	return nil
}
