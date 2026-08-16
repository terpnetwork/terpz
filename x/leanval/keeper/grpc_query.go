package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

var _ types.QueryServer = Querier{}

type Querier struct{ K *Keeper }

func NewQuerier(k *Keeper) Querier { return Querier{K: k} }

func (q Querier) BondedSet(ctx context.Context, req *types.QueryBondedSetRequest) (*types.QueryBondedSetResponse, error) {
	if q.K == nil {
		return &types.QueryBondedSetResponse{}, nil
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	q.K.BindContext(sdkCtx)
	set := q.K.QueryBondedSet(req.GetPeriod())
	rows := make([]types.BondedSetRow, 0, len(set))
	for _, s := range set {
		rows = append(rows, types.BondedSetRow{
			Subject:  append([]byte(nil), s.Subject...),
			Weight:   s.Weight,
			HasProof: s.HasProof,
		})
	}
	return &types.QueryBondedSetResponse{Rows: rows}, nil
}
