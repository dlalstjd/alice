// Copyright © 2020 AMIS Technologies
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package signer

import (
	fmt "fmt"
	"math/big"

	"github.com/getamis/alice/crypto/birkhoffinterpolation"
	pt "github.com/getamis/alice/crypto/ecpointgrouplaw"
	"github.com/getamis/alice/crypto/homo"
	"github.com/getamis/alice/crypto/tss"
	"github.com/getamis/alice/crypto/tss/message"
	"github.com/getamis/alice/crypto/tss/message/types"
	"github.com/getamis/sirius/log"
)

type Result struct {
	R *big.Int
	S *big.Int
}

type Signer struct {
	ph *pubkeyHandler
	*message.MsgMain
}

func NewSigner(peerManager types.PeerManager, expectedPubkey *pt.ECPoint, homo homo.Crypto, secret *big.Int, bks map[string]*birkhoffinterpolation.BkParameter, msg []byte, listener types.StateChangedListener) (*Signer, error) {
	numPeers := peerManager.NumPeers()

	//u0 := big.NewInt(0)
	//fmt.Printf("secret: %d\n", secret)

	ph, err := newPubkeyHandler(expectedPubkey, peerManager, homo, secret, bks, msg)
	if err != nil {
		log.Warn("Failed to new a public key handler", "err", err)
		return nil, err
	}
	return &Signer{
		ph: ph,
		MsgMain: message.NewMsgMain(peerManager.SelfID(),
			numPeers,
			listener,
			ph,
			types.MessageType(Type_Pubkey),
			types.MessageType(Type_EncK),
			types.MessageType(Type_Mta),
			types.MessageType(Type_Delta),
			types.MessageType(Type_ProofAi),
			types.MessageType(Type_CommitViAi),
			types.MessageType(Type_DecommitViAi),
			types.MessageType(Type_CommitUiTi),
			types.MessageType(Type_DecommitUiTi),
			types.MessageType(Type_Si),
		),
	}, nil
}

func (s *Signer) GetPubkeyMessage() *Message {
	return s.ph.GetPubkeyMessage()
}

// GetResult returns the final result: public key, share, bks (including self bk)
func (s *Signer) GetResult() (*Result, error) {
	if s.GetState() != types.StateDone {
		return nil, tss.ErrNotReady
	}

	h := s.GetHandler()
	rh, ok := h.(*siHandler)
	if !ok {
		log.Error("We cannot convert to result handler in done state")
		return nil, tss.ErrNotReady
	}

	// This is copied from:
	// https://github.com/btcsuite/btcd/blob/c26ffa870fd817666a857af1bf6498fabba1ffe3/btcec/signature.go#L442-L444
	// This is needed because of tendermint checks here:
	// https://github.com/tendermint/tendermint/blob/d9481e3648450cb99e15c6a070c1fb69aa0c255b/crypto/secp256k1/secp256k1_nocgo.go#L43-L47
	sumS := new(big.Int).Set(rh.s)

	curve := s.ph.publicKey.GetCurve()
	secp256k1halfN := new(big.Int).Rsh(curve.Params().N, 1)
	if sumS.Cmp(secp256k1halfN) > 0 {
		sumS.Sub(curve.Params().N, sumS)
	}

	fmt.Printf("r: %d s: %d\n", rh.r.GetX(), rh.s)
	return &Result{
		R: new(big.Int).Set(rh.r.GetX()),
		S: new(big.Int).Set(rh.s),
	}, nil
}
