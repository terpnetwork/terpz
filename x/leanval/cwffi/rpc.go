package cwffi

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ServeRPC is a thin Comet-shaped JSON-RPC so ict-rs can still read height on 26657.
func ServeRPC(listen string, d *Driver) error {
	addr := strings.TrimPrefix(listen, "tcp://")
	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, statusResult(d))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status" {
			writeJSON(w, statusResult(d))
			return
		}
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/broadcast_tx_sync") {
			handleBroadcastQuery(w, r, d)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		var req rpcReq
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, rpcResp{JSONRPC: "2.0", ID: req.ID, Error: &rpcErr{Code: -32700, Message: err.Error()}})
			return
		}
		switch req.Method {
		case "status":
			writeJSON(w, rpcResp{JSONRPC: "2.0", ID: req.ID, Result: statusResult(d)})
		case "broadcast_tx_sync", "broadcast_tx_commit", "broadcast_tx_async":
			tx, err := parseTxParam(req.Params)
			if err != nil {
				writeJSON(w, rpcResp{JSONRPC: "2.0", ID: req.ID, Error: &rpcErr{Code: -32602, Message: err.Error()}})
				return
			}
			writeJSON(w, rpcResp{JSONRPC: "2.0", ID: req.ID, Result: broadcast(d, tx)})
		default:
			writeJSON(w, rpcResp{JSONRPC: "2.0", ID: req.ID, Error: &rpcErr{Code: -32601, Message: "unknown method"}})
		}
	})
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	return nil
}

type rpcReq struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcResp struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcErr         `json:"error,omitempty"`
}

type rpcErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func statusResult(d *Driver) map[string]any {
	h := int64(0)
	if d != nil {
		h = d.Height()
	}
	hs := strconv.FormatInt(h, 10)
	return map[string]any{
		"node_info": map[string]any{
			"network": "lean",
			"moniker": "terpz",
		},
		"sync_info": map[string]any{
			"latest_block_height": hs,
			"latest_block_time":   time.Now().UTC().Format(time.RFC3339Nano),
			"catching_up":         false,
		},
		"validator_info": map[string]any{},
	}
}

func handleBroadcastQuery(w http.ResponseWriter, r *http.Request, d *Driver) {
	q := r.URL.Query().Get("tx")
	raw, err := decodeTx(q)
	if err != nil {
		writeJSON(w, rpcResp{JSONRPC: "2.0", ID: json.RawMessage("1"), Error: &rpcErr{Code: -32602, Message: err.Error()}})
		return
	}
	writeJSON(w, rpcResp{JSONRPC: "2.0", ID: json.RawMessage("1"), Result: broadcast(d, raw)})
}

func parseTxParam(params json.RawMessage) ([]byte, error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("missing tx")
	}
	var obj struct {
		Tx string `json:"tx"`
	}
	if err := json.Unmarshal(params, &obj); err == nil && obj.Tx != "" {
		return decodeTx(obj.Tx)
	}
	var arr []string
	if err := json.Unmarshal(params, &arr); err == nil && len(arr) > 0 {
		return decodeTx(arr[0])
	}
	return nil, fmt.Errorf("missing tx")
}

func decodeTx(s string) ([]byte, error) {
	s = strings.TrimPrefix(s, "0x")
	if b, err := hex.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.StdEncoding.DecodeString(s)
}

func broadcast(d *Driver, tx []byte) map[string]any {
	code := uint32(0)
	log := ""
	hash := sha256Hex(tx)
	if d != nil {
		resp, err := d.CheckTx(tx)
		if err != nil {
			code = 1
			log = err.Error()
		} else if resp != nil {
			code = resp.Code
			log = resp.Log
		}
	}
	return map[string]any{
		"code":      code,
		"data":      "",
		"log":       log,
		"hash":      hash,
		"codespace": "",
	}
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
