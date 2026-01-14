package application

import (
	"fmt"
	"time"

	"github.com/oxaxxaxaxaxaxaxxaaaxax/snake/internal/application/transport"
	"github.com/oxaxxaxaxaxaxaxxaaaxax/snake/internal/domain"
	pb "github.com/oxaxxaxaxaxaxaxxaaaxax/snake/proto"
)

type Normal struct {
	Engine *Engine

	lastSend time.Time
	lastRecv time.Time
}

func NewNormal(e *Engine) *Normal {
	return &Normal{Engine: e}
}

func (n *Normal) SetEngine(e *Engine) {
	n.Engine = e
}

func (n *Normal) AddMasterInfo(addr string, id int32) {
	n.Engine.Peers.PeersInfo[addr] = &transport.PeerInfo{
		Acknowledges: &transport.PendingAcks{Acks: make(map[int64]*transport.PendingAckMsg)},
		LastRecv:     time.Time{},
		LastSend:     time.Time{},
	}
	n.Engine.GameCtx.MasterInfo = domain.MasterInfo{
		MasterAddr: addr,
		MasterId:   id,
	}
}

func (n *Normal) SetGameContext() {
	n.Engine.GameCtx = &domain.GameCtx{}
	n.Engine.GameCtx.PlayerID = UninitializedId
	n.Engine.GameCtx.PlayerName = "normal_name"
	n.Engine.GameCtx.PlayerType = pb.PlayerType_HUMAN
	n.Engine.GameCtx.Role = pb.NodeRole_NORMAL
	n.Engine.GameCtx.StateMsDelay = DefaultStateMsDelay
}

func (n *Normal) SetId(id int32) {
	n.Engine.GameCtx.PlayerID = id
}

func (n *Normal) SendPingToMaster() {
	ctx := n.Engine.GameCtx
	n.Engine.USock.SendPing(ctx.MasterInfo.MasterAddr, ctx.PlayerID,
		n.Engine.Peers.PeersInfo[ctx.MasterInfo.MasterAddr])
}

func (n *Normal) ChangeMasterInfo() domain.MasterInfo {
	ctx := n.Engine.GameCtx
	ctx.MasterInfo.MasterId = ctx.DeputyInfo.DeputyId
	ctx.MasterInfo.MasterAddr = ctx.DeputyInfo.DeputyAddr
	return ctx.MasterInfo
}

func (n *Normal) BecomeViewer() {
	viewer := NewViewer(n.Engine)
	viewer.LeaveGame()
}

func (n *Normal) HandleRoleChange(senderRole, receiverRole pb.NodeRole, peerInfo *transport.PeerInfo) RolePlayer {
	switch senderRole {

	case pb.NodeRole_MASTER:
		switch receiverRole {
		// от главного к умершему игроку (receiver_role = VIEWER)
		case pb.NodeRole_VIEWER:
			fmt.Println("Your snake is dead!")
			v := NewViewer(n.Engine)
			v.Watch()
			return v
		//от заместителя другим игрокам о том, что пора начинать считать его главным (sender_role = MASTER)
		case pb.NodeRole_NORMAL:
			fmt.Println("New master!")
			//о новом депути нормалы узнают из StateMsg
			n.Engine.GameCtx.MasterInfo = n.ChangeMasterInfo()
			return n
		//назначение кого-то заместителем (receiver_role = DEPUTY)
		case pb.NodeRole_DEPUTY:
			fmt.Println("You are new deputy!")
			d := NewDeputy(n.Engine) //??? хз насколько это надо upd14.01 - надо!
			return d

		}
	}
	return n
}

func (n *Normal) Start() {
	fmt.Println("normal Start game")
	err := n.Engine.USock.SendDiscover(-1)
	if err != nil {
		fmt.Println(err)
	}
}

func (n *Normal) StartNetTicker() {
	fmt.Println("normal Start NetTicker")
	interval := n.Engine.GameCtx.StateMsDelay / 10
	recvInterval := interval * 8

	ticker := time.NewTicker(time.Duration(interval) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			n.CheckTimeoutInteraction(interval, recvInterval)
		}
	}
}

func (n *Normal) CheckTimeoutInteraction(interval, recvInterval int32) RolePlayer {
	//fmt.Println("normal CheckTimeoutInteraction")
	masterInfo := n.Engine.GameCtx.MasterInfo
	peerInfo := n.Engine.Peers.PeersInfo[masterInfo.MasterAddr]

	if peerInfo == nil {
		fmt.Println("Master is nil")
		return n
	}
	fmt.Println("Master not nil")
	if !peerInfo.LastSend.IsZero() {
		if time.Now().Sub(peerInfo.LastSend) >= time.Duration(interval)*time.Millisecond {
			fmt.Println("Master send timeout - SEND PING")
			n.SendPingToMaster()
		}
	}

	acks := peerInfo.Acknowledges.Acks
	for _, ack := range acks {
		if ack.SendTime.IsZero() {
			continue
		}
		if time.Now().Sub(ack.SendTime) >= time.Duration(interval)*time.Millisecond {
			fmt.Println("Master ack timeout - RETRY")
			n.Engine.USock.SendMessage(ack.RequestMsg, masterInfo.MasterAddr, peerInfo)
		}
	}

	if !peerInfo.LastRecv.IsZero() {
		if time.Now().Sub(peerInfo.LastRecv) >= time.Duration(recvInterval)*time.Millisecond {
			fmt.Println("Master recieve timeout - DISCONNECT")
			n.Engine.GameCtx.MasterInfo = n.ChangeMasterInfo()
		}
	}
	return n
}
