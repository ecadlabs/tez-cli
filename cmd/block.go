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
Operations:   {{.OperationsNum}}

{{with .OperationsInfo -}}
{{range . -}}
Type:   {{or .Title .Kind}}
From:   {{or .Source "--"}}
To:     {{or .Destination "--"}}
Amount: {{if .Amount}}{{printf "%.6f ꜩ" .Amount}}{{else}}--{{end}}
Fee:    {{if .Fee}}{{printf "%.6f ꜩ" .Fee}}{{else}}--{{end}}
Hash:   {{.Hash}}

{{end -}}
{{end -}}
{{end -}}
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

var operationTitles = map[string]string{
	opEndorsement:               "Endorsement",
	opSeedNonceRevelation:       "Nonce",
	opDoubleEndorsementEvidence: "Double Endorsement Evidence",
	opDoubleBakingEvidence:      "Double Baking Evidence",
	opActivateAccount:           "Activation",
	opProposals:                 "Proposals",
	opBallot:                    "Ballot",
	opReveal:                    "Reveal",
	opTransaction:               "Transaction",
	opOrigination:               "Origination",
	opDelegation:                "Delegation",
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

			return ctx.printBlocksSummaryText(blocks, false, nil)
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

			// TODO JSON/YAML
			if ctx.newEncoder != nil {
				return errors.New("Not implemented")
			}

			blocks, err := ctx.getBlocks(args, true)
			if err != nil {
				return err
			}

			return ctx.printBlocksSummaryText(blocks, true, kinds)
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
		sign := 1
		if query[i] == '~' {
			sign = -1
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

		offset *= sign
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

func (c *BlockCommandContext) printBlocksSummaryText(blocks []*xblock, getops bool, opsFilter map[string]struct{}) error {
	tpl, err := template.New("block").Funcs(c.templateFuncMap).Parse(blockTplText)
	if err != nil {
		return err
	}

	type blockTplData struct {
		*xblock
		*blockInfo
		OperationsInfo []*opInfo
	}

	tplData := make([]*blockTplData, len(blocks))

	for i, b := range blocks {
		tplData[i] = &blockTplData{
			xblock:    b,
			blockInfo: getBlockInfo(b.Block),
		}

		if getops {
			tplData[i].OperationsInfo = getBlockOperations(b.Block, opsFilter)
		}
	}

	return tpl.Execute(os.Stdout, tplData)
}

// brief block info suitable for the template rendering
type opInfo struct {
	Source      string
	Kind        string
	Title       string
	Destination string
	Amount      *big.Float
	Fee         *big.Float
	Hash        string
}

type blockInfo struct {
	Volume        *big.Float
	Fees          *big.Float
	OperationsNum int
}

func getBlockInfo(b *tezos.Block) *blockInfo {
	bi := blockInfo{
		Volume: big.NewFloat(0),
		Fees:   big.NewFloat(0),
	}

	for _, ol := range b.Operations {
		for _, o := range ol {
			bi.OperationsNum += len(o.Contents)

			for _, c := range o.Contents {
				if el, ok := c.(tezos.OperationWithFee); ok {
					var fee big.Float
					if f := el.OperationFee(); f != nil {
						fee.SetInt(f)
						bi.Fees.Add(bi.Fees, &fee)
					}
				}

				if el, ok := c.(*tezos.TransactionOperationElem); ok {
					var amount big.Float
					if el.Amount != nil {
						amount.SetInt(&el.Amount.Int)
						bi.Volume.Add(bi.Volume, &amount)
					}
				}
			}
		}
	}

	bi.Volume.Mul(bi.Volume, big.NewFloat(1e-6))
	bi.Fees.Mul(bi.Fees, big.NewFloat(1e-6))

	return &bi
}

func getBlockOperations(b *tezos.Block, opsFilter map[string]struct{}) (info []*opInfo) {
	for _, ol := range b.Operations {
		for _, o := range ol {
			for _, c := range o.Contents {
				if _, ok := opsFilter[c.OperationElemKind()]; !ok && opsFilter != nil {
					// Skip
					continue
				}

				oi := &opInfo{
					Kind:  c.OperationElemKind(),
					Hash:  o.Hash,
					Title: operationTitles[c.OperationElemKind()],
				}

				if el, ok := c.(tezos.OperationWithFee); ok {
					if f := el.OperationFee(); f != nil {
						oi.Fee = big.NewFloat(0)
						oi.Fee.SetInt(f)
						oi.Fee.Mul(oi.Fee, big.NewFloat(1e-6))
					}
				}

				switch el := c.(type) {
				case *tezos.EndorsementOperationElem:
					oi.Source = el.Metadata.Delegate

				case *tezos.TransactionOperationElem:
					oi.Source = el.Source
					oi.Destination = el.Destination
					if el.Amount != nil {
						oi.Amount = big.NewFloat(0)
						oi.Amount.SetInt(&el.Amount.Int)
						oi.Amount.Mul(oi.Amount, big.NewFloat(1e-6))
					}

				case *tezos.BallotOperationElem:
					oi.Source = el.Source

				case *tezos.ProposalOperationElem:
					oi.Source = el.Source

				case *tezos.ActivateAccountOperationElem:
					oi.Source = el.PKH
					oi.Amount = big.NewFloat(0)
					for _, b := range el.Metadata.BalanceUpdates {
						if bu, ok := b.(*tezos.ContractBalanceUpdate); ok {
							var amount big.Float
							amount.SetInt64(int64(bu.Change))
							oi.Amount.Add(oi.Amount, &amount)
						}
					}
					oi.Amount.Mul(oi.Amount, big.NewFloat(1e-6))

				case *tezos.RevealOperationElem:
					oi.Source = el.Source

				case *tezos.OriginationOperationElem:
					oi.Source = el.Source
					oi.Destination = el.Delegate
					if el.Balance != nil {
						oi.Amount = big.NewFloat(0)
						oi.Amount.SetInt(&el.Balance.Int)
						oi.Amount.Mul(oi.Amount, big.NewFloat(1e-6))
					}

				case *tezos.DelegationOperationElem:
					oi.Source = el.Source
					oi.Destination = el.Delegate
					if el.Balance != nil {
						oi.Amount = big.NewFloat(0)
						oi.Amount.SetInt(&el.Balance.Int)
						oi.Amount.Mul(oi.Amount, big.NewFloat(1e-6))
					}
				}

				info = append(info, oi)
			}
		}
	}

	return
}
