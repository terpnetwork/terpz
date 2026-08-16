package leanval

import (
	"context"
	"strconv"

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
		Short: "Query lean_store.bonded_set(P)",
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
			qc := types.NewQueryClient(clientCtx)
			res, err := qc.BondedSet(context.Background(), &types.QueryBondedSetRequest{Period: p})
			if err != nil {
				return err
			}
			return clientCtx.PrintObjectLegacy(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
