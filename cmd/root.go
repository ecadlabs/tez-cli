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
	"fmt"

	"github.com/ecadlabs/go-tezos"
	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
)

type RootContext struct {
	TezosURL    string
	ChainID     string
	TezosClient *tezos.RPCClient
	Colorizer   aurora.Aurora
}

func NewRootCommand() *cobra.Command {
	var (
		useColors bool
		ctx       RootContext
	)

	rootCmd := &cobra.Command{
		Use:   "tez",
		Short: "An alternative CLI utility for Tezos",
		Long:  `This utility allows you to inspect and manipulate a running Tezos instance`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
			// cmd always points to the top level command!!!
			ctx.Colorizer = aurora.NewAurora(useColors)
			ctx.TezosClient, err = tezos.NewRPCClient(nil, ctx.TezosURL)
			if err != nil {
				err = fmt.Errorf("Failed to initilize tezos RPC client: %v", err)
			}
			return
		},
	}

	f := rootCmd.PersistentFlags()

	f.StringVar(&ctx.TezosURL, "url", "https://rpc.tezrpc.me/", "Tezos RPC end-point URL")
	f.StringVar(&ctx.ChainID, "chain", "main", "Chain ID")
	f.BoolVar(&useColors, "colors", true, "Use colors")

	rootCmd.AddCommand(NewBlockCommand(&ctx))

	return rootCmd
}

// Execute executes root command
func Execute() error {
	return NewRootCommand().Execute()
}
