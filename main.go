// Copyright © 2018 ECAD Labs <frontdesk@ecadlabs.com>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package main

import (
	"fmt"
	"log"
	"os"

	"github.com/ecadlabs/go-tezos"
	"github.com/ecadlabs/tezos-cli/cmd"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string
var tezosURL string
var chainID string

func main() {
	var rootCmd = &cobra.Command{
		Use:   "tezos-cli",
		Short: "An alternative CLI utility for Tezos",
		Long:  `This utility allows you to inspect and manipulate a running Tezos instance`,
	}

	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.tezos-cli.yaml)")
	rootCmd.PersistentFlags().StringVar(&tezosURL, "url", "http://localhost:8732/", "Tezor RPC end-point URL")
	rootCmd.PersistentFlags().StringVar(&chainID, "chain", "main", "Chain ID (default=main)")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	client, err := tezos.NewRPCClient(nil, tezosURL)

	if err != nil {
		log.Printf("Failed to initilize tezos RPC client: %s", err)
		os.Exit(1)
	}
	rootCmd.AddCommand(cmd.NewBlockCommand(client, chainID))
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".tezos-cli" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".tezos-cli")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
