package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/icon-project/btp2/common/cli"
	"github.com/spf13/cobra"
)

var (
	version = "unknown"
	build   = "unknown"
)

func main() {
	rootCmd, rootVc := cli.NewCommand(nil, nil, "btp-sdk-cli", "BTP-SDK CLI")
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	cli.SetEnvKeyReplacer(rootVc, strings.NewReplacer(" ", "_", ".", "_", "-", "_"))
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(rootCmd.Use, "version", version, build)
		},
	})

	var logoLines = []string{`
  ____ _______ _____        _____ _____  _  __
 |  _ \__   __|  __ \      / ____|  __ \| |/ /
 | |_) | | |  | |__) |____| (___ | |  | | ' / 
 |  _ <  | |  |  ___/______\___ \| |  | |  <  
 | |_) | | |  | |          ____) | |__| | . \ 
 |____/  |_|  |_|         |_____/|_____/|_|\_\
`,
	}
	NewServerCommand(rootCmd, rootVc, version, build, logoLines)
	NewApiCommand(rootCmd, rootVc)
	NewMonitorCommand(rootCmd, rootVc)

	genMdCmd := cli.NewGenerateMarkdownCommand(rootCmd, rootVc)
	genMdCmd.Hidden = true

	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("%+v\n", err)
		os.Exit(1)
	}
}
