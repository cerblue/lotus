package messagesigner

import (
	"context"
	"sync"
	"testing"

	"github.com/filecoin-project/lotus/chain/wallet"

	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/stretchr/testify/require"

	ds_sync "github.com/ipfs/go-datastore/sync"

	"github.com/filecoin-project/go-address"

	"github.com/filecoin-project/lotus/chain/types"
	"github.com/ipfs/go-datastore"
)

type mockMpool struct {
	lk     sync.RWMutex
	nonces map[address.Address]uint64
}

func newMockMpool() *mockMpool {
	return &mockMpool{nonces: make(map[address.Address]uint64)}
}

func (mp *mockMpool) setNonce(addr address.Address, nonce uint64) {
	mp.lk.Lock()
	defer mp.lk.Unlock()

	mp.nonces[addr] = nonce
}

func (mp *mockMpool) GetNonce(addr address.Address) (uint64, error) {
	mp.lk.RLock()
	defer mp.lk.RUnlock()

	return mp.nonces[addr], nil
}

func TestMessageSignerSignMessage(t *testing.T) {
	ctx := context.Background()

	w, _ := wallet.NewWallet(wallet.NewMemKeyStore())
	from1, err := w.GenerateKey(crypto.SigTypeSecp256k1)
	require.NoError(t, err)
	from2, err := w.GenerateKey(crypto.SigTypeSecp256k1)
	require.NoError(t, err)
	to1, err := w.GenerateKey(crypto.SigTypeSecp256k1)
	require.NoError(t, err)
	to2, err := w.GenerateKey(crypto.SigTypeSecp256k1)
	require.NoError(t, err)

	type msgSpec struct {
		msg        *types.Message
		mpoolNonce [1]uint64
		expNonce   uint64
	}
	tests := []struct {
		name string
		msgs []msgSpec
	}{{
		// No nonce yet in datastore
		name: "no nonce yet",
		msgs: []msgSpec{{
			msg: &types.Message{
				To:   to1,
				From: from1,
			},
			expNonce: 0,
		}},
	}, {
		// Get nonce value of zero from mpool
		name: "mpool nonce zero",
		msgs: []msgSpec{{
			msg: &types.Message{
				To:   to1,
				From: from1,
			},
			mpoolNonce: [1]uint64{0},
			expNonce:   0,
		}},
	}, {
		// Get non-zero nonce value from mpool
		name: "mpool nonce set",
		msgs: []msgSpec{{
			msg: &types.Message{
				To:   to1,
				From: from1,
			},
			mpoolNonce: [1]uint64{5},
			expNonce:   5,
		}, {
			msg: &types.Message{
				To:   to1,
				From: from1,
			},
			// Should ignore mpool nonce because after the first message nonce
			// will come from the datastore
			mpoolNonce: [1]uint64{10},
			expNonce:   6,
		}},
	}, {
		// Nonce should increment independently for each address
		name: "nonce increments per address",
		msgs: []msgSpec{{
			msg: &types.Message{
				To:   to1,
				From: from1,
			},
			expNonce: 0,
		}, {
			msg: &types.Message{
				To:   to1,
				From: from1,
			},
			expNonce: 1,
		}, {
			msg: &types.Message{
				To:   to2,
				From: from2,
			},
			mpoolNonce: [1]uint64{5},
			expNonce:   5,
		}, {
			msg: &types.Message{
				To:   to2,
				From: from2,
			},
			expNonce: 6,
		}, {
			msg: &types.Message{
				To:   to1,
				From: from1,
			},
			expNonce: 2,
		}},
	}}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			mpool := newMockMpool()
			ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
			ms := newMessageSigner(w, mpool, ds)

			for _, m := range tt.msgs {
				if len(m.mpoolNonce) == 1 {
					mpool.setNonce(m.msg.From, m.mpoolNonce[0])
				}
				smsg, err := ms.SignMessage(ctx, m.msg)
				require.NoError(t, err)
				require.Equal(t, m.expNonce, smsg.Message.Nonce)
			}
		})
	}
}
