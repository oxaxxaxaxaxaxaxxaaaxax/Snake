package application

import (
	"errors"
	"fmt"
	"time"

	"github.com/oxaxxaxaxaxaxaxxaaaxax/snake/internal/application/transport"
	"github.com/oxaxxaxaxaxaxaxxaaaxax/snake/internal/domain"
	pb "github.com/oxaxxaxaxaxaxaxxaaaxax/snake/proto"
)

const (
	UninitializedId     = -1
	DefaultStateMsDelay = 1000
)

var (
	ErrUnknownMessageType = errors.New("unknown message type")
)

type Engine struct {
	GameCtx *domain.GameCtx

	USock transport.UnicastSocket
	MSock transport.MulticastSocket

	//Acknowledges *PendingAcks
	Peers *transport.Peers

	EventChan chan domain.Event
}

func (e *Engine) StartGame(player RolePlayer) {
	//e.Acknowledges = NewPendingAcks()
	e.Peers = &transport.Peers{}
	e.Peers.PeersInfo = make(map[string]*transport.PeerInfo)
	e.EventChan = make(chan domain.Event, 256)

	e.GameCtx = &domain.GameCtx{}
	//e.GameCtx.PlayerID = UninitializedId

	e.USock = transport.NewUnicastSocket()
	e.MSock = transport.NewMulticastSocket()
	player.SetEngine(e)
	player.SetGameContext()

	go e.USock.ListenMessage(e.EventChan, e.Peers)
	go e.MSock.ReadFromMulticastSocket(e.EventChan)

	go player.Start()
	go player.StartNetTicker()

	for {
		ev := <-e.EventChan
		fmt.Println("new event!")
		HandleMessage(e.USock, ev, e.GameCtx, player, e.Peers)
	}
}

func HandleMessage(uSock transport.UnicastSocket, event domain.Event,
	gameCtx *domain.GameCtx, player RolePlayer, peers *transport.Peers) (RolePlayer, error) {
	message := event.GameMessage

	switch message.Type.(type) {
	case *pb.GameMessage_Ping:
		HandlePing(uSock, event, gameCtx.PlayerID, peers)
		return player, nil
	case *pb.GameMessage_Steer:
		HandleSteer(uSock, event, gameCtx.PlayerID, peers)
		return player, nil
	case *pb.GameMessage_Ack:
		HandleAck(uSock, event, gameCtx.PlayerID, player, peers)
		return player, nil
	case *pb.GameMessage_State:
		HandleState(uSock, event, gameCtx.PlayerID, peers)
		return player, nil
	case *pb.GameMessage_Announcement:
		HandleAnnouncement(uSock, event, gameCtx, peers)
		return player, nil
	case *pb.GameMessage_Discover:
		HandleDiscover(uSock, event, gameCtx, player, peers)
		return player, nil
	case *pb.GameMessage_Join:
		HandleJoin(uSock, event, gameCtx.PlayerID, player, peers)
		return player, nil
	case *pb.GameMessage_Error:
		HandleError(uSock, event, gameCtx.PlayerID, peers)
		return player, nil
	case *pb.GameMessage_RoleChange:
		return HandleRoleChange(uSock, event, gameCtx.PlayerID, player, peers), nil
	default:
		return nil, ErrUnknownMessageType
	}
}

func HandlePing(uSock transport.UnicastSocket, event domain.Event, senderId int32, peers *transport.Peers) {
	fmt.Println("Handle ping")
	uSock.SendAck(event.GameMessage, event.From, senderId, transport.AssignedId, peers.PeersInfo[event.From])
}

func HandleSteer(uSock transport.UnicastSocket, event domain.Event, senderId int32, peers *transport.Peers) {
	uSock.SendAck(event.GameMessage, event.From, senderId, transport.AssignedId, peers.PeersInfo[event.From])
}

func HandleAck(uSock transport.UnicastSocket, event domain.Event, senderId int32, player RolePlayer, peers *transport.Peers) {
	fmt.Println("Handle Ack")
	if *event.GameMessage.ReceiverId != transport.AssignedId {
		fmt.Println("My Id:", *event.GameMessage.ReceiverId)
		player.SetId(*event.GameMessage.ReceiverId)
		switch player.(type) {
		case *Normal:
			n := player.(*Normal)
			n.AddMasterInfo(event.From, *event.GameMessage.SenderId)
		case *Viewer:
			v := player.(*Viewer)
			v.AddMasterInfo(event.From, *event.GameMessage.SenderId)
		default:
			fmt.Println("Unknown player type!")
		}

		//сделать добавление пира
		//???????????
		return
	}
	//ackMsg, err := acks.GetPendingAckById(*event.GameMessage.SenderId)
	//if err != nil {
	//	fmt.Println("AcK err! ", err)
	//}
	//if ackMsg.RequestMsg.MsgSeq == event.GameMessage.MsgSeq {
	//	acks.RemoveAcks(ackMsg)
	//}
	var err error
	var ok bool
	if ok, err = peers.PeersInfo[event.From].Acknowledges.RemoveAcks(*event.GameMessage.MsgSeq); ok {
		fmt.Println("Remove Ack:", *event.GameMessage.MsgSeq)
		return
	}
	fmt.Println(err)
}

func HandleState(uSock transport.UnicastSocket, event domain.Event, senderId int32, peers *transport.Peers) {
	uSock.SendAck(event.GameMessage, event.From, senderId, transport.AssignedId, peers.PeersInfo[event.From])
}

func HandleAnnouncement(uSock transport.UnicastSocket, event domain.Event,
	gameCtx *domain.GameCtx, peers *transport.Peers) {
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
	err := uSock.SendJoin(event.From, *gameCtx, peers.PeersInfo[event.From])
	if err != nil {
		fmt.Println("Join err! ", err)
	}
	return
}

func HandleDiscover(uSock transport.UnicastSocket, event domain.Event, gameCtx *domain.GameCtx,
	player RolePlayer, peers *transport.Peers) {
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
		m.SendAnnouncement(event.From, peers.PeersInfo[event.From])
	default:
		fmt.Println("DISCOVER FROM NOT MASTER")
	}

	//m := player.(*Master)
}

func HandleJoin(uSock transport.UnicastSocket, event domain.Event,
	senderId int32, player RolePlayer, peers *transport.Peers) {
	fmt.Println("Receive Join")
	m := player.(*Master)
	switch *event.GameMessage.GetJoin().RequestedRole {
	case pb.NodeRole_NORMAL:
		if !m.PlaceSnake() {
			errorMsg := "Cannot join game: no place for snake"
			err := uSock.SendError(event.From, senderId, errorMsg, peers.PeersInfo[event.From])
			if err != nil {
				fmt.Println("Error err! ", err)
			}
			return
		}
		fmt.Println("Master placed snake!")
	}
	peers.PeersInfo[event.From] = &transport.PeerInfo{
		Acknowledges: &transport.PendingAcks{Acks: make(map[int64]*transport.PendingAckMsg)},
		LastRecv:     time.Time{},
		LastSend:     time.Time{},
	}
	m.SendJoinAck(event.GameMessage, event.From, peers.PeersInfo[event.From])

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

func HandleError(uSock transport.UnicastSocket, event domain.Event, senderId int32, peer *transport.Peers) {
	uSock.SendAck(event.GameMessage, event.From, senderId, transport.AssignedId, peer.PeersInfo[event.From])
}

func HandleRoleChange(uSock transport.UnicastSocket, event domain.Event,
	senderId int32, player RolePlayer, peer *transport.Peers) RolePlayer {
	fmt.Println("Handle RoleChange")
	uSock.SendAck(event.GameMessage, event.From, senderId, transport.AssignedId, peer.PeersInfo[event.From])
	roleCh := event.GameMessage.GetRoleChange()
	return player.HandleRoleChange(*roleCh.SenderRole, *roleCh.ReceiverRole, peer.PeersInfo[event.From])
}

type RolePlayer interface {
	HandleRoleChange(senderRole, receiverRole pb.NodeRole, peer *transport.PeerInfo) RolePlayer
	Start()
	SetEngine(engine *Engine)
	SetId(id int32)
	SetGameContext()
	StartNetTicker()
	CheckTimeoutInteraction(interval, recvInterval int32)
}
