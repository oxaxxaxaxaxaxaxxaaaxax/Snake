package application

import (
	"fmt"
	"time"

	"github.com/oxaxxaxaxaxaxaxxaaaxax/snake/internal/domain"
	pb "github.com/oxaxxaxaxaxaxaxxaaaxax/snake/proto"
)

type Viewer struct {
	Engine *Engine

	lastSend time.Time
	lastRecv time.Time
}

func NewViewer(e *Engine) Viewer {
	return Viewer{Engine: e}
}

func (v Viewer) SetEngine(e *Engine) {
	v.Engine = e
}

func (v Viewer) SetId(id int32) {
	v.Engine.GameCtx.PlayerID = id
}

func (v Viewer) LeaveGame() {
	ctx := v.Engine.GameCtx
	v.Engine.USock.SendRoleChange(ctx.MasterInfo.MasterAddr, ctx.PlayerID, pb.NodeRole_MASTER, pb.NodeRole_VIEWER)
}

func (v Viewer) MasterToViewer(newMasterAddr string) {
	ctx := v.Engine.GameCtx
	v.Engine.USock.SendRoleChange(newMasterAddr, ctx.PlayerID, pb.NodeRole_MASTER, pb.NodeRole_VIEWER)
}

func (v Viewer) Watch() {
	fmt.Println("You watch")
}

func (v Viewer) HandleRoleChange(senderRole, receiverRole pb.NodeRole) RolePlayer {
	fmt.Println("stub! this case is unreachable")
	return v
}

func (v Viewer) SetGameContext() {
	v.Engine.GameCtx = &domain.GameCtx{}
	v.Engine.GameCtx.PlayerID = UninitializedId
	v.Engine.GameCtx.PlayerName = "viewer_name"
	v.Engine.GameCtx.PlayerType = pb.PlayerType_HUMAN
	v.Engine.GameCtx.Role = pb.NodeRole_VIEWER
}

func (v Viewer) Start() {
	fmt.Println("viewer Start game")
	err := v.Engine.USock.SendDiscover(-1)
	if err != nil {
		fmt.Println(err)
	}
}
