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
package integration

import (
	"context"
	//"encoding/hex"
	"crypto/ecdsa"
	"crypto/elliptic"
	"fmt"
	"log"
	"math/big"
	"testing"
	"time"

    "github.com/ethereum/go-ethereum/common"
    core "github.com/ethereum/go-ethereum/core/types"
    "github.com/ethereum/go-ethereum/crypto"
    "github.com/ethereum/go-ethereum/ethclient"
    //"github.com/ethereum/go-ethereum/rlp"
	//"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/btcsuite/btcd/btcec"
	"github.com/getamis/alice/crypto/birkhoffinterpolation"
	"github.com/getamis/alice/crypto/ecpointgrouplaw"
	"github.com/getamis/alice/crypto/homo"
	"github.com/getamis/alice/crypto/homo/cl"
	"github.com/getamis/alice/crypto/homo/paillier"
	"github.com/getamis/alice/crypto/tss"
	"github.com/getamis/alice/crypto/tss/dkg"
	"github.com/getamis/alice/crypto/tss/message"
	"github.com/getamis/alice/crypto/tss/message/types"
	"github.com/getamis/alice/crypto/tss/message/types/mocks"
	"github.com/getamis/alice/crypto/tss/signer"
	"github.com/golang/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	"gonum.org/v1/gonum/stat/combin"
)

func TestTSS(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TSS Suite")
}

var _ = Describe("TSS", func() {
	DescribeTable("TSS flow", func(c elliptic.Curve, threshold uint32, ranks []uint32) {
		lens := len(ranks)
		listener := make([]*mocks.StateChangedListener, lens)

		// homo functions for signer
		homoFuncs := []func() (homo.Crypto, error){
			func() (homo.Crypto, error) {
				return paillier.NewPaillier(2048)
			},
			func() (homo.Crypto, error) {
				safeParameter := 1348
				distributionDistance := uint(40)
				return cl.NewCL(big.NewInt(1024), 40, c.Params().N, safeParameter, distributionDistance)
			},
		}

		By("Step 1: DKG")
		dkgs := make(map[string]*dkg.DKG, lens)
		msgMain := make(map[string]*message.MsgMain, lens)
		dkgPeerManagers := make([]types.PeerManager, lens)
		for i := 0; i < lens; i++ {
			id := getID(i)
			pm := newPeerManager(id, lens-1)
			pm.setMsgMains(msgMain)
			dkgPeerManagers[i] = pm
			listener[i] = new(mocks.StateChangedListener)
			listener[i].On("OnStateChanged", types.StateInit, types.StateDone).Once()
			var err error
			dkgs[id], err = dkg.NewDKG(c, dkgPeerManagers[i], threshold, ranks[i], listener[i])
			Expect(err).Should(BeNil())
			msgMain[id] = dkgs[id].MsgMain
			dkgResult, err := dkgs[id].GetResult()
			Expect(dkgResult).Should(BeNil())
			Expect(err).Should(Equal(tss.ErrNotReady))
			dkgs[id].Start()
		}

		// Send out peer message
		for fromID, fromD := range dkgs {
			msg := fromD.GetPeerMessage()
			for toID, toD := range dkgs {
				if fromID == toID {
					continue
				}
				Expect(toD.AddMessage(msg)).Should(BeNil())
			}
		}
		time.Sleep(1 * time.Second)

		secret := big.NewInt(0)
		fo := big.NewInt(0)
		// Stop DKG process and record the result.
		var r *result
		for id, dkg := range dkgs {
			dkg.Stop()
			// build private key for test
			secret = new(big.Int).Add(secret, dkg.GetU0())

			dkgResult, err := dkg.GetResult()
			Expect(err).Should(BeNil())
			if r == nil {
				r = &result{
					publicKey: dkgResult.PublicKey,
					bks:       dkgResult.Bks,
					share:     make(map[string]*big.Int),
				}
			} else {
				// public key and bks should be the same
				Expect(r.publicKey).Should(Equal(dkgResult.PublicKey))
				Expect(r.bks).Should(Equal(dkgResult.Bks))
			}
			r.share[id] = dkgResult.Share
			fo = new(big.Int).Set(dkg.GetFieldOrder())
		}
		secret = new(big.Int).Mod(secret, fo)
		fmt.Printf("reconstructed private key : %d\n", secret)
		assertListener(listener, lens)

		By("Step 2: Signer")
		for _, homoFunc := range homoFuncs {
			sign(homoFunc, int(threshold), lens, r, listener, secret)
		}
		/*
			By("Step 3: Reshare")
			reshares := make(map[string]*reshare.Reshare, lens)
			msgMain = make(map[string]*message.MsgMain, lens)
			resharePeerManagers := make([]types.PeerManager, lens)
			for i := 0; i < lens; i++ {
				id := getID(i)
				pm := newPeerManager(id, lens-1)
				pm.setMsgMains(msgMain)
				resharePeerManagers[i] = pm
				listener[i].On("OnStateChanged", types.StateInit, types.StateDone).Once()
				var err error
				reshares[id], err = reshare.NewReshare(resharePeerManagers[i], threshold, r.publicKey, r.share[id], r.bks, listener[i])
				Expect(err).Should(BeNil())
				msgMain[id] = reshares[id].MsgMain
				reshareResult, err := reshares[id].GetResult()
				Expect(reshareResult).Should(BeNil())
				Expect(err).Should(Equal(tss.ErrNotReady))
				reshares[id].Start()
			}

			// Send out commit message
			for fromID, fromD := range reshares {
				msg := fromD.GetCommitMessage()
				for toID, toD := range reshares {
					if fromID == toID {
						continue
					}
					Expect(toD.AddMessage(msg)).Should(BeNil())
				}
			}
			time.Sleep(1 * time.Second)

			// Stop Reshare process and update the share.
			for id, reshare := range reshares {
				reshare.Stop()
				reshareResult, err := reshare.GetResult()
				Expect(err).Should(BeNil())
				r.share[id] = reshareResult.Share
			}
			assertListener(listener, lens)

			By("Step 4: Signer again")
			for _, homoFunc := range homoFuncs {
				sign(homoFunc, int(threshold), lens, r, listener)
			}

			By("Step 5: Add new share")
			newPeerID := getID(lens)
			newPeerRank := uint32(0)

			var addShareForNew *newpeer.AddShare
			var addSharesForOld = make(map[string]*oldpeer.AddShare, lens)
			msgMain = make(map[string]*message.MsgMain, lens+1)

			pmNew := newPeerManager(newPeerID, lens)
			pmNew.setMsgMains(msgMain)
			listenerNew := new(mocks.StateChangedListener)
			listenerNew.On("OnStateChanged", types.StateInit, types.StateDone).Once()
			addShareForNew = newpeer.NewAddShare(pmNew, r.publicKey, threshold, newPeerRank, listenerNew)
			msgMain[newPeerID] = addShareForNew.MsgMain
			addShareNewResult, err := addShareForNew.GetResult()
			Expect(addShareNewResult).Should(BeNil())
			Expect(err).Should(Equal(tss.ErrNotReady))
			addShareForNew.Start()

			pmOlds := make([]types.PeerManager, lens)
			listenersOld := make([]*mocks.StateChangedListener, lens)
			for i := 0; i < lens; i++ {
				id := getID(i)
				pm := newPeerManager(id, lens-1)
				pm.setMsgMains(msgMain)
				pmOlds[i] = pm
				listenersOld[i] = new(mocks.StateChangedListener)
				listenersOld[i].On("OnStateChanged", types.StateInit, types.StateDone).Once()
				var err error
				addSharesForOld[id], err = oldpeer.NewAddShare(pmOlds[i], r.publicKey, threshold, r.share[id], r.bks, newPeerID, listenersOld[i])
				Expect(err).Should(BeNil())
				msgMain[id] = addSharesForOld[id].MsgMain
				addShareOldResult, err := addSharesForOld[id].GetResult()
				Expect(addShareOldResult).Should(BeNil())
				Expect(err).Should(Equal(tss.ErrNotReady))
				addSharesForOld[id].Start()
			}

			// Send out all old peer message to new peer
			for _, fromA := range addSharesForOld {
				msg := fromA.GetPeerMessage()
				Expect(addShareForNew.AddMessage(msg)).Should(BeNil())
			}
			time.Sleep(1 * time.Second)

			// Stop add share process and check the result.
			for id, addshare := range addSharesForOld {
				addshare.Stop()
				addshareResult, err := addshare.GetResult()
				Expect(err).Should(BeNil())
				Expect(r.publicKey).Should(Equal(addshareResult.PublicKey))
				Expect(r.share[id]).Should(Equal(addshareResult.Share))
				Expect(r.bks[id]).Should(Equal(addshareResult.Bks[id]))
			}
			addShareForNew.Stop()
			addshareResult, err := addShareForNew.GetResult()
			Expect(err).Should(BeNil())
			Expect(r.publicKey).Should(Equal(addshareResult.PublicKey))
			Expect(addshareResult.Share).ShouldNot(BeNil())
			Expect(addshareResult.Bks[newPeerID]).ShouldNot(BeNil())
			// Update the new peer into result
			r.share[newPeerID] = addshareResult.Share
			r.bks[newPeerID] = addshareResult.Bks[newPeerID]

			for i := 0; i < lens; i++ {
				listenersOld[i].AssertExpectations(GinkgoT())
			}
			listenerNew.AssertExpectations(GinkgoT())
			assertListener(listener, lens)

			By("Step 6: Signer again")
			lens++
			listener = make([]*mocks.StateChangedListener, lens)
			for _, homoFunc := range homoFuncs {
				sign(homoFunc, int(threshold), lens, r, listener)
			}*/
	},
		Entry("S256 curve, 3 of (0,0,0)", btcec.S256(), uint32(3), []uint32{0, 0, 0}),
		//Entry("S256 curve, 3 of (0,0,0,0,0)", btcec.S256(), uint32(3), []uint32{0, 0, 0, 0, 0}),
		//Entry("S256 curve, 3 of (0,0,0,1,1)", btcec.S256(), uint32(3), []uint32{0, 0, 0, 1, 1}),
		//Entry("S256 curve, 3 of (0,0,0)", btcec.S256(), uint32(3), []uint32{0, 0, 0}),
	)
})

func sign(homoFunc func() (homo.Crypto, error), threshold, num int, dkgResult *result, listener []*mocks.StateChangedListener, secret *big.Int) {
	combination := combin.Combinations(num, threshold)
	
	client, err := ethclient.Dial("https://ropsten.infura.io/v3/375a84d45ba0456a8d39a32cce31471c")
	if err != nil{
		log.Fatal(err)
	}
	
	nonce := uint64(0)
	value := big.NewInt(1000000000000000) // 0.001 eth in wei
	gasLimit := uint64(21000)
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("gas Price %d\n", gasPrice)
	toAddress := common.HexToAddress("0x743376fd2a693723A60942D0b4B2F1765ea1Dbb0")
	var data []byte
	tx := core.NewTransaction(nonce, toAddress, value, gasLimit, gasPrice, data)

	chainID, err := client.NetworkID(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	s := core.NewEIP155Signer(chainID)
	h := s.Hash(tx)

	hex_secret := fmt.Sprintf("%x", secret)

	privateKey, err := crypto.HexToECDSA(hex_secret)
	if err != nil {
		log.Fatal(err)
	}
	signedTx, err := core.SignTx(tx, s, privateKey)
	if err != nil{
		log.Fatal(err)
	}

	t_r := new(big.Int)
    t_s := new(big.Int)
    t_v := new(big.Int)
    t_v, t_r, t_s = signedTx.RawSignatureValues()
    fmt.Printf("--private key sign-- r: %d s: %d v: %d\n", t_r,t_s,t_v)

	msg := h[:]
	// Loop over all combinations.
	for _, c := range combination {
		signers := make(map[string]*signer.Signer, threshold)
		doneChs := make(map[string]chan struct{}, threshold)
		msgMain := make(map[string]*message.MsgMain, threshold)
		for _, i := range c {
			h, err := homoFunc()
			Expect(err).Should(BeNil())
			id := getID(i)
			pm := newPeerManager(id, threshold-1)
			pm.setMsgMains(msgMain)
			doneChs[id] = make(chan struct{})
			doneCh := doneChs[id]
			listener[i] = new(mocks.StateChangedListener)
			listener[i].On("OnStateChanged", types.StateInit, types.StateDone).Run(func(args mock.Arguments) {
				close(doneCh)
			}).Once()
			bks := make(map[string]*birkhoffinterpolation.BkParameter)
			bks[id] = dkgResult.bks[id]
			for _, j := range c {
				if i == j {
					continue
				}
				pID := getID(j)
				bks[pID] = dkgResult.bks[pID]
			}
			signers[id], err = signer.NewSigner(pm, dkgResult.publicKey, h, dkgResult.share[id], bks, msg, listener[i])
			Expect(err).Should(BeNil())
			msgMain[id] = signers[id].MsgMain
			signerResult, err := signers[id].GetResult()
			Expect(signerResult).Should(BeNil())
			Expect(err).Should(Equal(tss.ErrNotReady))
			signers[id].Start()
		}

		// Send out pubkey message.
		for fromID, fromD := range signers {
			msg := fromD.GetPubkeyMessage()
			for toID, toD := range signers {
				if fromID == toID {
					continue
				}
				Expect(toD.AddMessage(msg)).Should(BeNil())
			}
		}

		for _, i := range c {
			id := getID(i)
			<-doneChs[id]
		}

		// Stop signer process and verify the signature.
		var r, s *big.Int
		for _, signer := range signers {
			signer.Stop()
			signerResult, err := signer.GetResult()
			Expect(err).Should(BeNil())
			// All R and S should be the same.
			if r != nil {
				Expect(r).Should(Equal(signerResult.R))
				Expect(s).Should(Equal(signerResult.S))
			} else {
				r = signerResult.R
				s = signerResult.S
			}
		}
		// check r, s
		fmt.Printf("-------------- r: %d s: %d -------------\n", r, s)

		ecdsaPublicKey := &ecdsa.PublicKey{
			Curve: dkgResult.publicKey.GetCurve(),
			X:     dkgResult.publicKey.GetX(),
			Y:     dkgResult.publicKey.GetY(),
		}

		//check public key
		fmt.Printf("------------ X: %d Y: %d -----------\n", ecdsaPublicKey.X, ecdsaPublicKey.Y)
		Expect(ecdsa.Verify(ecdsaPublicKey, msg, r, s)).Should(BeTrue())
		assertListener(listener, threshold)

		Expect(ecdsa.Verify(ecdsaPublicKey, msg, t_r, t_s)).Should(BeTrue())
	}
}

func assertListener(listener []*mocks.StateChangedListener, lens int) {
	for i := 0; i < lens; i++ {
		listener[i].AssertExpectations(GinkgoT())
	}
}

type result struct {
	publicKey *ecpointgrouplaw.ECPoint
	bks       map[string]*birkhoffinterpolation.BkParameter
	share     map[string]*big.Int
}

func getID(id int) string {
	return fmt.Sprintf("id-%d", id)
}

type peerManager struct {
	id       string
	numPeers uint32
	msgMains map[string]*message.MsgMain
}

func newPeerManager(id string, numPeers int) *peerManager {
	return &peerManager{
		id:       id,
		numPeers: uint32(numPeers),
	}
}

func (p *peerManager) NumPeers() uint32 {
	return p.numPeers
}

func (p *peerManager) SelfID() string {
	return p.id
}

func (p *peerManager) MustSend(id string, message proto.Message) {
	msg := message.(types.Message)
	Expect(p.msgMains[id].AddMessage(msg)).Should(BeNil())
}

func (p *peerManager) setMsgMains(msgMains map[string]*message.MsgMain) {
	p.msgMains = msgMains
}
