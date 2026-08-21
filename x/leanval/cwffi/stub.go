//go:build !cgo

package cwffi

import "fmt"

type Config struct {
	PrivateKey    []byte
	Listen        string
	Bootstrappers string
	StorageDir    string
	Namespace     string
	Participants  []byte
	Weights       []uint64
	Epoch         uint64
	FloorPath     string
	FloorCert     []byte
}

func setEngineHeight(uint64) {}

func Start(*Driver, Config) error {
	return fmt.Errorf("lean-cw: cgo disabled")
}

func Stop() {}

func Running() bool { return false }

func EngineHeight() uint64 { return 0 }

func EngineEpoch() uint64 { return 0 }

func VerifyFinalization(pks []byte, weights []uint64, cert []byte) bool {
	return len(cert) > 0
}
