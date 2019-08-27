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

const blockTplText = `{{range . -}}
Block:        {{.Hash | au.BgGreen}}
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
Operations:   {{.OperationsNum}}
{{end}}
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
	blockCmd.AddCommand(headerCmd)
	blockCmd.AddCommand(operationsCmd)

	return blockCmd
}

func (c *BlockCommandContext) getBlock(query string, getSuccessor bool) (*xblock, error) {
	var i int
	for i < len(query) && (query[i] >= '0' && query[i] <= '9' || query[i] >= 'a' && query[i] <= 'z' || query[i] >= 'A' && query[i] <= 'Z') {
		i++
	}

	id := query[:i]

	var offset int
	if i < len(query) {
		// parse the offset
		if query[i] == '~' {
			for i < len(query) && query[i] == '~' {
				i++
				offset++
			}
		}

		if i < len(query) {
			v, err := strconv.ParseInt(query[i:], 10, 32)
			if err != nil {
				return nil, err
			}
			offset = int(v)
		}
	}

	s := &tezos.Service{Client: c.tezosClient}

	var (
		block *tezos.Block
		err   error
	)

	if len(id) == 0 || (id[0] >= '0' && id[0] <= '9') {
		// parse level
		var level int
		if len(id) != 0 {
			v, err := strconv.ParseInt(id, 10, 32)
			if err != nil {
				return nil, err
			}
			level = int(v)
		}

		block, err = s.GetBlock(context.TODO(), c.chainID, strconv.FormatInt(int64(level+offset), 10))
		if err != nil {
			return nil, err
		}
	} else {
		// traverse
		block, err = s.GetBlock(context.TODO(), c.chainID, id)
		if err != nil {
			return nil, err
		}

		if offset != 0 {
			block, err = s.GetBlock(context.TODO(), c.chainID, strconv.FormatInt(int64(block.Header.Level+offset), 10))
			if err != nil {
				return nil, err
			}
		}
	}

	xb := xblock{
		Block: block,
	}

	if getSuccessor {
		xb.Successor, _ = s.GetBlock(context.TODO(), c.chainID, strconv.Itoa(int(block.Header.Level)+1)) // Just ignore an error
	}

	return &xb, nil
}

func (c *BlockCommandContext) getBlocks(ids []string, getSuccessors bool) ([]*xblock, error) {
	blocks := make([]*xblock, len(ids))

	for i, id := range ids {
		var err error
		blocks[i], err = c.getBlock(id, getSuccessors)
		if err != nil {
			return nil, err
		}
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
		*blockInfo
	}

	tplData := make([]*blockTplData, len(blocks))

	for i, b := range blocks {
		tplData[i] = &blockTplData{
			xblock:    b,
			blockInfo: getBlockInfo(b.Block),
		}
	}

	return tpl.Execute(os.Stdout, tplData)
}

// brief block info suitable for the template rendering
type opInfo struct {
	From   string
	Type   string
	To     string
	Amount *big.Float
	Fee    *big.Float
	Hash   string
}

type blockInfo struct {
	Volume         *big.Float
	Fees           *big.Float
	Rewards        *big.Float
	OperationsNum  int
	OperationsInfo []*opInfo
}

func getBlockInfo(b *tezos.Block) *blockInfo {
	bi := blockInfo{
		Volume:  big.NewFloat(0),
		Fees:    big.NewFloat(0),
		Rewards: big.NewFloat(0),
	}

	for _, b := range b.Metadata.BalanceUpdates {
		if bu, ok := b.(*tezos.FreezerBalanceUpdate); ok {
			if bu.Category == "rewards" {
				var rewards big.Float
				rewards.SetInt64(int64(bu.Change))
				bi.Rewards.Add(bi.Rewards, &rewards)
			}
		}
	}

	for _, ol := range b.Operations {
		for _, o := range ol {
			bi.OperationsNum += len(o.Contents)

			for _, c := range o.Contents {
				switch el := c.(type) {
				case *tezos.TransactionOperationElem:
					var fee, amount big.Float
					fee.SetInt((*big.Int)(&el.Fee))
					bi.Fees.Add(bi.Fees, &fee)

					amount.SetInt((*big.Int)(&el.Amount))
					bi.Volume.Add(bi.Volume, &amount)

				case *tezos.EndorsementOperationElem:
					if el.Metadata != nil {
						for _, b := range el.Metadata.BalanceUpdates {
							if bu, ok := b.(*tezos.FreezerBalanceUpdate); ok {
								if bu.Category == "rewards" {
									var rewards big.Float
									rewards.SetInt64(int64(bu.Change))
									bi.Rewards.Add(bi.Rewards, &rewards)
								}
							}
						}
					}
				}
			}
		}
	}

	bi.Volume.Mul(bi.Volume, big.NewFloat(1e-6))
	bi.Fees.Mul(bi.Fees, big.NewFloat(1e-6))
	bi.Rewards.Mul(bi.Rewards, big.NewFloat(1e-6))

	return &bi
}
