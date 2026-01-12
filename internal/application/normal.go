package application

import (
	"fmt"
	"time"

	"github.com/oxaxxaxaxaxaxaxxaaaxax/snake/internal/domain"
	pb "github.com/oxaxxaxaxaxaxaxxaaaxax/snake/proto"
)

type Normal struct {
	Engine *Engine

	lastSend time.Time
	lastRecv time.Time
}

func NewNormal(e *Engine) Normal {
	return Normal{Engine: e}
}

func (n Normal) SetEngine(e *Engine) {
	n.Engine = e
}

func (n Normal) SetGameContext() {
	n.Engine.GameCtx = &domain.GameCtx{}
	n.Engine.GameCtx.PlayerID = UninitializedId
	n.Engine.GameCtx.PlayerName = "normal_name"
	n.Engine.GameCtx.PlayerType = pb.PlayerType_HUMAN
	n.Engine.GameCtx.Role = pb.NodeRole_NORMAL
}

func (n Normal) SetId(id int32) {
	n.Engine.GameCtx.PlayerID = id
}

func (n Normal) SendPingToMaster() {
	ctx := n.Engine.GameCtx
	n.Engine.USock.SendPing(ctx.MasterInfo.MasterAddr, ctx.PlayerID)
}

func (n Normal) ChangeMasterInfo() domain.GameCtx {
	ctx := n.Engine.GameCtx
	ctx.MasterInfo.MasterId = ctx.DeputyInfo.DeputyId
	ctx.MasterInfo.MasterAddr = ctx.DeputyInfo.DeputyAddr
	return *ctx
}

func (n Normal) BecomeViewer() {
	viewer := NewViewer(n.Engine)
	viewer.LeaveGame()
}

func (n Normal) HandleRoleChange(senderRole, receiverRole pb.NodeRole) RolePlayer {
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
			*n.Engine.GameCtx = n.ChangeMasterInfo()
			return n
		//назначение кого-то заместителем (receiver_role = DEPUTY)
		case pb.NodeRole_DEPUTY:
			fmt.Println("You are new deputi!")
			d := NewDeputy(n.Engine) //??? хз насколько это надо
			return d

		}
	}
	return n
}

func (n Normal) Start() {
	fmt.Println("normal Start game")
	err := n.Engine.USock.SendDiscover(-1)
	if err != nil {
		fmt.Println(err)
	}
}
