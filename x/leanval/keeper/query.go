package keeper

import (
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

// QueryBondedSetCLI is the cobra shape for `terpz query leanval bonded-set [period]`.
// Proto gen is not required; testers call Keeper.QueryBondedSet(period).
func QueryBondedSetCLI(k *Keeper) *cobra.Command {
	return &cobra.Command{
		Use:   "bonded-set [period]",
		Short: "Query lean_store.bonded_set(P)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return err
			}
			if k == nil {
				return fmt.Errorf("leanval keeper not bound (use QueryBondedSet on node)")
			}
			set := k.QueryBondedSet(p)
			for _, s := range set {
				fmt.Fprintf(cmd.OutOrStdout(), "%s %d has_proof=%v\n", hex.EncodeToString(s.Subject), s.Weight, s.HasProof)
			}
			return nil
		},
	}
}
