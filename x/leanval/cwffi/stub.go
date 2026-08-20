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
}

func setEngineHeight(uint64) {}

func Start(*Driver, Config) error {
	return fmt.Errorf("lean-cw: cgo disabled")
}

func Stop() {}

func EngineHeight() uint64 { return 0 }
