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

package dkg

import (
	"crypto/elliptic"
	"errors"
	fmt "fmt"
	"math/big"

	"github.com/getamis/alice/crypto/birkhoffinterpolation"
	"github.com/getamis/alice/crypto/commitment"
	"github.com/getamis/alice/crypto/ecpointgrouplaw"
	"github.com/getamis/alice/crypto/polynomial"
	"github.com/getamis/alice/crypto/tss"
	"github.com/getamis/alice/crypto/tss/message/types"
	"github.com/getamis/alice/crypto/utils"
	"github.com/getamis/sirius/log"
	proto "github.com/golang/protobuf/proto"
)

var (
	ErrNotEnoughRanks = errors.New("not enough ranks")
)

type peerData struct {
	bk *birkhoffinterpolation.BkParameter
}

type peerHandler struct {
	// self information
	bk                  *birkhoffinterpolation.BkParameter
	poly                *polynomial.Polynomial
	threshold           uint32
	curve               elliptic.Curve
	u0g                 *ecpointgrouplaw.ECPoint
	u0gCommiter         *commitment.HashCommitmenter
	feldmanCommitmenter *commitment.FeldmanCommitmenter

	peerManager types.PeerManager
	peerNum     uint32
	peers       map[string]*peer
}

func newPeerHandler(curve elliptic.Curve, peerManager types.PeerManager, threshold uint32, rank uint32) (*peerHandler, error) {
	params := curve.Params()
	fieldOrder := params.N
	fmt.Printf("fieldOrder: %d\n", params.N)
	poly, err := polynomial.RandomPolynomial(fieldOrder, threshold-1)
	if err != nil {
		return nil, err
	}

	//u0 for private key reconstruction test( shoud delete )
	fmt.Printf("-------------------- u0: %s -------------------\n", poly.Get(0).Text(10))
	fmt.Printf("test: %d\n", poly.Get(0))
	// Random x and build bk
	x, err := utils.RandomPositiveInt(fieldOrder)
	if err != nil {
		return nil, err
	}
	return newPeerHandlerWithPolynomial(curve, peerManager, threshold, x, rank, poly)
}

func newPeerHandlerWithPolynomial(curve elliptic.Curve, peerManager types.PeerManager, threshold uint32, x *big.Int, rank uint32, poly *polynomial.Polynomial) (*peerHandler, error) {
	if err := utils.EnsureThreshold(threshold, peerManager.NumPeers()+1); err != nil {
		return nil, err
	}

	// Build Feldman commitmenter
	feldmanCommitmenter, err := commitment.NewFeldmanCommitmenter(curve, poly)
	if err != nil {
		return nil, err
	}

	// Build bk
	bk := birkhoffinterpolation.NewBkParameter(x, rank)

	// Calculate u0g
	u0 := poly.Get(0)
	u0g := ecpointgrouplaw.ScalarBaseMult(curve, u0)
	fmt.Printf("u0g: %d,  %d\n", u0g.GetX(), u0g.GetY())
	u0gCommiter, err := tss.NewCommitterByPoint(u0g)
	if err != nil {
		return nil, err
	}

	return &peerHandler{
		bk:                  bk,
		poly:                poly,
		threshold:           threshold,
		curve:               curve,
		u0g:                 u0g,
		u0gCommiter:         u0gCommiter,
		feldmanCommitmenter: feldmanCommitmenter,

		peerManager: peerManager,
		peerNum:     peerManager.NumPeers(),
		peers:       make(map[string]*peer, peerManager.NumPeers()),
	}, nil
}

func (p *peerHandler) MessageType() types.MessageType {
	return types.MessageType(Type_Peer)
}

func (p *peerHandler) GetRequiredMessageCount() uint32 {
	return p.peerNum
}

func (p *peerHandler) IsHandled(logger log.Logger, id string) bool {
	_, ok := p.peers[id]
	return ok
}

func (p *peerHandler) HandleMessage(logger log.Logger, message types.Message) error {
	msg := getMessage(message)
	id := msg.GetId()
	body := msg.GetPeer()
	peer := newPeer(id)
	peer.peer = &peerData{
		bk: body.GetBk().ToBk(),
	}
	p.peers[id] = peer
	return peer.AddMessage(msg)
}

func (p *peerHandler) Finalize(logger log.Logger) (types.Handler, error) {
	// Check if the bks are ok
	bks := make(birkhoffinterpolation.BkParameters, p.peerNum+1)
	bks[0] = p.bk
	i := 1
	for _, peer := range p.peers {
		bks[i] = peer.peer.bk
		i++
	}
	err := bks.CheckValid(p.threshold, p.curve.Params().N)
	if err != nil {
		logger.Warn("Failed to check bks", "err", err)
		return nil, err
	}

	// Send out Feldman commit message and decommit message to all peers
	msg := p.getDecommitMessage()
	p.broadcast(msg)
	return newDecommitHandler(p), nil
}

func (p *peerHandler) GetPeerMessage() *Message {
	return &Message{
		Type: Type_Peer,
		Id:   p.peerManager.SelfID(),
		Body: &Message_Peer{
			Peer: &BodyPeer{
				Bk:         p.bk.ToMessage(),
				Commitment: p.u0gCommiter.GetCommitmentMessage(),
			},
		},
	}
}

func (p *peerHandler) getDecommitMessage() *Message {
	return &Message{
		Type: Type_Decommit,
		Id:   p.peerManager.SelfID(),
		Body: &Message_Decommit{
			Decommit: &BodyDecommit{
				HashDecommitment: p.u0gCommiter.GetDecommitmentMessage(),
				PointCommitment:  p.feldmanCommitmenter.GetCommitmentMessage(),
			},
		},
	}
}

func (p *peerHandler) broadcast(msg proto.Message) {
	for id := range p.peers {
		p.peerManager.MustSend(id, msg)
	}
}

func getMessage(messsage types.Message) *Message {
	return messsage.(*Message)
}

func getMessageByType(peer *peer, t Type) *Message {
	return getMessage(peer.GetMessage(types.MessageType(t)))
}
