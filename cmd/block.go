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
	"math/big"
	"os"
	"strconv"
	"text/template"

	tezos "github.com/ecadlabs/go-tezos"
	"github.com/ecadlabs/tez/cmd/utils"
	"github.com/spf13/cobra"
)

const blockTemplateSrc = `{{range . -}}
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
	userTemplate    *template.Template
	watch           bool
}

type xblock struct {
	*tezos.Block `yaml:",inline"`
	Successor    *tezos.Block `json:"-" yaml:"-"`
}

type xblockInfo struct {
	*xblock
	Volume        *big.Float
	Fees          *big.Float
	OperationsNum int
}

// NewBlockCommand returns new `block' command
func NewBlockCommand(rootCtx *RootContext) *cobra.Command {
	var (
		outputFormat string
		userTemplate string
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

			if userTemplate != "" {
				tpl, err := template.New("user").Funcs(ctx.templateFuncMap).Parse(userTemplate)
				if err != nil {
					return nil
				}
				ctx.userTemplate = tpl
			}

			return nil
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				args = []string{"head"}
			}

			var enc utils.Encoder
			if ctx.newEncoder != nil {
				enc = ctx.newEncoder(os.Stdout)
			}

			// Standard template
			tpl, err := template.New("block").Funcs(ctx.templateFuncMap).Parse(blockTemplateSrc)
			if err != nil {
				return err
			}

			if ctx.watch {
				var monErr error
				ch := make(chan *tezos.BlockInfo, 10)
				go func() {
					monErr = ctx.monitorHeads(ch)
					close(ch)
				}()

				var (
					tplErr error
					tplCh  chan *xblockInfo
					tplSem chan struct{}
				)

				if enc == nil && ctx.userTemplate == nil {
					tplCh = make(chan *xblockInfo, 10)
					tplSem = make(chan struct{})

					// Run template engine in background
					go func() {
						tplErr = tpl.Execute(os.Stdout, tplCh)
						close(tplSem)
					}()
				}

				for bi := range ch {
					block, err := ctx.getBlock(bi.Hash, false)
					if err != nil {
						if err != context.Canceled {
							return err
						}
						return nil
					}

					if enc != nil {
						if err := enc.Encode(block); err != nil {
							return err
						}
						continue
					}

					info := getBlockInfo(block)
					if ctx.userTemplate != nil {
						if err := ctx.userTemplate.Execute(os.Stdout, info); err != nil {
							return err
						}
						continue
					}
					// Send to the template
					tplCh <- info
				}

				if tplCh != nil {
					close(tplCh)
					<-tplSem
					if tplErr != nil {
						return tplErr
					}
				}

				if monErr != nil && monErr != context.Canceled {
					return monErr
				}
				return nil
			}

			// Get all at once
			blocks := make([]*xblock, len(args))
			for i, blockID := range args {
				block, err := ctx.getBlock(blockID, enc == nil)
				if err != nil {
					return err
				}
				blocks[i] = block
			}

			if enc != nil {
				// Encode as a slice
				return enc.Encode(blocks)
			}

			info := make([]*xblockInfo, len(blocks))
			for i, b := range blocks {
				info[i] = getBlockInfo(b)
			}

			if ctx.userTemplate != nil {
				for _, bi := range info {
					if err := ctx.userTemplate.Execute(os.Stdout, bi); err != nil {
						return err
					}
				}
				return nil
			}

			// Standard template expects a slice or a channel
			return tpl.Execute(os.Stdout, info)
		},
	}

	// Just an alias
	headerCmd := &cobra.Command{
		Use:   "header",
		Short: "Block header summary",
		RunE:  blockCmd.RunE,
	}

	blockCmd.PersistentFlags().StringVarP(&outputFormat, "output-encoding", "o", "text", "Output encoding: one of [text, yaml, json]")
	blockCmd.PersistentFlags().StringVar(&userTemplate, "output-fmt", "", "Output format (Go template)")
	blockCmd.PersistentFlags().BoolVar(&ctx.watch, "watch", false, "Ignore provided IDs and watch for new head blocks in a chain")
	blockCmd.AddCommand(headerCmd)

	blockCmd.AddCommand(newBlockOperationsCommand(&ctx))

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

		block, err = c.service.GetBlock(c.context, c.chainID, strconv.FormatInt(int64(level+offset), 10))
		if err != nil {
			return nil, err
		}
	} else {
		// traverse
		block, err = c.service.GetBlock(c.context, c.chainID, id)
		if err != nil {
			return nil, err
		}

		if offset != 0 {
			block, err = c.service.GetBlock(c.context, c.chainID, strconv.FormatInt(int64(block.Header.Level+offset), 10))
			if err != nil {
				return nil, err
			}
		}
	}

	xb := xblock{
		Block: block,
	}

	if getSuccessor {
		xb.Successor, _ = c.service.GetBlock(c.context, c.chainID, strconv.Itoa(int(block.Header.Level)+1)) // Just ignore an error
	}

	return &xb, nil
}

func (c *BlockCommandContext) monitorHeads(results chan<- *tezos.BlockInfo) (err error) {
	// Some endpoints closes connection
	for err == nil {
		err = c.service.MonitorHeads(c.context, c.chainID, results)
	}
	return
}

func getBlockInfo(b *xblock) *xblockInfo {
	bi := xblockInfo{
		xblock: b,
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
