package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtcfg "github.com/cometbft/cometbft/config"
	pvm "github.com/cometbft/cometbft/privval"
	cmttypes "github.com/cometbft/cometbft/types"
	terpserver "github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/server/types"
	"golang.org/x/sync/errgroup"

	"github.com/terpnetwork/terp-core/v6/x/leanval/cwffi"
	leanvaltypes "github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

func useCommonware() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("LEAN_CONSENSUS")))
	return v == "commonware" || v == "cw" || v == "simplex"
}

func skipCommonwareEngine(pv *pvm.FilePV, participants []byte) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("LEAN_CW_SKIP")))
	if v == "1" || v == "true" || v == "yes" {
		return true
	}
	pk, err := pv.GetPubKey()
	if err != nil {
		return true
	}
	raw := pk.Bytes()
	if len(raw) != 32 || len(participants) < 32 {
		return true
	}
	for i := 0; i+32 <= len(participants); i += 32 {
		if bytes.Equal(raw, participants[i:i+32]) {
			return false
		}
	}
	return true
}

type cwEngine struct {
	mu      sync.Mutex
	pv      *pvm.FilePV
	seed    []byte
	listen  string
	boot    string
	storage string
	drv     *cwffi.Driver
	app     types.Application
	log     interface{ Info(string, ...interface{}) }
	epoch   uint64
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

	var genesisPks []byte
	for _, v := range doc.Validators {
		b := v.PubKey.Bytes()
		if len(b) != 32 {
			continue
		}
		genesisPks = append(genesisPks, b...)
	}
	if len(genesisPks) == 0 {
		if pk, err := pv.GetPubKey(); err == nil {
			genesisPks = append(genesisPks, pk.Bytes()...)
		}
	}

	listen := os.Getenv("LEAN_CW_LISTEN")
	if listen == "" {
		listen = strings.TrimPrefix(cfg.P2P.ListenAddress, "tcp://")
	}
	if pub := strings.TrimSpace(os.Getenv("LEAN_CW_PUBLIC")); pub != "" {
		listen = listen + "|" + pub
	}
	boot := os.Getenv("LEAN_CW_BOOTSTRAPPERS")
	storage := filepath.Join(cfg.RootDir, "lean-cw")
	if err := os.MkdirAll(storage, 0o755); err != nil {
		return err
	}

	drv := cwffi.NewDriver(app, doc.ChainID, []byte(pv.GetAddress()))
	drv.SetStoreQuery(func(path string, data []byte) []byte {
		type querier interface {
			Query(context.Context, *abci.RequestQuery) (*abci.ResponseQuery, error)
		}
		q, ok := app.(querier)
		if !ok {
			return nil
		}
		resp, err := q.Query(context.Background(), &abci.RequestQuery{Path: path, Data: data, Prove: false})
		if err != nil || resp == nil {
			return nil
		}
		return resp.Value
	})

	eng := &cwEngine{
		pv:      pv,
		seed:    seed,
		listen:  listen,
		boot:    boot,
		storage: storage,
		drv:     drv,
		app:     app,
		log:     svrCtx.Logger,
	}

	pks := drv.Participants(0)
	if len(pks) == 0 {
		pks = genesisPks
	}
	rotateCh := make(chan uint64, 4)
	drv.SetOnCommit(func(h int64) {
		cur := leanvaltypes.PeriodFromHeight(h)
		next := leanvaltypes.PeriodFromHeight(h + 1)
		if next != cur {
			select {
			case rotateCh <- next:
			default:
			}
		}
	})

	if skipCommonwareEngine(pv, pks) {
		svrCtx.Logger.Info("Commonware simplex skipped (not in BondedSet participants); catching up AppState")
		go catchupLoop(ctx, drv)
	} else {
		svrCtx.Logger.Info("starting Commonware simplex (Comet consensus disabled)", "listen", listen, "epoch", uint64(0))
		if err := eng.startEpoch(0, pks); err != nil {
			return err
		}
	}

	if err := cwffi.ServeRPC(cfg.RPC.ListenAddress, drv); err != nil {
		cwffi.Stop()
		return err
	}

	g.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				cwffi.Stop()
				return nil
			case ep := <-rotateCh:
				if err := eng.rotate(ep); err != nil {
					svrCtx.Logger.Info("lean-cw epoch rotate", "epoch", ep, "err", err.Error())
				}
			}
		}
	})
	return nil
}

func (e *cwEngine) startEpoch(epoch uint64, pks []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if cwffi.Running() {
		cwffi.Stop()
	}
	if len(pks) == 0 {
		pks = e.drv.Participants(epoch)
	}
	if skipCommonwareEngine(e.pv, pks) {
		return fmt.Errorf("local pubkey not in BondedSet for epoch %d", epoch)
	}
	dir := filepath.Join(e.storage, fmt.Sprintf("epoch-%d", epoch))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	weights := make([]uint64, len(pks)/32)
	if wpks, w, ok := func() ([]byte, []uint64, bool) {
		a, b := e.drv.ParticipantsWeights(epoch)
		return a, b, len(a) == len(pks)
	}(); ok {
		_ = wpks
		weights = w
	} else {
		for i := range weights {
			weights[i] = 1
		}
	}
	floorPath := filepath.Join(e.storage, "last-finalization")
	floorCert := e.drv.LastCertificateRaw()
	if len(floorCert) == 0 {
		if b, err := os.ReadFile(floorPath); err == nil {
			floorCert = b
		}
	}
	var last error
	for i := 0; i < 15; i++ {
		last = cwffi.Start(e.drv, cwffi.Config{
			PrivateKey:    e.seed,
			Listen:        e.listen,
			Bootstrappers: e.boot,
			StorageDir:    dir,
			Namespace:     "lean-terpz",
			Participants:  pks,
			Weights:       weights,
			Epoch:         epoch,
			FloorPath:     floorPath,
			FloorCert:     floorCert,
		})
		if last == nil {
			e.epoch = epoch
			e.log.Info("Commonware simplex engine started", "epoch", epoch, "n", len(pks)/32)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
		if cwffi.Running() {
			cwffi.Stop()
		}
	}
	return last
}

func (e *cwEngine) rotate(epoch uint64) error {
	e.mu.Lock()
	cur := e.epoch
	e.mu.Unlock()
	if epoch <= cur && cwffi.Running() {
		return nil
	}
	pks := e.drv.Participants(epoch)
	if skipCommonwareEngine(e.pv, pks) {
		e.log.Info("epoch boundary: still not in BondedSet", "epoch", epoch)
		return nil
	}
	e.log.Info("epoch boundary: (re)starting simplex", "epoch", epoch, "n", len(pks)/32)
	return e.startEpoch(epoch, pks)
}

func catchupURL() string {
	if u := strings.TrimSpace(os.Getenv("LEAN_CW_CATCHUP")); u != "" {
		return strings.TrimRight(u, "/")
	}
	boot := os.Getenv("LEAN_CW_BOOTSTRAPPERS")
	for _, part := range strings.Split(boot, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		hostport := part
		if i := strings.IndexByte(part, '@'); i >= 0 {
			hostport = part[i+1:]
		}
		host := hostport
		if i := strings.LastIndexByte(hostport, ':'); i >= 0 {
			host = hostport[:i]
		}
		if host != "" {
			return "http://" + host + ":26657"
		}
	}
	return ""
}

func catchupLoop(ctx context.Context, drv *cwffi.Driver) {
	base := catchupURL()
	if base == "" {
		return
	}
	client := &http.Client{Timeout: 3 * time.Second}
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if cwffi.Running() {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		next := drv.Height() + 1
		if next < 1 {
			next = 1
		}
		payload, cert := fetchPayloadAndCert(client, base, next)
		if len(payload) > 0 && len(cert) > 0 {
			drv.ApplyRemote(payload, cert)
			// Period boundary: give the rotator time to Start if we are in BondedSet.
			h := drv.Height()
			if leanvaltypes.PeriodFromHeight(h+1) != leanvaltypes.PeriodFromHeight(h) {
				time.Sleep(time.Second)
				continue
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func fetchPayloadAndCert(client *http.Client, base string, height int64) (payload, cert []byte) {
	urls := []string{
		fmt.Sprintf("%s/lean/certificate?height=%d", base, height),
		fmt.Sprintf("%s/payload?height=%d", base, height),
		fmt.Sprintf("%s/block?height=%d", base, height),
	}
	for _, u := range urls {
		resp, err := client.Get(u)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		_ = resp.Body.Close()
		p, c := decodeCatchupPayload(body, height)
		if len(p) > 0 {
			payload = p
		}
		if len(c) > 0 {
			cert = c
		}
		if len(payload) > 0 && len(cert) > 0 {
			return payload, cert
		}
	}
	return payload, cert
}

func decodeCatchupPayload(body []byte, want int64) (payload, cert []byte) {
	var wrap struct {
		Result  json.RawMessage `json:"result"`
		Height  json.RawMessage `json:"height"`
		Payload string          `json:"payload"`
		Block   json.RawMessage `json:"block"`
	}
	if json.Unmarshal(body, &wrap) != nil {
		return nil, nil
	}
	raw := wrap.Result
	if len(raw) == 0 {
		raw = body
	}
	var res struct {
		Payload     string `json:"payload"`
		Certificate string `json:"certificate"`
		Height      string `json:"height"`
		Block       struct {
			Header struct {
				Height string `json:"height"`
			} `json:"header"`
			Data struct {
				Txs []string `json:"txs"`
			} `json:"data"`
		} `json:"block"`
	}
	if json.Unmarshal(raw, &res) != nil {
		return nil, nil
	}
	hs := res.Height
	if hs == "" {
		hs = res.Block.Header.Height
	}
	if hs != "" {
		var h int64
		fmt.Sscanf(hs, "%d", &h)
		if h != 0 && h != want {
			return nil, nil
		}
	}
	b64 := res.Payload
	if b64 == "" && len(res.Block.Data.Txs) > 0 {
		b64 = res.Block.Data.Txs[0]
	}
	if b64 != "" {
		p, err := base64.StdEncoding.DecodeString(b64)
		if err == nil {
			if _, err := cwffi.DecodePayload(p); err == nil {
				payload = p
			}
		}
	}
	if res.Certificate != "" {
		if c, err := base64.StdEncoding.DecodeString(res.Certificate); err == nil {
			cert = c
		}
	}
	return payload, cert
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
