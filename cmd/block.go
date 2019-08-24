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
	"os"
	"strconv"
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

var (
	outputFormat string
	funcsMap     = template.FuncMap{"au": func() interface{} { return au }}
	blockTpl     = template.Must(template.New("block").Funcs(funcsMap).Parse(blockTplText))
)

var blockCmd = &cobra.Command{
	Use:   "block",
	Short: "Inspects blocks",
	Long:  `This command supports inspecting blocks.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			args = []string{"head"}
		}

		if newOutputEncoder := utils.GetEncoderFunc(outputFormat); newOutputEncoder != nil {
			return printEncoded(newOutputEncoder(os.Stdout), args)
		}
		return printText(args)
	},
}

func printEncoded(enc utils.Encoder, args []string) error {
	s := &tezos.Service{Client: tezosClient}
	blocks := make([]*tezos.Block, len(args))

	for i, id := range args {
		block, err := s.GetBlock(context.TODO(), chainID, id)
		if err != nil {
			return err
		}
		blocks[i] = block
	}

	return enc.Encode(blocks)
}

func printText(args []string) error {
	s := &tezos.Service{Client: tezosClient}

	type blockTplData struct {
		*tezos.Block
		Successor *tezos.Block
		Volume    int64
		Fees      int64
	}

	tplData := make([]*blockTplData, len(args))

	for i, id := range args {
		block, err := s.GetBlock(context.TODO(), chainID, id)
		if err != nil {
			return err
		}

		t := &blockTplData{
			Block: block,
		}

		t.Successor, _ = s.GetBlock(context.TODO(), chainID, strconv.Itoa(int(block.Header.Level)+1)) // Just ignore an error

		for _, ol := range block.Operations {
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

	return blockTpl.Execute(os.Stdout, tplData)
}

func init() {
	blockCmd.PersistentFlags().StringVarP(&outputFormat, "output-format", "o", "text", "Output format: one of [text, yaml, json]")
	rootCmd.AddCommand(blockCmd)
}
