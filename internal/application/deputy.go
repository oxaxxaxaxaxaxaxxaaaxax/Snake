package application

import (
	"fmt"
	"time"

	"github.com/oxaxxaxaxaxaxaxxaaaxax/snake/internal/domain"
	pb "github.com/oxaxxaxaxaxaxaxxaaaxax/snake/proto"
)

type Deputy struct {
	Engine *Engine

	lastSend time.Time
	lastRecv time.Time
}

func NewDeputy(e *Engine) Deputy {
	return Deputy{Engine: e}
}

func (d Deputy) SetEngine(e *Engine) {
	d.Engine = e
}

func (d Deputy) SetId(id int32) {
	d.Engine.GameCtx.PlayerID = id
}

func (d Deputy) SetGameContext() {
	d.Engine.GameCtx = &domain.GameCtx{}
	d.Engine.GameCtx.PlayerID = UninitializedId
	d.Engine.GameCtx.PlayerName = "deputy_name"
	d.Engine.GameCtx.PlayerType = pb.PlayerType_HUMAN
	d.Engine.GameCtx.Role = pb.NodeRole_DEPUTY
}

func (d Deputy) BecomeMaster() {
	master := NewMaster(d.Engine)
	master.DeputyToMaster()
}

func (d Deputy) BecomeViewer() {
	viewer := NewViewer(d.Engine)
	viewer.LeaveGame()
}

func (d Deputy) HandleRoleChange(senderRole, receiverRole pb.NodeRole) RolePlayer {
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

func (d Deputy) Start() {
	fmt.Println("deputy Start game")
}
