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
	"strconv"

	"github.com/spf13/cobra"

	tezos "github.com/ecadlabs/go-tezos"
	. "github.com/logrusorgru/aurora"
)

var outputFormat string

// blockCmd represents the block command
var blockCmd = &cobra.Command{
	Use:   "block",
	Short: "Inspects blocks",
	Long:  `This command supports inspecting blocks.`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		for _, blockString := range args {
			blockNo, err := strconv.Atoi(blockString)
			if err != nil {
				fmt.Println("Invalid block number:", blockString)
				continue
			}
			blockPrint(blockNo)
		}
	},
}

func init() {
	rootCmd.AddCommand(blockCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// blockCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	blockCmd.Flags().StringVarP(&outputFormat, "output-format", "o", "text", "Output format: one of [text, json]")
}

func blockPrint(blockNo int) {
	c, err := tezos.NewRPCClient(nil, tezosURL)
	if err != nil {
		fmt.Println("Cannot connect to", tezosURL)
		panic(err)
	}
	s := &tezos.Service{Client: c}

	block, err := s.GetBlock(context.TODO(), chainID, strconv.Itoa(blockNo))

	if err != nil {
		fmt.Println(err)
		return
	}

	switch outputFormat {
	case "json":
		fmt.Println(jsonifyWhatever(block))
	case "text":
		blockPrintText(block)
	}
}

func blockPrintText(block *tezos.Block) {
	c, err := tezos.NewRPCClient(nil, tezosURL)
	if err != nil {
		fmt.Println("Cannot connect to", tezosURL)
		panic(err)
	}
	s := &tezos.Service{Client: c}
	succBlock, err := s.GetBlock(context.TODO(), chainID, strconv.Itoa(int(block.Header.Level)+1))
	var successor = "--"
	if err == nil {
		successor = succBlock.Hash
	}

	fmt.Println("Block:\t\t", BgGreen(block.Hash))
	fmt.Println("Predecessor:\t", Blue(block.Header.Predecessor))
	fmt.Println("Successor:\t", successor)
}
