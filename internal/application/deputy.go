package application

import (
	"fmt"
	"time"

	"github.com/oxaxxaxaxaxaxaxxaaaxax/snake/internal/application/transport"
	"github.com/oxaxxaxaxaxaxaxxaaaxax/snake/internal/domain"
	pb "github.com/oxaxxaxaxaxaxaxxaaaxax/snake/proto"
)

type Deputy struct {
	Engine *Engine

	lastSend time.Time
	lastRecv time.Time
}

func NewDeputy(e *Engine) *Deputy {
	return &Deputy{Engine: e}
}

func (d *Deputy) SetEngine(e *Engine) {
	d.Engine = e
}

func (d *Deputy) SetId(id int32) {
	d.Engine.GameCtx.PlayerID = id
}

func (d *Deputy) SetGameContext() {
	d.Engine.GameCtx = &domain.GameCtx{}
	d.Engine.GameCtx.PlayerID = UninitializedId
	d.Engine.GameCtx.PlayerName = "deputy_name"
	d.Engine.GameCtx.PlayerType = pb.PlayerType_HUMAN
	d.Engine.GameCtx.Role = pb.NodeRole_DEPUTY
	d.Engine.GameCtx.StateMsDelay = DefaultStateMsDelay
}

func (d *Deputy) BecomeMaster() RolePlayer {
	master := NewMaster(d.Engine)
	return master.DeputyToMaster()
}

func (d *Deputy) BecomeViewer() {
	viewer := NewViewer(d.Engine)
	viewer.LeaveGame()
}

func (d *Deputy) SendPingToMaster() {
	ctx := d.Engine.GameCtx
	d.Engine.USock.SendPing(ctx.MasterInfo.MasterAddr, ctx.PlayerID,
		d.Engine.Peers.PeersInfo[ctx.MasterInfo.MasterAddr])
}

func (d *Deputy) HandleRoleChange(senderRole, receiverRole pb.NodeRole, peerInfo *transport.PeerInfo) RolePlayer {
	switch senderRole {
	case pb.NodeRole_VIEWER:
		switch receiverRole {
		case pb.NodeRole_MASTER:
			m := NewMaster(d.Engine)
			m.DeputyToMaster()
			return m
		}
	}
	return d
}

func (d *Deputy) AddMasterInfo(addr string, id int32) {
	d.Engine.Peers.PeersInfo[addr] = &transport.PeerInfo{
		Acknowledges: &transport.PendingAcks{Acks: make(map[int64]*transport.PendingAckMsg)},
		LastRecv:     time.Time{},
		LastSend:     time.Time{},
	}
	d.Engine.GameCtx.MasterInfo = domain.MasterInfo{
		MasterAddr: addr,
		MasterId:   id,
	}
}

// Start this is stub
func (d *Deputy) Start() {
	fmt.Println("deputy Start game")
}

func (d *Deputy) StartNetTicker() {
	interval := d.Engine.GameCtx.StateMsDelay / 10
	recvInterval := interval * 8

	ticker := time.NewTicker(time.Duration(interval) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			d.CheckTimeoutInteraction(interval, recvInterval)
		}
	}
}

func (d *Deputy) CheckTimeoutInteraction(interval, recvInterval int32) RolePlayer {
	masterInfo := d.Engine.GameCtx.MasterInfo
	peerInfo := d.Engine.Peers.PeersInfo[masterInfo.MasterAddr]

	if peerInfo == nil || peerInfo.Dead {
		return d
	}
	if !peerInfo.LastRecv.IsZero() {
		if time.Now().Sub(peerInfo.LastSend) >= time.Duration(interval)*time.Millisecond {
			fmt.Println(" (deputy)Master send timeout - SEND PING")
			d.SendPingToMaster()
		}
	}

	acks := peerInfo.Acknowledges.Acks
	for _, ack := range acks {
		if ack.SendTime.IsZero() {
			continue
		}
		if time.Now().Sub(ack.SendTime) >= time.Duration(interval)*time.Millisecond {
			fmt.Println("(deputy) Master ack timeout - RETRY")
			err := d.Engine.USock.SendMessage(ack.RequestMsg, masterInfo.MasterAddr, peerInfo)
			if err != nil {

			}
		}
	}

	if !peerInfo.LastRecv.IsZero() {
		if time.Now().Sub(peerInfo.LastRecv) >= time.Duration(recvInterval)*time.Millisecond {
			fmt.Println("(deputy) Master recieve timeout - DISCONNECT")
			return d.BecomeMaster()
			//d.Engine.GameCtx.MasterInfo = d.ChangeMasterInfo()
			//*d.Engine.GameCtx = d.ChangeMasterInfo()
		}
	}
	return d
}
