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

	//lastSend time.Time
	//lastRecv time.Time
}

func NewViewer(e *Engine) *Viewer {
	return &Viewer{Engine: e}
}

func (v *Viewer) SetEngine(e *Engine) {
	v.Engine = e
}

func (v *Viewer) SetId(id int32) {
	v.Engine.GameMtx.Lock()
	v.Engine.GameCtx.PlayerID = id
	v.Engine.GameMtx.Unlock()
}

func (v *Viewer) ChangeMasterInfo() domain.MasterInfo {
	v.Engine.GameMtx.Lock()
	defer v.Engine.GameMtx.Unlock()
	ctx := v.Engine.GameCtx
	ctx.MasterInfo.MasterId = ctx.DeputyInfo.DeputyId
	ctx.MasterInfo.MasterAddr = ctx.DeputyInfo.DeputyAddr
	return ctx.MasterInfo
}

func (v *Viewer) AddMasterInfo(addr string, id int32) {
	//недостижимый кейс
	//v.Engine.Peers.PeersInfo[addr] = &transport.PeerInfo{
	//	Acknowledges: &transport.PendingAcks{Acks: make(map[int64]*transport.PendingAckMsg)},
	//	LastRecv:     time.Time{},
	//	LastSend:     time.Time{},
	//}
	v.Engine.GameMtx.Lock()
	v.Engine.GameCtx.MasterInfo = domain.MasterInfo{
		MasterAddr: addr,
		MasterId:   id,
	}
	v.Engine.GameMtx.Unlock()
}

func (v *Viewer) LeaveGame() {
	v.Engine.GameMtx.Lock()
	masterAddr := v.Engine.GameCtx.MasterInfo.MasterAddr
	senderId := v.Engine.GameCtx.PlayerID
	v.Engine.GameMtx.Unlock()

	v.Engine.PeerMtx.Lock()
	peerInfo := v.Engine.Peers.PeersInfo[masterAddr]
	v.Engine.PeerMtx.Unlock()

	err := v.Engine.USock.SendRoleChange(masterAddr, senderId, pb.NodeRole_MASTER,
		pb.NodeRole_VIEWER, peerInfo, &v.Engine.PeerMtx)
	if err != nil {
		fmt.Println(err)
	}
}

func (v *Viewer) MasterToViewer(newMasterAddr string, peerInfo *transport.PeerInfo) {
	v.Engine.GameMtx.Lock()
	senderId := v.Engine.GameCtx.PlayerID
	v.Engine.GameMtx.Unlock()

	err := v.Engine.USock.SendRoleChange(newMasterAddr, senderId, pb.NodeRole_MASTER,
		pb.NodeRole_VIEWER, peerInfo, &v.Engine.PeerMtx)
	if err != nil {
		fmt.Println(err)
	}
}

func (v *Viewer) Watch() {
	fmt.Println("You watch")
}

func (v *Viewer) HandleRoleChange(senderRole, receiverRole pb.NodeRole, peerInfo *transport.PeerInfo) RolePlayer {
	fmt.Println("stub! this case is unreachable")
	return v
}

func (v *Viewer) SetGameContext() {
	v.Engine.GameMtx.Lock()
	v.Engine.GameCtx = &domain.GameCtx{}
	v.Engine.GameCtx.PlayerID = UninitializedId
	v.Engine.GameCtx.PlayerName = "viewer_name"
	v.Engine.GameCtx.PlayerType = pb.PlayerType_HUMAN
	v.Engine.GameCtx.Role = pb.NodeRole_VIEWER
	v.Engine.GameCtx.StateMsDelay = DefaultStateMsDelay
	v.Engine.GameMtx.Unlock()
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
	v.Engine.GameMtx.Lock()
	masterAddr := v.Engine.GameCtx.MasterInfo.MasterAddr
	senderId := v.Engine.GameCtx.PlayerID
	v.Engine.GameMtx.Unlock()

	v.Engine.PeerMtx.Lock()
	peerInfo := v.Engine.Peers.PeersInfo[masterAddr]
	v.Engine.PeerMtx.Unlock()

	err := v.Engine.USock.SendPing(masterAddr, senderId, peerInfo, &v.Engine.PeerMtx)
	if err != nil {
		fmt.Println(err)
	}
}

func (v *Viewer) CheckTimeoutInteraction(interval, recvInterval int32) RolePlayer {
	v.Engine.GameMtx.Lock()
	masterInfo := v.Engine.GameCtx.MasterInfo
	v.Engine.GameMtx.Unlock()

	v.Engine.PeerMtx.Lock()
	peerInfo := v.Engine.Peers.PeersInfo[masterInfo.MasterAddr]
	v.Engine.PeerMtx.Unlock()

	if peerInfo == nil {
		return v
	}

	now := time.Now()

	peerInfo.M.Lock()
	if peerInfo.Dead {
		peerInfo.M.Unlock()
		return v
	}
	lastSend := peerInfo.LastSend
	peerInfo.M.Unlock()

	if !lastSend.IsZero() {
		if now.Sub(lastSend) >= time.Duration(interval)*time.Millisecond {
			fmt.Println("Master send timeout - SEND PING")
			v.SendPingToMaster()
		}
	}

	peerInfo.M.Lock()
	//acks := peerInfo.Acknowledges.Acks
	acks := CopyPendingAcks(peerInfo.Acknowledges)
	peerInfo.M.Unlock()

	if acks == nil {
		return v
	}

	for _, ack := range acks.Acks {
		if ack == nil {
			continue
		}
		if ack.SendTime.IsZero() {
			continue
		}
		if now.Sub(ack.SendTime) >= time.Duration(interval)*time.Millisecond {
			fmt.Println("Master ack timeout - RETRY")
			err := v.Engine.USock.SendMessage(ack.RequestMsg, masterInfo.MasterAddr, peerInfo, &v.Engine.PeerMtx)
			if err != nil {
				fmt.Println("Send mes to master error:", err)
			}
		}
	}

	peerInfo.M.Lock()
	lastRecv := peerInfo.LastRecv
	peerInfo.M.Unlock()

	if !lastRecv.IsZero() {
		if now.Sub(lastRecv) >= time.Duration(recvInterval)*time.Millisecond {
			fmt.Println("Master recieve timeout - DISCONNECT")
			v.ChangeMasterInfo()
		}
	}
	return v
}
