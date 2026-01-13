package application

import (
	"fmt"
	"time"

	"github.com/oxaxxaxaxaxaxaxxaaaxax/snake/internal/application/transport"
	"github.com/oxaxxaxaxaxaxaxxaaaxax/snake/internal/domain"
	pb "github.com/oxaxxaxaxaxaxaxxaaaxax/snake/proto"
)

type Viewer struct {
	Engine *Engine

	lastSend time.Time
	lastRecv time.Time
}

func NewViewer(e *Engine) *Viewer {
	return &Viewer{Engine: e}
}

func (v *Viewer) SetEngine(e *Engine) {
	v.Engine = e
}

func (v *Viewer) SetId(id int32) {
	v.Engine.GameCtx.PlayerID = id
}

func (v *Viewer) ChangeMasterInfo() domain.GameCtx {
	ctx := v.Engine.GameCtx
	ctx.MasterInfo.MasterId = ctx.DeputyInfo.DeputyId
	ctx.MasterInfo.MasterAddr = ctx.DeputyInfo.DeputyAddr
	return *ctx
}

func (v *Viewer) AddMasterInfo(addr string, id int32) {
	v.Engine.Peers.PeersInfo[addr] = &transport.PeerInfo{
		Acknowledges: &transport.PendingAcks{Acks: make(map[int64]*transport.PendingAckMsg)},
		LastRecv:     time.Time{},
		LastSend:     time.Time{},
	}
	v.Engine.GameCtx.MasterInfo = domain.MasterInfo{
		MasterAddr: addr,
		MasterId:   id,
	}
}

func (v *Viewer) LeaveGame() {
	ctx := v.Engine.GameCtx
	v.Engine.USock.SendRoleChange(ctx.MasterInfo.MasterAddr, ctx.PlayerID, pb.NodeRole_MASTER,
		pb.NodeRole_VIEWER, v.Engine.Peers.PeersInfo[ctx.MasterInfo.MasterAddr])
}

func (v *Viewer) MasterToViewer(newMasterAddr string, peerInfo *transport.PeerInfo) {
	ctx := v.Engine.GameCtx
	v.Engine.USock.SendRoleChange(newMasterAddr, ctx.PlayerID, pb.NodeRole_MASTER, pb.NodeRole_VIEWER, peerInfo)
}

func (v *Viewer) Watch() {
	fmt.Println("You watch")
}

func (v *Viewer) HandleRoleChange(senderRole, receiverRole pb.NodeRole, peerInfo *transport.PeerInfo) RolePlayer {
	fmt.Println("stub! this case is unreachable")
	return v
}

func (v *Viewer) SetGameContext() {
	v.Engine.GameCtx = &domain.GameCtx{}
	v.Engine.GameCtx.PlayerID = UninitializedId
	v.Engine.GameCtx.PlayerName = "viewer_name"
	v.Engine.GameCtx.PlayerType = pb.PlayerType_HUMAN
	v.Engine.GameCtx.Role = pb.NodeRole_VIEWER
	v.Engine.GameCtx.StateMsDelay = DefaultStateMsDelay
}

func (v *Viewer) Start() {
	fmt.Println("viewer Start game")
	err := v.Engine.USock.SendDiscover(-1)
	if err != nil {
		fmt.Println(err)
	}
}

func (v *Viewer) StartNetTicker() {
	interval := v.Engine.GameCtx.StateMsDelay / 10
	recvInterval := interval * 8

	ticker := time.NewTicker(time.Duration(interval) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			v.CheckTimeoutInteraction(interval, recvInterval)
		}
	}
}

func (v *Viewer) SendPingToMaster() {
	ctx := v.Engine.GameCtx
	v.Engine.USock.SendPing(ctx.MasterInfo.MasterAddr, ctx.PlayerID,
		v.Engine.Peers.PeersInfo[ctx.MasterInfo.MasterAddr])
}

func (v *Viewer) CheckTimeoutInteraction(interval, recvInterval int32) {
	masterInfo := v.Engine.GameCtx.MasterInfo
	peerInfo := v.Engine.Peers.PeersInfo[masterInfo.MasterAddr]

	if peerInfo == nil {
		return
	}
	if !peerInfo.LastSend.IsZero() {
		if time.Now().Sub(peerInfo.LastSend) >= time.Duration(interval)*time.Millisecond {
			fmt.Println("Master send timeout - SEND PING")
			v.SendPingToMaster()
		}
	}

	acks := peerInfo.Acknowledges.Acks
	for _, ack := range acks {
		if ack.SendTime.IsZero() {
			continue
		}
		if time.Now().Sub(ack.SendTime) >= time.Duration(interval)*time.Millisecond {
			fmt.Println("Master ack timeout - RETRY")
			v.Engine.USock.SendMessage(ack.RequestMsg, masterInfo.MasterAddr, peerInfo)
		}
	}

	if !peerInfo.LastRecv.IsZero() {
		if time.Now().Sub(peerInfo.LastRecv) >= time.Duration(recvInterval)*time.Millisecond {
			fmt.Println("Master recieve timeout - DISCONNECT")
			*v.Engine.GameCtx = v.ChangeMasterInfo()
		}
	}
}
