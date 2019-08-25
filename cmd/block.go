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
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/template"

	"github.com/spf13/cobra"

	tezos "github.com/ecadlabs/go-tezos"
	"github.com/ecadlabs/tez/cmd/utils"
)

const blockTplText = `{{range . -}}
Block:{{"\t\t"}}{{au.BgGreen .Hash}}
Predecessor:{{"\t"}}{{au.Blue .Header.Predecessor}}
Successor:{{"\t"}}{{with .Successor}}{{.Hash}}{{else}}--{{end}}
Level:{{"\t\t"}}{{.Header.Level}}
Timestamp:{{"\t"}}{{printf "%-30v" .Header.Timestamp}}{{"\t"}}Nonce hash:{{"\t"}}{{.Metadata.NonceHash}}
Volume:{{"\t\t"}}{{printf "%-30d" .Volume | au.Green}}{{"\t"}}Fees:{{"\t"}}{{.Fees}}
{{end}}
`

// TODO: not all of these operation are supported by the client library
var knownKinds = map[string]struct{}{
	"endorsement":                 struct{}{},
	"seed_nonce_revelation":       struct{}{},
	"double_endorsement_evidence": struct{}{},
	"double_baking_evidence":      struct{}{},
	"activate_account":            struct{}{},
	"proposals":                   struct{}{},
	"ballot":                      struct{}{},
	"reveal":                      struct{}{},
	"transaction":                 struct{}{},
	"origination":                 struct{}{},
	"delegation":                  struct{}{},
}

// BlockCommandContext represents `block' command context shared with its children
type BlockCommandContext struct {
	*RootContext
	newEncoder      utils.NewEncoderFunc
	templateFuncMap template.FuncMap
}

type xblock struct {
	*tezos.Block
	Successor *tezos.Block `json:"-" yaml:"-"`
}

// NewBlockCommand returns new `block' command
func NewBlockCommand(rootCtx *RootContext) *cobra.Command {
	var (
		outputFormat string
		blockCmd     *cobra.Command // Forward declaration, see PersistentPreRunE below
	)

	ctx := BlockCommandContext{
		RootContext: rootCtx,
	}

	blockCmd = &cobra.Command{
		Use:     "block",
		Aliases: []string{"b"},
		Short:   "Blocks inspection",

		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// https://github.com/spf13/cobra/issues/216
			// Also note that `cmd` always points to the top level command and not to ourselves
			if err := blockCmd.Parent().PersistentPreRunE(cmd, args); err != nil {
				return err
			}

			ctx.newEncoder = utils.GetEncoderFunc(outputFormat)
			ctx.templateFuncMap = template.FuncMap{"au": func() interface{} { return ctx.colorizer }}

			return nil
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				args = []string{"head"}
			}

			blocks, err := ctx.getBlocks(args, ctx.newEncoder == nil)
			if err != nil {
				return err
			}

			if ctx.newEncoder != nil {
				enc := ctx.newEncoder(os.Stdout)
				return enc.Encode(blocks)
			}

			return ctx.printBlocksSummaryText(blocks)
		},
	}

	// Just an alias
	headerCmd := &cobra.Command{
		Use:   "header",
		Short: "Block header summary",
		RunE:  blockCmd.RunE,
	}

	var opKinds string

	operationsCmd := &cobra.Command{
		Use:     "operations",
		Aliases: []string{"op"},
		Short:   "Inspect block operations",

		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				args = []string{"head"}
			}

			var kinds map[string]struct{}
			if opKinds != "all" {
				s := strings.Split(opKinds, ",")
				kinds = make(map[string]struct{}, len(s))

				for _, op := range s {
					if _, ok := knownKinds[op]; !ok {
						return fmt.Errorf("Unknown operation kind: `%s'", op)
					}
					kinds[op] = struct{}{}
				}
			}

			return errors.New("Not implemented")
		},
	}

	// TODO: other kinds
	operationsCmd.Flags().StringVarP(&opKinds, "kind", "k", "all", "Operation kinds: either comma separated list of [transaction, endorsement] or `all'")

	blockCmd.PersistentFlags().StringVarP(&outputFormat, "output-format", "o", "text", "Output format: one of [text, yaml, json]")
	blockCmd.AddCommand(headerCmd)
	blockCmd.AddCommand(operationsCmd)

	return blockCmd
}

func (c *BlockCommandContext) getBlocks(ids []string, getSuccessors bool) ([]*xblock, error) {
	s := &tezos.Service{Client: c.tezosClient}

	blocks := make([]*xblock, len(ids))

	for i, id := range ids {
		block, err := s.GetBlock(context.TODO(), c.chainID, id)
		if err != nil {
			return nil, err
		}

		xb := xblock{
			Block: block,
		}

		if getSuccessors {
			xb.Successor, _ = s.GetBlock(context.TODO(), c.chainID, strconv.Itoa(int(block.Header.Level)+1)) // Just ignore an error
		}

		blocks[i] = &xb
	}

	return blocks, nil
}

func (c *BlockCommandContext) printBlocksSummaryText(blocks []*xblock) error {
	tpl, err := template.New("block").Funcs(c.templateFuncMap).Parse(blockTplText)
	if err != nil {
		return err
	}

	type blockTplData struct {
		*xblock
		Volume int64
		Fees   int64
	}

	tplData := make([]*blockTplData, len(blocks))

	for i, b := range blocks {
		t := &blockTplData{
			xblock: b,
		}

		for _, ol := range b.Operations {
			for _, o := range ol {
				for _, c := range o.Contents {
					if el, ok := c.(*tezos.TransactionOperationElem); ok {
						t.Fees += el.Fee.Int64()
						t.Volume += el.Amount.Int64()
					}
				}
			}
		}

		tplData[i] = t
	}

	return tpl.Execute(os.Stdout, tplData)
}
