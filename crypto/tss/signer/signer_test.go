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
	"crypto/ecdsa"
	"crypto/elliptic"
	"fmt"
	"math/big"
	"testing"

	"github.com/btcsuite/btcd/btcec"
	"github.com/getamis/alice/crypto/birkhoffinterpolation"
	"github.com/getamis/alice/crypto/ecpointgrouplaw"
	"github.com/getamis/alice/crypto/homo/paillier"
	"github.com/getamis/alice/crypto/tss"
	"github.com/getamis/alice/crypto/tss/message/types"
	"github.com/getamis/alice/crypto/tss/message/types/mocks"
	proto "github.com/golang/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
)

func TestSigner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Signer Suite")
}

var _ = Describe("Signer", func() {
	var (
		curve = btcec.S256()
		msg   = []byte{1, 2, 3}
	)

	DescribeTable("NewSigner()", func(ss [][]*big.Int, gScale *big.Int) {
		// new peer managers and dkgs
		expPublic := ecpointgrouplaw.ScalarBaseMult(curve, gScale)
		threshold := len(ss)
		signers, listeners := newSigners(curve, expPublic, ss, msg)
		doneChs := make([]chan struct{}, threshold)
		i := 0

		for _, s := range signers {
			r, err := s.GetResult()
			Expect(r).Should(BeNil())
			Expect(err).Should(Equal(tss.ErrNotReady))
		}

		for _, l := range listeners {
			doneChs[i] = make(chan struct{})
			doneCh := doneChs[i]
			l.On("OnStateChanged", types.StateInit, types.StateDone).Run(func(args mock.Arguments) {
				close(doneCh)
			}).Once()
			i++
		}

		// Send out pubkey message
		for fromID, fromD := range signers {
			msg := fromD.GetPubkeyMessage()
			for toID, toD := range signers {
				if fromID == toID {
					continue
				}
				Expect(toD.AddMessage(msg)).Should(BeNil())
			}
		}

		for i := 0; i < threshold; i++ {
			<-doneChs[i]
		}

		// Build public key
		var r, s *big.Int
		for _, signer := range signers {
			signer.Stop()
			result, err := signer.GetResult()
			Expect(err).Should(BeNil())
			// All R and S should be the same
			if r != nil {
				Expect(r).Should(Equal(result.R))
				Expect(s).Should(Equal(result.S))
			} else {
				r = result.R
				s = result.S
			}
		}

		ecdsaPublicKey := &ecdsa.PublicKey{
			Curve: expPublic.GetCurve(),
			X:     expPublic.GetX(),
			Y:     expPublic.GetY(),
		}
		Expect(ecdsa.Verify(ecdsaPublicKey, msg, r, s)).Should(BeTrue())

		for _, l := range listeners {
			l.AssertExpectations(GinkgoT())
		}

		x := new(big.Int)
		x, ok := x.SetString("85556674236879568521519464397895875853009790432415548013957742860428482257814", 10)
		if !ok {
			return
		}

		y := new(big.Int)
		y, okok := y.SetString("30994920491134501242484194368073251458639356656492191736687915449103623375987", 10)
		if !okok {
			return
		}

		rr := new(big.Int)
		rr, okokok := rr.SetString("29860421401746323085666773732583269052286762170227307798122711355561284363296", 10)
		if !okokok {
			return
		}

		rs := new(big.Int)
		rs, o := rs.SetString("25398801185773830584947880320881540957958746367752501504457184409022103774550", 10)
		if !o {
			return
		}

		ecdsaPublicKey1 := &ecdsa.PublicKey{
			Curve: expPublic.GetCurve(),
			X:     x,
			Y:     y,
			//X:     expPublic.GetX(),
			//Y:     expPublic.GetY(),
		}

		msg = []byte{101, 76, 2, 206, 14, 133, 132, 177, 17, 12, 19, 100, 189, 78, 153, 159, 62, 37, 106, 6, 191, 114, 151, 81, 10, 110, 208, 163, 0, 39, 217, 27}
		Expect(ecdsa.Verify(ecdsaPublicKey1, msg, rr, rs)).Should(BeTrue())
		verify := ecdsa.Verify(ecdsaPublicKey1, msg, rr, rs)
		fmt.Sprint(verify)

	},
		Entry("(shareX, shareY, rank):(1,3,0),(10,111,0),(20,421,0)", [][]*big.Int{
			{big.NewInt(1), big.NewInt(3), big.NewInt(0)},
			{big.NewInt(10), big.NewInt(111), big.NewInt(0)},
			{big.NewInt(20), big.NewInt(421), big.NewInt(0)},
		}, big.NewInt(1)),
		Entry("(shareX, shareY, rank):(108,4517821,0),(344,35822,1),(756,46,2)", [][]*big.Int{
			{big.NewInt(108), big.NewInt(4517821), big.NewInt(0)},
			{big.NewInt(344), big.NewInt(35822), big.NewInt(1)},
			{big.NewInt(756), big.NewInt(46), big.NewInt(2)},
		}, big.NewInt(2089765)),
		Entry("(shareX, shareY, rank):(53,2816277,0),(24,48052,1),(96,9221170,0)", [][]*big.Int{
			{big.NewInt(53), big.NewInt(2816277), big.NewInt(0)},
			{big.NewInt(24), big.NewInt(48052), big.NewInt(1)},
			{big.NewInt(96), big.NewInt(9221170), big.NewInt(0)},
		}, big.NewInt(4786)),
		Entry("(shareX, shareY, rank):(756,1408164810,0),(59887,285957312,1),(817291849,3901751343900,1)", [][]*big.Int{
			{big.NewInt(756), big.NewInt(1408164810), big.NewInt(0)},
			{big.NewInt(59887), big.NewInt(285957312), big.NewInt(1)},
			{big.NewInt(817291849), big.NewInt(3901751343900), big.NewInt(1)},
		}, big.NewInt(987234)),
		Entry("(shareX, shareY, rank):(999,1990866633,0),(877,1535141367,1),(6542,85090458377,1)", [][]*big.Int{
			{big.NewInt(999), big.NewInt(1990866633), big.NewInt(0)},
			{big.NewInt(877), big.NewInt(1535141367), big.NewInt(0)},
			{big.NewInt(6542), big.NewInt(85090458377), big.NewInt(0)},
		}, big.NewInt(5487)),
		Entry("(shareX, shareY, rank):(1094,591493497,0),(59887,58337825,1),(6542,20894113809,0)", [][]*big.Int{
			{big.NewInt(1094), big.NewInt(591493497), big.NewInt(0)},
			{big.NewInt(59887), big.NewInt(58337825), big.NewInt(1)},
			{big.NewInt(6542), big.NewInt(20894113809), big.NewInt(0)},
		}, big.NewInt(5987)),
		Entry("(shareX, shareY, rank):(404,1279853690,0),(99555,1548484036,1),(64444,15554,2)", [][]*big.Int{
			{big.NewInt(404), big.NewInt(1279853690), big.NewInt(0)},
			{big.NewInt(99555), big.NewInt(1548484036), big.NewInt(1)},
			{big.NewInt(64444), big.NewInt(15554), big.NewInt(2)},
		}, big.NewInt(8274194)),
	)
})

func getID(id int) string {
	return fmt.Sprintf("id-%d", id)
}

type peerManager struct {
	id       string
	numPeers uint32
	signers  map[string]*Signer
}

func newPeerManager(id string, numPeers int) *peerManager {
	return &peerManager{
		id:       id,
		numPeers: uint32(numPeers),
	}
}

func (p *peerManager) setSigners(signers map[string]*Signer) {
	p.signers = signers
}

func (p *peerManager) NumPeers() uint32 {
	return p.numPeers
}

func (p *peerManager) SelfID() string {
	return p.id
}

func (p *peerManager) MustSend(id string, message proto.Message) {
	d := p.signers[id]
	msg := message.(types.Message)
	Expect(d.AddMessage(msg)).Should(BeNil())
}

func newSigners(curve elliptic.Curve, expPublic *ecpointgrouplaw.ECPoint, ss [][]*big.Int, msg []byte) (map[string]*Signer, map[string]*mocks.StateChangedListener) {
	threshold := len(ss)
	signers := make(map[string]*Signer, threshold)
	peerManagers := make([]types.PeerManager, threshold)
	listeners := make(map[string]*mocks.StateChangedListener, threshold)

	bks := make(map[string]*birkhoffinterpolation.BkParameter, threshold)
	for i := 0; i < threshold; i++ {
		bks[getID(i)] = birkhoffinterpolation.NewBkParameter(ss[i][0], uint32(ss[i][2].Uint64()))
	}

	for i := 0; i < threshold; i++ {
		id := getID(i)
		pm := newPeerManager(id, threshold-1)
		pm.setSigners(signers)
		peerManagers[i] = pm
		listeners[id] = new(mocks.StateChangedListener)
		homo, err := paillier.NewPaillier(2048)
		Expect(err).Should(BeNil())
		signers[id], err = NewSigner(peerManagers[i], expPublic, homo, ss[i][1], bks, msg, listeners[id])
		Expect(err).Should(BeNil())
		r, err := signers[id].GetResult()
		Expect(r).Should(BeNil())
		Expect(err).Should(Equal(tss.ErrNotReady))
		signers[id].Start()
	}
	return signers, listeners
}
