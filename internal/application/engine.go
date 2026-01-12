package application

import (
	"errors"
	"fmt"

	"github.com/oxaxxaxaxaxaxaxxaaaxax/snake/internal/application/transport"
	"github.com/oxaxxaxaxaxaxaxxaaaxax/snake/internal/domain"
	pb "github.com/oxaxxaxaxaxaxaxxaaaxax/snake/proto"
)

const UninitializedId = -1

var (
	ErrUnknownMessageType = errors.New("unknown message type")
)

type Engine struct {
	GameCtx *domain.GameCtx

	USock transport.UnicastSocket
	MSock transport.MulticastSocket

	Acknowledges *PendingAcks

	EventChan chan domain.Event
}

func (e *Engine) StartGame(player RolePlayer) {
	e.Acknowledges = NewPendingAcks()
	e.EventChan = make(chan domain.Event)

	e.GameCtx = &domain.GameCtx{}
	//e.GameCtx.PlayerID = UninitializedId

	e.USock = transport.NewUnicastSocket()
	e.MSock = transport.NewMulticastSocket()
	player.SetEngine(e)
	player.SetGameContext()

	go e.USock.ListenMessage(e.EventChan)
	go e.MSock.ReadFromMulticastSocket(e.EventChan)

	go player.Start()

	for {
		ev := <-e.EventChan
		fmt.Println("new event!")
		HandleMessage(e.USock, ev, e.GameCtx, player, e.Acknowledges)
	}
}

func HandleMessage(uSock transport.UnicastSocket, event domain.Event,
	gameCtx *domain.GameCtx, player RolePlayer, acks *PendingAcks) (RolePlayer, error) {
	message := event.GameMessage

	switch message.Type.(type) {
	case *pb.GameMessage_Ping:
		HandlePing(uSock, event, gameCtx.PlayerID)
		return player, nil
	case *pb.GameMessage_Steer:
		HandleSteer(uSock, event, gameCtx.PlayerID)
		return player, nil
	case *pb.GameMessage_Ack:
		HandleAck(uSock, event, gameCtx.PlayerID, acks, player)
		return player, nil
	case *pb.GameMessage_State:
		HandleState(uSock, event, gameCtx.PlayerID)
		return player, nil
	case *pb.GameMessage_Announcement:
		HandleAnnouncement(uSock, event, gameCtx)
		return player, nil
	case *pb.GameMessage_Discover:
		HandleDiscover(uSock, event, gameCtx, player)
		return player, nil
	case *pb.GameMessage_Join:
		HandleJoin(uSock, event, gameCtx.PlayerID, player)
		return player, nil
	case *pb.GameMessage_Error:
		HandleError(uSock, event, gameCtx.PlayerID)
		return player, nil
	case *pb.GameMessage_RoleChange:
		return HandleRoleChange(uSock, event, gameCtx.PlayerID, player), nil
	default:
		return nil, ErrUnknownMessageType
	}
}

func HandlePing(uSock transport.UnicastSocket, event domain.Event, senderId int32) {
	uSock.SendAck(event.GameMessage, event.From, senderId, transport.AssignedId)
}

func HandleSteer(uSock transport.UnicastSocket, event domain.Event, senderId int32) {
	uSock.SendAck(event.GameMessage, event.From, senderId, transport.AssignedId)
}

func HandleAck(uSock transport.UnicastSocket, event domain.Event, senderId int32,
	acks *PendingAcks, player RolePlayer) {
	fmt.Println("Handle Ack")
	if *event.GameMessage.ReceiverId != transport.AssignedId {
		fmt.Println("My Id:", *event.GameMessage.ReceiverId)
		player.SetId(*event.GameMessage.ReceiverId)
		return
	}
	ackMsg, err := acks.GetPendingAckById(*event.GameMessage.SenderId)
	if err != nil {
		fmt.Println("AcK err! ", err)
	}
	if ackMsg.RequestMsg.MsgSeq == event.GameMessage.MsgSeq {
		acks.RemoveAcks(ackMsg)
	}
}

func HandleState(uSock transport.UnicastSocket, event domain.Event, senderId int32) {
	uSock.SendAck(event.GameMessage, event.From, senderId, transport.AssignedId)
}

func HandleAnnouncement(uSock transport.UnicastSocket, event domain.Event, gameCtx *domain.GameCtx) {
	//fmt.Println("Receive Announcement")
	//fmt.Println("my id", gameCtx.PlayerID)
	if gameCtx.PlayerID == *event.GameMessage.SenderId {
		//fmt.Println("It's my message, skip")
		return
	}
	gameName := event.GameMessage.GetAnnouncement().Games[0].GameName

	gameCtx.GameName = *gameName
	fmt.Println("Announcement", gameCtx.GameName)
	if event.Transport == domain.Multicast {
		return
	}
	fmt.Println("role join", gameCtx.Role)
	err := uSock.SendJoin(event.From, *gameCtx)
	if err != nil {
		fmt.Println("Join err! ", err)
	}
	return
}

func HandleDiscover(uSock transport.UnicastSocket, event domain.Event, gameCtx *domain.GameCtx, player RolePlayer) {
	fmt.Println("Recieve Discover:", gameCtx.GameName, "from", event.From)
	fmt.Println(gameCtx.PlayerID, *event.GameMessage.SenderId)
	if gameCtx.PlayerID == *event.GameMessage.SenderId {
		fmt.Println("It's my message, skip")
		return
	}
	//announcement := pb.GameAnnouncement{GameName: &gameCtx.GameName}
	//
	switch m := player.(type) {
	case *Master:
		m.SendAnnouncement(event.From)
	default:
		fmt.Println("DISCOVER FROM NOT MASTER")
	}

	//m := player.(*Master)
}

func HandleJoin(uSock transport.UnicastSocket, event domain.Event,
	senderId int32, player RolePlayer) {
	fmt.Println("Receive Join")
	m := player.(*Master)
	switch *event.GameMessage.GetJoin().RequestedRole {
	case pb.NodeRole_NORMAL:
		if !m.PlaceSnake() {
			errorMsg := "Cannot join game: no place for snake"
			err := uSock.SendError(event.From, senderId, errorMsg)
			if err != nil {
				fmt.Println("Error err! ", err)
			}
			return
		}
		fmt.Println("Master placed snake!")
	}
	m.SendJoinAck(event.GameMessage, event.From)

	//if m.PlaceSnake() {
	//	fmt.Println("Master placed snake!")
	//	m.SendJoinAck(event.GameMessage, event.From)
	//}
	////!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
	//errorMsg := "Cannot join game: no place for snake"
	//err := uSock.SendError(event.From, senderId, errorMsg)
	//if err != nil {
	//	fmt.Println("Error err! ", err)
	//}
}

func HandleError(uSock transport.UnicastSocket, event domain.Event, senderId int32) {
	uSock.SendAck(event.GameMessage, event.From, senderId, transport.AssignedId)
}

func HandleRoleChange(uSock transport.UnicastSocket, event domain.Event,
	senderId int32, player RolePlayer) RolePlayer {
	uSock.SendAck(event.GameMessage, event.From, senderId, transport.AssignedId)
	roleCh := event.GameMessage.GetRoleChange()
	return player.HandleRoleChange(*roleCh.SenderRole, *roleCh.ReceiverRole)
}

type RolePlayer interface {
	HandleRoleChange(senderRole, receiverRole pb.NodeRole) RolePlayer
	Start()
	SetEngine(engine *Engine)
	SetId(id int32)
	SetGameContext()
}
