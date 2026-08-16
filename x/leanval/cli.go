package leanval

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

func cliQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Querying commands for the leanval module",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}
	cmd.AddCommand(cmdBondedSet())
	return cmd
}

func cmdBondedSet() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bonded-set [period]",
		Short: "Query lean_store.bonded_set(P) via ABCI store/subspace (no proto Query)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			p, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return err
			}
			res, err := clientCtx.QueryABCI(abci.RequestQuery{
				Path:  fmt.Sprintf("/store/%s/subspace", types.StoreKey),
				Data:  types.BondedPrefixForPeriod(p),
				Prove: false,
			})
			if err != nil {
				return err
			}
			rows, err := types.DecodeBondedSubspace(p, res.Value)
			if err != nil {
				return err
			}
			type outRow struct {
				Subject  string `json:"subject"`
				Weight   int64  `json:"weight"`
				HasProof bool   `json:"has_proof"`
			}
			out := struct {
				Period uint64   `json:"period"`
				Rows   []outRow `json:"rows"`
			}{Period: p}
			for _, r := range rows {
				out.Rows = append(out.Rows, outRow{
					Subject:  hex.EncodeToString(r.Subject),
					Weight:   r.Weight,
					HasProof: r.HasProof,
				})
			}
			bz, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				return err
			}
			return clientCtx.PrintBytes(bz)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
