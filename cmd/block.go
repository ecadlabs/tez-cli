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

// NewBlockCommand returns a cobra command for interacting with blocks
func NewBlockCommand(client *tezos.RPCClient, chainID string) *cobra.Command {

	blockCmd := &cobra.Command{
		Use:   "block",
		Short: "Inspects blocks",
		Long:  `This command supports inspecting blocks.`,
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			for _, blockString := range args {
				blockPrint(client, blockString, chainID)
				/*
					for i := 8000; i < 18000; i++ {
						blockPrint(strconv.Itoa(i))
					}
				*/
			}
		},
	}

	blockCmd.Flags().StringVarP(&outputFormat, "output-format", "o", "text", "Output format: one of [text, json]")
	return blockCmd
}

func blockPrint(c *tezos.RPCClient, blockString, chainID string) {
	s := &tezos.Service{Client: c}

	block, err := s.GetBlock(context.TODO(), chainID, blockString)

	if err != nil {
		fmt.Println(err)
		return
	}

	switch outputFormat {
	case "json":
		fmt.Println(jsonifyWhatever(block))
	case "text":
		blockPrintText(c, block, chainID)
	}
}

func blockPrintText(c *tezos.RPCClient, block *tezos.Block, chainID string) {
	s := &tezos.Service{Client: c}
	succBlock, err := s.GetBlock(context.TODO(), chainID, strconv.Itoa(int(block.Header.Level)+1))
	var successor = "--"
	if err == nil {
		successor = succBlock.Hash
	}
	fmt.Println("Block:\t\t", BgGreen(block.Hash))
	fmt.Println("Predecessor:\t", Blue(block.Header.Predecessor))
	fmt.Println("Successor:\t", successor)
	fmt.Println("Level:\t\t", block.Header.Level)
	fmt.Println()
	fmt.Println("Timestamp:", block.Header.Timestamp, "\t\tNonce hash:", block.Metadata.NonceHash)

	volume := int64(0)
	fees := int64(0)
	for _, ol := range block.Operations {
		for _, o := range ol {
			for _, c := range o.Contents {
				if c.OperationElemKind() != "transaction" {
					continue
				}
				//fmt.Println(block.Header.Level, jsonifyWhatever(c))
				t := c.(*tezos.TransactionOperationElem)

				fees += t.Fee.Int64()
				volume += t.Amount.Int64()
			}
		}
	}

	fmt.Println("Volume:\t", Green(volume), "\t\tFees:", fees)
}
