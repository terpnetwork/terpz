//go:build cgo

package cwffi

/*
#cgo CFLAGS: -I${SRCDIR}/../../../crates/lean-cw-ffi/include
#cgo LDFLAGS: -llean_cw_ffi -lpthread -ldl -lm
#cgo darwin LDFLAGS: -framework Security -framework SystemConfiguration
#cgo linux LDFLAGS: -lstdc++
#include "lean_cw.h"
#include <stdlib.h>
#include <string.h>

void lean_cw_fill_callbacks(lean_cw_callbacks *c, void *user);
*/
import "C"

import (
	"fmt"
	"sync"
	"unsafe"
)

var engineMu sync.Mutex
var engineDrv *Driver

func setEngineHeight(h uint64) {
	C.lean_cw_set_height(C.uint64_t(h))
}

// Config starts the in-process simplex engine.
type Config struct {
	PrivateKey    []byte
	Listen        string
	Bootstrappers string
	StorageDir    string
	Namespace     string
	Participants  []byte // concatenated 32-byte ed25519 pubkeys
	Weights       []uint64
	Epoch         uint64
	FloorPath     string
	FloorCert     []byte
}

func Start(drv *Driver, cfg Config) error {
	if drv == nil {
		return fmt.Errorf("lean-cw: nil driver")
	}
	if len(cfg.PrivateKey) < 32 {
		return fmt.Errorf("lean-cw: private key must be 32 bytes")
	}
	engineMu.Lock()
	engineDrv = drv
	engineMu.Unlock()
	drv.SetVerifyCert(VerifyFinalization)

	var ccfg C.lean_cw_cfg
	for i := 0; i < 32; i++ {
		ccfg.private_key[i] = C.uint8_t(cfg.PrivateKey[i])
	}
	listen := C.CString(cfg.Listen)
	boots := C.CString(cfg.Bootstrappers)
	stor := C.CString(cfg.StorageDir)
	ns := C.CString(cfg.Namespace)
	defer C.free(unsafe.Pointer(listen))
	defer C.free(unsafe.Pointer(boots))
	defer C.free(unsafe.Pointer(stor))
	defer C.free(unsafe.Pointer(ns))
	ccfg.listen = listen
	ccfg.bootstrappers = boots
	ccfg.storage_dir = stor
	ccfg.namespace = ns
	if len(cfg.Participants) > 0 {
		ccfg.participants = (*C.uint8_t)(C.CBytes(cfg.Participants))
		ccfg.participants_len = C.size_t(len(cfg.Participants))
		defer C.free(unsafe.Pointer(ccfg.participants))
	}
	if len(cfg.Weights) > 0 {
		ccfg.weights = (*C.uint64_t)(C.CBytes(uint64Bytes(cfg.Weights)))
		ccfg.weights_len = C.size_t(len(cfg.Weights))
		defer C.free(unsafe.Pointer(ccfg.weights))
	}
	ccfg.epoch = C.uint64_t(cfg.Epoch)
	fp := C.CString(cfg.FloorPath)
	defer C.free(unsafe.Pointer(fp))
	ccfg.floor_path = fp
	if len(cfg.FloorCert) > 0 {
		ccfg.floor_cert = (*C.uint8_t)(C.CBytes(cfg.FloorCert))
		ccfg.floor_cert_len = C.size_t(len(cfg.FloorCert))
		defer C.free(unsafe.Pointer(ccfg.floor_cert))
	}

	var cb C.lean_cw_callbacks
	C.lean_cw_fill_callbacks(&cb, nil)
	if rc := C.lean_cw_start(&ccfg, &cb); rc != 0 {
		return fmt.Errorf("lean-cw: start failed rc=%d", int(rc))
	}
	return nil
}

func Stop() {
	C.lean_cw_stop()
}

func Running() bool {
	return C.lean_cw_running() != 0
}

func EngineHeight() uint64 {
	return uint64(C.lean_cw_height())
}

func EngineEpoch() uint64 {
	return uint64(C.lean_cw_epoch())
}

func currentDriver() *Driver {
	engineMu.Lock()
	defer engineMu.Unlock()
	return engineDrv
}

//export goLeanCwPropose
func goLeanCwPropose(user unsafe.Pointer, epoch, view C.uint64_t, parent, digest *C.uint8_t, payload **C.uint8_t, payloadLen *C.size_t) C.int {
	_ = user
	d := currentDriver()
	if d == nil {
		return -1
	}
	parentB := C.GoBytes(unsafe.Pointer(parent), 32)
	dgst, pay, err := d.Propose(uint64(epoch), uint64(view), parentB)
	if err != nil || len(dgst) != 32 {
		return -1
	}
	C.memcpy(unsafe.Pointer(digest), unsafe.Pointer(&dgst[0]), 32)
	*payload = (*C.uint8_t)(C.CBytes(pay))
	*payloadLen = C.size_t(len(pay))
	return 0
}

//export goLeanCwVerify
func goLeanCwVerify(user unsafe.Pointer, epoch, view C.uint64_t, digest *C.uint8_t, payload *C.uint8_t, payloadLen C.size_t) C.int {
	_ = user
	d := currentDriver()
	if d == nil {
		return 0
	}
	dgst := C.GoBytes(unsafe.Pointer(digest), 32)
	var pay []byte
	if payload != nil && payloadLen > 0 {
		pay = C.GoBytes(unsafe.Pointer(payload), C.int(payloadLen))
	}
	if d.Verify(uint64(epoch), uint64(view), dgst, pay) {
		return 1
	}
	return 0
}

//export goLeanCwCertify
func goLeanCwCertify(user unsafe.Pointer, epoch, view C.uint64_t, digest *C.uint8_t) C.int {
	_ = user
	d := currentDriver()
	if d == nil {
		return 0
	}
	dgst := C.GoBytes(unsafe.Pointer(digest), 32)
	if d.Certify(uint64(epoch), uint64(view), dgst) {
		return 1
	}
	return 0
}

//export goLeanCwReport
func goLeanCwReport(user unsafe.Pointer, kind C.uint32_t, epoch, view C.uint64_t, digest *C.uint8_t) {
	_ = user
	d := currentDriver()
	if d == nil {
		return
	}
	dgst := C.GoBytes(unsafe.Pointer(digest), 32)
	d.Report(uint32(kind), uint64(epoch), uint64(view), dgst)
}

//export goLeanCwFinalize
func goLeanCwFinalize(user unsafe.Pointer, epoch, view C.uint64_t, digest *C.uint8_t, payload *C.uint8_t, payloadLen C.size_t, certificate *C.uint8_t, certificateLen C.size_t) {
	_ = user
	d := currentDriver()
	if d == nil {
		return
	}
	dgst := C.GoBytes(unsafe.Pointer(digest), 32)
	var pay []byte
	if payload != nil && payloadLen > 0 {
		pay = C.GoBytes(unsafe.Pointer(payload), C.int(payloadLen))
	}
	var cert []byte
	if certificate != nil && certificateLen > 0 {
		cert = C.GoBytes(unsafe.Pointer(certificate), C.int(certificateLen))
	}
	d.Finalize(uint64(epoch), uint64(view), dgst, pay, cert)
}

func uint64Bytes(w []uint64) []byte {
	out := make([]byte, 8*len(w))
	for i, v := range w {
		out[i*8+0] = byte(v)
		out[i*8+1] = byte(v >> 8)
		out[i*8+2] = byte(v >> 16)
		out[i*8+3] = byte(v >> 24)
		out[i*8+4] = byte(v >> 32)
		out[i*8+5] = byte(v >> 40)
		out[i*8+6] = byte(v >> 48)
		out[i*8+7] = byte(v >> 56)
	}
	return out
}

// VerifyFinalization asks the Rust scheme adapter (weighted + ed25519).
func VerifyFinalization(pks []byte, weights []uint64, cert []byte) bool {
	if len(pks) == 0 || len(cert) == 0 {
		return false
	}
	var wptr *C.uint64_t
	if len(weights) > 0 {
		wb := uint64Bytes(weights)
		wptr = (*C.uint64_t)(C.CBytes(wb))
		defer C.free(unsafe.Pointer(wptr))
	}
	pb := C.CBytes(pks)
	defer C.free(pb)
	cb := C.CBytes(cert)
	defer C.free(cb)
	rc := C.lean_cw_verify_finalization(
		(*C.uint8_t)(pb), C.size_t(len(pks)),
		wptr, C.size_t(len(weights)),
		(*C.uint8_t)(cb), C.size_t(len(cert)),
	)
	return rc == 1
}

//export goLeanCwParticipants
func goLeanCwParticipants(user unsafe.Pointer, epoch C.uint64_t, pkOut **C.uint8_t, pkLen *C.size_t) C.int {
	_ = user
	d := currentDriver()
	if d == nil || pkOut == nil || pkLen == nil {
		return -1
	}
	pks := d.Participants(uint64(epoch))
	if len(pks) == 0 {
		*pkOut = nil
		*pkLen = 0
		return 0
	}
	*pkOut = (*C.uint8_t)(C.CBytes(pks))
	*pkLen = C.size_t(len(pks))
	return 0
}
