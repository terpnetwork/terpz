package cwffi

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

// Participants returns concatenated 32-byte ed25519 pubkeys for BondedSet
// bits=1 (membership SoT). Sorted so every node agrees on simplex indices.
func (d *Driver) Participants(epoch uint64) []byte {
	d.mu.Lock()
	q := d.storeQuery
	d.mu.Unlock()
	if q == nil {
		return nil
	}
	return BondedParticipants(q, epoch)
}

// BondedParticipants reads leanval store subspace via ABCI Query.
func BondedParticipants(q StoreQuery, epoch uint64) []byte {
	if q == nil {
		return nil
	}
	path := fmt.Sprintf("/store/%s/subspace", types.StoreKey)
	pks := pubkeysFromBits(q(path, []byte{types.BitfieldPrefix}), q(path, []byte{types.DepositIndexPrefix}))
	if len(pks) == 0 {
		pks = pubkeysFromBonded(q(path, types.BondedPrefixForPeriod(epoch)), epoch)
	}
	if len(pks) == 0 {
		return nil
	}
	sort.Slice(pks, func(i, j int) bool { return bytes.Compare(pks[i], pks[j]) < 0 })
	out := make([]byte, 0, len(pks)*32)
	for _, pk := range pks {
		out = append(out, pk...)
	}
	return out
}

func pubkeysFromBits(bfRaw, idxRaw []byte) [][]byte {
	bfPairs, err := types.DecodeSubspacePairs(bfRaw)
	if err != nil {
		return nil
	}
	var bf []byte
	for _, p := range bfPairs {
		if len(p.Key) == 1 && p.Key[0] == types.BitfieldPrefix {
			bf = p.Value
		}
	}
	if len(bf) == 0 {
		return nil
	}
	idxPairs, err := types.DecodeSubspacePairs(idxRaw)
	if err != nil {
		return nil
	}
	var out [][]byte
	for _, p := range idxPairs {
		if len(p.Key) < 2 || p.Key[0] != types.DepositIndexPrefix {
			continue
		}
		subj := p.Key[1:]
		if len(subj) != 32 {
			continue
		}
		if len(p.Value) < 5 {
			continue
		}
		idx := types.GetU32(p.Value[len(p.Value)-4:])
		if !bitGet(bf, idx) {
			continue
		}
		out = append(out, append([]byte(nil), subj...))
	}
	return out
}

func pubkeysFromBonded(raw []byte, epoch uint64) [][]byte {
	rows, err := types.DecodeBondedSubspace(epoch, raw)
	if err != nil {
		return nil
	}
	var out [][]byte
	for _, r := range rows {
		if len(r.Subject) != 32 {
			continue
		}
		if !r.HasProof {
			continue
		}
		out = append(out, append([]byte(nil), r.Subject...))
	}
	return out
}

func bitGet(bf []byte, idx uint32) bool {
	i := int(idx / 8)
	if i >= len(bf) {
		return false
	}
	return bf[i]&(1<<uint(idx%8)) != 0
}

// BondedParticipantsWeights returns sorted 32-byte pubkeys and parallel EB weights.
func BondedParticipantsWeights(q StoreQuery, epoch uint64) ([]byte, []uint64) {
	pks := BondedParticipants(q, epoch)
	if len(pks) == 0 {
		return nil, nil
	}
	n := len(pks) / 32
	weights := make([]uint64, n)
	for i := 0; i < n; i++ {
		weights[i] = 1
	}
	if q == nil {
		return pks, weights
	}
	path := fmt.Sprintf("/store/%s/subspace", types.StoreKey)
	idxRaw := q(path, []byte{types.DepositIndexPrefix})
	ebRaw := q(path, []byte{types.EBPrefix})
	idxPairs, _ := types.DecodeSubspacePairs(idxRaw)
	ebPairs, _ := types.DecodeSubspacePairs(ebRaw)
	ebByIdx := map[uint32]uint64{}
	for _, p := range ebPairs {
		if len(p.Key) != 5 || p.Key[0] != types.EBPrefix || len(p.Value) < 1 {
			continue
		}
		idx := types.GetU32(p.Key[1:])
		ebByIdx[idx] = uint64(p.Value[0])
	}
	subjIdx := map[string]uint32{}
	for _, p := range idxPairs {
		if len(p.Key) < 2 || p.Key[0] != types.DepositIndexPrefix {
			continue
		}
		subj := p.Key[1:]
		if len(subj) != 32 || len(p.Value) < 5 {
			continue
		}
		subjIdx[string(subj)] = types.GetU32(p.Value[len(p.Value)-4:])
	}
	for i := 0; i < n; i++ {
		subj := pks[i*32 : (i+1)*32]
		if idx, ok := subjIdx[string(subj)]; ok {
			if w, ok := ebByIdx[idx]; ok && w > 0 {
				weights[i] = w
			}
		}
	}
	return pks, weights
}

func (d *Driver) ParticipantsWeights(epoch uint64) ([]byte, []uint64) {
	d.mu.Lock()
	q := d.storeQuery
	d.mu.Unlock()
	if q == nil {
		return nil, nil
	}
	return BondedParticipantsWeights(q, epoch)
}
