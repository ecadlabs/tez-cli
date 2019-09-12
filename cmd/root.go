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

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/ecadlabs/go-tezos"
	"github.com/logrusorgru/aurora"
	"github.com/mattn/go-isatty"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// RootContext represents root command context shared with its children
type RootContext struct {
	tezosURL  string
	chainID   string
	service   *tezos.Service
	colorizer aurora.Aurora
	context   context.Context
}

// NewRootCommand returns new root command
func NewRootCommand(ctx context.Context) *cobra.Command {
	var (
		useColors bool
		level     string
	)

	c := RootContext{
		context: ctx,
	}

	rootCmd := &cobra.Command{
		Use:   "tez",
		Short: "An alternative CLI utility for Tezos",
		Long:  `This utility allows you to inspect and manipulate a running Tezos instance`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
			// cmd always points to the top level command!!!
			c.colorizer = aurora.NewAurora(useColors && isatty.IsTerminal(os.Stdout.Fd()))
			client, err := tezos.NewRPCClient(nil, c.tezosURL)
			if err != nil {
				err = fmt.Errorf("Failed to initilize tezos RPC client: %v", err)
			}

			c.service = &tezos.Service{Client: client}

			lv, err := log.ParseLevel(level)
			if err != nil {
				return err
			}

			log.SetLevel(lv)

			return
		},
	}

	f := rootCmd.PersistentFlags()

	f.StringVarP(&c.tezosURL, "url", "u", "https://api.tez.ie/", "Tezos RPC end-point URL")
	f.StringVar(&c.chainID, "chain", "main", "Chain ID")
	f.BoolVar(&useColors, "colors", true, "Use colors")
	f.StringVar(&level, "log", "info", "Log level: [error, warn, info, debug, trace]")

	rootCmd.AddCommand(NewBlockCommand(&c))

	return rootCmd
}

// Execute executes root command
func Execute(ctx context.Context) error {
	return NewRootCommand(ctx).Execute()
}
