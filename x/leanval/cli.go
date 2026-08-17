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

	"github.com/terpnetwork/terp-core/v6/x/leanval/keeper"
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
	cmd.AddCommand(cmdBondedSet(), cmdFoldVerify())
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



func cmdFoldVerify() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fold-verify [proof-file] [a,b]...",
		Short: "ONE Stwo (prover_id=2 M31) same-statement fold verify; Dummy-N fails",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return keeper.FoldVerifyFile(args[0], args[1:])
		},
	}
	return cmd
}

func cliTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Committed Lean join/leave (not lean-pending.json)",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}
	cmd.AddCommand(cmdJoin(), cmdLeave())
	return cmd
}

func cmdJoin() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "join [hex-pubkey] [weight] [period]",
		Short: "Broadcast JOIN bytes; BondedSet is written on commit",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			subj, err := hex.DecodeString(args[0])
			if err != nil {
				return err
			}
			w, err := strconv.ParseInt(args[1], 10, 64)
			if err != nil {
				return err
			}
			per, err := strconv.ParseUint(args[2], 10, 64)
			if err != nil {
				return err
			}
			bz := types.EncodeJoin(types.JoinBlob{Period: per, Subject: subj, Weight: w})
			if clientCtx.Offline {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), hex.EncodeToString(bz))
				return err
			}
			if clientCtx.Client == nil {
				return fmt.Errorf("no RPC client")
			}
			res, err := clientCtx.Client.BroadcastTxSync(cmd.Context(), bz)
			if err != nil {
				return err
			}
			out, err := json.MarshalIndent(res, "", "  ")
			if err != nil {
				return err
			}
			return clientCtx.PrintBytes(out)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

func cmdLeave() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "leave [hex-pubkey] [period]",
		Short: "Broadcast LEAV bytes; BondedSet drops the subject on commit",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			subj, err := hex.DecodeString(args[0])
			if err != nil {
				return err
			}
			per, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return err
			}
			bz := types.EncodeLeave(types.LeaveBlob{Period: per, Subject: subj})
			if clientCtx.Offline {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), hex.EncodeToString(bz))
				return err
			}
			if clientCtx.Client == nil {
				return fmt.Errorf("no RPC client")
			}
			res, err := clientCtx.Client.BroadcastTxSync(cmd.Context(), bz)
			if err != nil {
				return err
			}
			out, err := json.MarshalIndent(res, "", "  ")
			if err != nil {
				return err
			}
			return clientCtx.PrintBytes(out)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
