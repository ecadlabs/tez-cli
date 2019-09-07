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
	"math/big"
	"os"
	"text/template"

	"github.com/spf13/cobra"

	tezos "github.com/ecadlabs/go-tezos"
)

// brief block info suitable for the template rendering
type opInfo struct {
	Source      string
	Kind        string
	Title       string
	Destination string
	Amount      *big.Float
	Fee         *big.Float
	Hash        string
	Block       *xblockInfo
}

func newBlockOperationsCommand(ctx *BlockCommandContext) *cobra.Command {
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

			blocks, err := ctx.getBlocks(args, true)
			if err != nil {
				return err
			}

			if ctx.newEncoder != nil {
				enc := ctx.newEncoder(os.Stdout)

				var data []*tezos.Operation
				for _, b := range blocks {
					ops := getRawBlockOperations(b.Block, kinds)
					data = append(data, ops...)
				}

				return enc.Encode(data)
			}

			if ctx.userTemplate != nil {
				for _, b := range blocks {
					ops := getBlockOperations(getBlockInfo(b), kinds)
					for _, op := range ops {
						if err := ctx.userTemplate.Execute(os.Stdout, op); err != nil {
							return err
						}
					}
				}
				return nil
			}

			// Standard template
			tpl, err := template.New("operation").Funcs(ctx.templateFuncMap).Parse(operationsTemplateSrc)
			if err != nil {
				return err
			}

			var data []*opInfo
			for _, b := range blocks {
				data = append(data, getBlockOperations(getBlockInfo(b), kinds)...)
			}

			return tpl.Execute(os.Stdout, data)
		},
	}

	operationsCmd.Flags().StringSliceVarP(&opKinds, "kind", "k", nil, "Operation kinds: either comma separated list of [end[orsement], act[ivate_account], prop[osals], bal[lot], rev[eal], transaction|tx, orig[ination], del[egation], seed_nonce_revelation, double_endorsement_evidence, double_baking_evidence] or `all'")

	return operationsCmd
}

func getBlockOperations(b *xblockInfo, opsFilter map[string]struct{}) (info []*opInfo) {
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
					Block: b,
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

func getRawBlockOperations(b *tezos.Block, opsFilter map[string]struct{}) (ops []*tezos.Operation) {
	for _, ol := range b.Operations {
		for _, o := range ol {
			for _, c := range o.Contents {
				if _, ok := opsFilter[c.OperationElemKind()]; ok || opsFilter == nil {
					ops = append(ops, o)
					break
				}
			}
		}
	}

	return
}
