package leanval

import (
	"context"
	"encoding/json"

	"cosmossdk.io/core/appmodule"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"

	"github.com/terpnetwork/terp-core/v6/x/leanval/keeper"
	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

var (
	_ module.AppModuleBasic   = AppModuleBasic{}
	_ module.AppModule        = AppModule{}
	_ module.HasGenesis       = AppModule{}
	_ appmodule.AppModule     = AppModule{}
	_ appmodule.HasEndBlocker = AppModule{}
)

type AppModuleBasic struct{}

func (AppModuleBasic) Name() string { return types.ModuleName }
func (AppModuleBasic) RegisterLegacyAminoCodec(*codec.LegacyAmino) {
}
func (AppModuleBasic) RegisterInterfaces(codectypes.InterfaceRegistry) {
}
func (AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	bz, _ := json.Marshal(types.DefaultGenesis())
	return bz
}
func (AppModuleBasic) ValidateGenesis(_ codec.JSONCodec, _ client.TxEncodingConfig, bz json.RawMessage) error {
	if len(bz) == 0 {
		return nil
	}
	var gs types.GenesisState
	return json.Unmarshal(bz, &gs)
}
func (AppModuleBasic) RegisterGRPCGatewayRoutes(client.Context, *runtime.ServeMux) {}
func (AppModuleBasic) GetTxCmd() *cobra.Command                                    { return nil }

// GetQueryCmd: terpz query leanval bonded-set [period]
func (AppModuleBasic) GetQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Querying commands for the leanval module",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
	}
	cmd.AddCommand(keeper.QueryBondedSetCLI(nil))
	return cmd
}

type AppModule struct {
	AppModuleBasic
	k *keeper.Keeper
}

func NewAppModule(k *keeper.Keeper) AppModule { return AppModule{k: k} }
func (am AppModule) IsOnePerModuleType()      {}
func (am AppModule) IsAppModule()             {}
func (AppModule) ConsensusVersion() uint64    { return 1 }

func (am AppModule) RegisterServices(module.Configurator) {}
func (am AppModule) InitGenesis(ctx sdk.Context, _ codec.JSONCodec, bz json.RawMessage) {
	if am.k != nil {
		am.k.BindContext(ctx)
	}
	var gs types.GenesisState
	if len(bz) > 0 {
		_ = json.Unmarshal(bz, &gs)
	}
	if am.k != nil {
		am.k.InitGenesis(gs)
	}
}
func (am AppModule) ExportGenesis(ctx sdk.Context, _ codec.JSONCodec) json.RawMessage {
	if am.k != nil {
		am.k.BindContext(ctx)
	}
	bz, _ := json.Marshal(am.k.ExportGenesis())
	return bz
}
func (am AppModule) EndBlock(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if am.k != nil {
		am.k.BindContext(sdkCtx)
		am.k.SetEndPeriod(types.PeriodFromHeight(sdkCtx.BlockHeight()))
	}
	return nil
}
