package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtcfg "github.com/cometbft/cometbft/config"
	pvm "github.com/cometbft/cometbft/privval"
	cmttypes "github.com/cometbft/cometbft/types"
	terpserver "github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/server/types"
	"golang.org/x/sync/errgroup"

	"github.com/terpnetwork/terp-core/v6/x/leanval/cwffi"
)

func useCommonware() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("LEAN_CONSENSUS")))
	return v == "commonware" || v == "cw" || v == "simplex"
}

func startCommonware(
	ctx context.Context,
	cfg *cmtcfg.Config,
	app types.Application,
	svrCtx *terpserver.Context,
	g *errgroup.Group,
) error {
	if err := maybeInitChain(app, cfg); err != nil {
		return err
	}

	pv := pvm.LoadOrGenFilePV(cfg.PrivValidatorKeyFile(), cfg.PrivValidatorStateFile())
	priv := pv.Key.PrivKey.Bytes()
	if len(priv) < 32 {
		return fmt.Errorf("lean-cw: privkey short")
	}
	seed := append([]byte(nil), priv[:32]...)

	doc, err := getGenDocProvider(cfg)()
	if err != nil {
		return err
	}

	var participants []byte
	for _, v := range doc.Validators {
		b := v.PubKey.Bytes()
		if len(b) != 32 {
			continue
		}
		participants = append(participants, b...)
	}
	if len(participants) == 0 {
		if pk, err := pv.GetPubKey(); err == nil {
			participants = append(participants, pk.Bytes()...)
		}
	}

	listen := os.Getenv("LEAN_CW_LISTEN")
	if listen == "" {
		listen = strings.TrimPrefix(cfg.P2P.ListenAddress, "tcp://")
	}
	boot := os.Getenv("LEAN_CW_BOOTSTRAPPERS")
	storage := filepath.Join(cfg.RootDir, "lean-cw")
	if err := os.MkdirAll(storage, 0o755); err != nil {
		return err
	}

	drv := cwffi.NewDriver(app, doc.ChainID, []byte(pv.GetAddress()))
	svrCtx.Logger.Info("starting Commonware simplex (Comet consensus disabled)", "listen", listen)
	if err := cwffi.Start(drv, cwffi.Config{
		PrivateKey:    seed,
		Listen:        listen,
		Bootstrappers: boot,
		StorageDir:    storage,
		Namespace:     "lean-terpz",
		Participants:  participants,
	}); err != nil {
		return err
	}
	if err := cwffi.ServeRPC(cfg.RPC.ListenAddress, drv); err != nil {
		cwffi.Stop()
		return err
	}
	g.Go(func() error {
		<-ctx.Done()
		cwffi.Stop()
		return nil
	})
	return nil
}

func maybeInitChain(app types.Application, cfg *cmtcfg.Config) error {
	info, err := app.Info(&abci.RequestInfo{})
	if err != nil {
		return err
	}
	if info.LastBlockHeight > 0 {
		return nil
	}
	doc, err := getGenDocProvider(cfg)()
	if err != nil {
		return err
	}
	vals := make([]abci.ValidatorUpdate, 0, len(doc.Validators))
	for _, v := range doc.Validators {
		vals = append(vals, cmttypes.TM2PB.NewValidatorUpdate(v.PubKey, v.Power))
	}
	cp := doc.ConsensusParams.ToProto()
	initial := doc.InitialHeight
	if initial == 0 {
		initial = 1
	}
	_, err = app.InitChain(&abci.RequestInitChain{
		Time:            doc.GenesisTime,
		ChainId:         doc.ChainID,
		ConsensusParams: &cp,
		Validators:      vals,
		AppStateBytes:   doc.AppState,
		InitialHeight:   initial,
	})
	return err
}
