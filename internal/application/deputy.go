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
	//lastSend time.Time
	//lastRecv time.Time
}

func NewDeputy(e *Engine) *Deputy {
	return &Deputy{Engine: e}
}

func (d *Deputy) SetEngine(e *Engine) {
	d.Engine = e
}

func (d *Deputy) SetId(id int32) {
	d.Engine.GameMtx.Lock()
	d.Engine.GameCtx.PlayerID = id
	d.Engine.GameMtx.Unlock()
}

func (d *Deputy) SetGameContext() {
	d.Engine.GameMtx.Lock()
	d.Engine.GameCtx = &domain.GameCtx{}
	d.Engine.GameCtx.PlayerID = UninitializedId
	d.Engine.GameCtx.PlayerName = "deputy_name"
	d.Engine.GameCtx.PlayerType = pb.PlayerType_HUMAN
	d.Engine.GameCtx.Role = pb.NodeRole_DEPUTY
	d.Engine.GameCtx.StateMsDelay = DefaultStateMsDelay
	d.Engine.GameMtx.Unlock()
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
	d.Engine.GameMtx.Lock()
	ctx := d.Engine.GameCtx
	masterAddr := ctx.MasterInfo.MasterAddr
	senderId := ctx.PlayerID
	d.Engine.GameMtx.Unlock()

	d.Engine.PeerMtx.Lock()
	peerInfo := d.Engine.Peers.PeersInfo[ctx.MasterInfo.MasterAddr]
	d.Engine.PeerMtx.Unlock()

	err := d.Engine.USock.SendPing(masterAddr, senderId, peerInfo, &d.Engine.PeerMtx)
	if err != nil {
		fmt.Println("Send ping to master error:", err)
	}
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
	//недостижимый кейс
	//d.Engine.Peers.PeersInfo[addr] = &transport.PeerInfo{
	//	Acknowledges: &transport.PendingAcks{Acks: make(map[int64]*transport.PendingAckMsg)},
	//	LastRecv:     time.Time{},
	//	LastSend:     time.Time{},
	//}
	d.Engine.GameMtx.Lock()
	d.Engine.GameCtx.MasterInfo = domain.MasterInfo{
		MasterAddr: addr,
		MasterId:   id,
	}
	d.Engine.GameMtx.Unlock()
}

// Start заглушка
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
	d.Engine.GameMtx.Lock()
	masterInfo := d.Engine.GameCtx.MasterInfo
	d.Engine.GameMtx.Unlock()

	d.Engine.PeerMtx.Lock()
	peerInfo := d.Engine.Peers.PeersInfo[masterInfo.MasterAddr]
	d.Engine.PeerMtx.Unlock()

	if peerInfo == nil {
		return d
	}

	now := time.Now()

	peerInfo.M.Lock()
	if peerInfo.Dead {
		peerInfo.M.Unlock()
		return d
	}
	lastSend := peerInfo.LastSend
	peerInfo.M.Unlock()

	if !lastSend.IsZero() {
		if now.Sub(lastSend) >= time.Duration(interval)*time.Millisecond {
			fmt.Println(" (deputy)Master send timeout - SEND PING")
			d.SendPingToMaster()
		}
	}

	peerInfo.M.Lock()
	//acks := peerInfo.Acknowledges.Acks
	acks := CopyPendingAcks(peerInfo.Acknowledges)
	peerInfo.M.Unlock()

	if acks == nil {
		return d
	}

	for _, ack := range acks.Acks {
		if ack == nil {
			continue
		}
		if ack.SendTime.IsZero() {
			continue
		}
		if now.Sub(ack.SendTime) >= time.Duration(interval)*time.Millisecond {
			fmt.Println("(deputy) Master ack timeout - RETRY")
			err := d.Engine.USock.SendMessage(ack.RequestMsg, masterInfo.MasterAddr, peerInfo, &d.Engine.PeerMtx)
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
			fmt.Println("(deputy) Master recieve timeout - DISCONNECT")
			return d.BecomeMaster()
			//d.Engine.GameCtx.MasterInfo = d.ChangeMasterInfo()
			//*d.Engine.GameCtx = d.ChangeMasterInfo()
		}
	}
	return d
}
