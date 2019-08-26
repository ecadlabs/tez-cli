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
	"math/big"
	"os"
	"strconv"
	"text/template"

	"github.com/spf13/cobra"

	tezos "github.com/ecadlabs/go-tezos"
	"github.com/ecadlabs/tez/cmd/utils"
)

const blockTplText = `Block:        {{.Hash | au.BgGreen}}
Predecessor:  {{.Header.Predecessor | au.Blue}}
Successor:    {{with .Successor}}{{.Hash}}{{else}}--{{end}}
Timestamp:    {{.Header.Timestamp}}
Level:        {{.Header.Level}}
Cycle:        {{.Metadata.Level.Cycle}}
Priority:     {{.Header.Priority}}
Solvetime:    {{.Metadata.MaxOperationsTTL}}
Baker:        {{.Metadata.Baker}}
Consumed Gas: {{.Metadata.ConsumedGas}}
Volume:       {{printf "%.6f ꜩ" .Volume | au.Green}}
Fees:         {{printf "%.6f ꜩ" .Fees}}
Rewards:      {{printf "%.6f ꜩ" .Rewards}}
Operations:   {{.Operations}}

`

const (
	opEndorsement               = "endorsement"
	opSeedNonceRevelation       = "seed_nonce_revelation"
	opDoubleEndorsementEvidence = "double_endorsement_evidence"
	opDoubleBakingEvidence      = "double_baking_evidence"
	opActivateAccount           = "activate_account"
	opProposals                 = "proposals"
	opBallot                    = "ballot"
	opReveal                    = "reveal"
	opTransaction               = "transaction"
	opOrigination               = "origination"
	opDelegation                = "delegation"
)

// TODO: not all of these operation are supported by the client library
var knownKinds = map[string]string{
	"endorsement":                 opEndorsement,
	"end":                         opEndorsement,
	"seed_nonce_revelation":       opSeedNonceRevelation,
	"double_endorsement_evidence": opDoubleEndorsementEvidence,
	"double_baking_evidence":      opDoubleBakingEvidence,
	"activate_account":            opActivateAccount,
	"act":                         opActivateAccount,
	"proposals":                   opProposals,
	"prop":                        opProposals,
	"ballot":                      opBallot,
	"bal":                         opBallot,
	"reveal":                      opReveal,
	"rev":                         opReveal,
	"transaction":                 opTransaction,
	"tx":                          opTransaction,
	"origination":                 opOrigination,
	"orig":                        opOrigination,
	"delegation":                  opDelegation,
	"del":                         opDelegation,
}

// BlockCommandContext represents `block' command context shared with its children
type BlockCommandContext struct {
	*RootContext
	blockTemplate   string
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
		Aliases: []string{"bl"},
		Short:   "Blocks inspection",

		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// https://github.com/spf13/cobra/issues/216
			// Also note that `cmd` always points to the top level command and not to ourselves
			if p := blockCmd.Parent(); p != nil {
				if pr := p.PersistentPreRunE; pr != nil {
					if err := pr(cmd, args); err != nil {
						return err
					}
				}
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

	var opKinds []string

	operationsCmd := &cobra.Command{
		Use:     "operations",
		Aliases: []string{"op"},
		Short:   "Inspect block operations",

		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				args = []string{"head"}
			}

			var kinds map[string]struct{}
			if len(opKinds) != 0 {
				kinds = make(map[string]struct{}, len(opKinds))
				for _, kind := range opKinds {
					if k, ok := knownKinds[kind]; ok {
						kinds[k] = struct{}{}
					} else {
						return fmt.Errorf("Unknown operation kind: `%s'", k)
					}
				}
			}

			return errors.New("Not implemented")
		},
	}

	// TODO: other kinds
	operationsCmd.Flags().StringSliceVarP(&opKinds, "kind", "k", nil, "Operation kinds: either comma separated list of [end[orsement], act[ivate_account], prop[osals], bal[lot], rev[eal], transaction|tx, orig[ination], del[egation], seed_nonce_revelation, double_endorsement_evidence, double_baking_evidence] or `all'")

	blockCmd.PersistentFlags().StringVarP(&outputFormat, "output-encoding", "o", "text", "Output encoding: one of [text, yaml, json]")
	blockCmd.PersistentFlags().StringVar(&ctx.blockTemplate, "format", "", "Go template for the text encoding")
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
	tplText := c.blockTemplate
	if tplText == "" {
		tplText = blockTplText
	}

	tpl, err := template.New("block").Funcs(c.templateFuncMap).Parse(tplText)
	if err != nil {
		return err
	}

	type blockTplData struct {
		*xblock
		Operations int
		Volume     *big.Float
		Fees       *big.Float
		Rewards    *big.Float
	}

	for _, b := range blocks {
		t := blockTplData{
			xblock:  b,
			Volume:  big.NewFloat(0),
			Fees:    big.NewFloat(0),
			Rewards: big.NewFloat(0),
		}

		for _, b := range b.Metadata.BalanceUpdates {
			if bu, ok := b.(*tezos.FreezerBalanceUpdate); ok {
				if bu.Category == "rewards" {
					var rewards big.Float
					rewards.SetInt64(int64(bu.Change))
					t.Rewards.Add(t.Rewards, &rewards)
				}
			}
		}

		for _, ol := range b.Operations {
			for _, o := range ol {
				t.Operations += len(o.Contents)
				for _, c := range o.Contents {
					switch el := c.(type) {
					case *tezos.TransactionOperationElem:
						var fee, amount big.Float
						fee.SetInt((*big.Int)(&el.Fee))
						t.Fees.Add(t.Fees, &fee)

						amount.SetInt((*big.Int)(&el.Amount))
						t.Volume.Add(t.Volume, &amount)

					case *tezos.EndorsementOperationElem:
						if el.Metadata != nil {
							for _, b := range el.Metadata.BalanceUpdates {
								if bu, ok := b.(*tezos.FreezerBalanceUpdate); ok {
									if bu.Category == "rewards" {
										var rewards big.Float
										rewards.SetInt64(int64(bu.Change))
										t.Rewards.Add(t.Rewards, &rewards)
									}
								}
							}
						}
					}
				}
			}
		}

		t.Volume.Mul(t.Volume, big.NewFloat(1e-6))
		t.Fees.Mul(t.Fees, big.NewFloat(1e-6))
		t.Rewards.Mul(t.Rewards, big.NewFloat(1e-6))

		if err := tpl.Execute(os.Stdout, &t); err != nil {
			return err
		}
	}

	return nil
}
