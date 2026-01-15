package application

import (
	"errors"
	"fmt"
	"sync"
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
	//Acknowledges *PendingAcks
	Peers *transport.Peers

	//насколько тут rw я хз
	PeerMtx sync.Mutex
	GameMtx sync.Mutex

	USock transport.UnicastSocket
	MSock transport.MulticastSocket

	EventChan chan domain.Event
}

func (e *Engine) StartGame(player RolePlayer) {
	var err error
	//e.Acknowledges = NewPendingAcks()
	e.Peers = &transport.Peers{}
	e.Peers.PeersInfo = make(map[string]*transport.PeerInfo)
	e.EventChan = make(chan domain.Event, 256)

	//e.GameCtx = &domain.GameCtx{}
	//e.GameCtx.PlayerID = UninitializedId

	e.USock, err = transport.NewUnicastSocket()
	if err != nil {
		return
	}
	e.MSock, err = transport.NewMulticastSocket()
	if err != nil {
		return
	}
	player.SetEngine(e)
	player.SetGameContext()

	go e.USock.ListenMessage(e.EventChan, e.Peers, &e.PeerMtx)
	go e.MSock.ReadFromMulticastSocket(e.EventChan)

	go player.Start()
	//go player.StartNetTicker()
	go e.StartNetTicker()

	for {
		ev := <-e.EventChan
		//fmt.Println("new event!")
		if ev.IsTick {
			player = player.CheckTimeoutInteraction(ev.Interval, ev.RecvInterval)
			continue
		}
		player, err = HandleMessage(&e.USock, ev, e.GameCtx, player, e.Peers, &e.GameMtx, &e.PeerMtx)
		if err != nil {
			fmt.Println("err in event loop!", err)
		}
	}
}

func (e *Engine) StartNetTicker() {
	interval := e.GameCtx.StateMsDelay / 10
	recvInterval := interval * 8

	ticker := time.NewTicker(time.Duration(interval) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			e.EventChan <- domain.Event{IsTick: true, Interval: interval, RecvInterval: recvInterval}
		}
	}
}

func HandleMessage(uSock *transport.UnicastSocket, event domain.Event, gameCtx *domain.GameCtx,
	player RolePlayer, peers *transport.Peers, gameMtx *sync.Mutex, peerMtx *sync.Mutex) (RolePlayer, error) {
	message := event.GameMessage

	switch message.Type.(type) {
	case *pb.GameMessage_Ping:
		HandlePing(uSock, event, gameCtx.PlayerID, peers, peerMtx)
		return player, nil
	case *pb.GameMessage_Steer:
		HandleSteer(uSock, event, gameCtx.PlayerID, peers, peerMtx)
		return player, nil
	case *pb.GameMessage_Ack:
		HandleAck(event, player, peers, peerMtx)
		return player, nil
	case *pb.GameMessage_State:
		HandleState(uSock, event, gameCtx.PlayerID, peers, peerMtx)
		return player, nil
	case *pb.GameMessage_Announcement:
		HandleAnnouncement(uSock, event, gameCtx, peers, gameMtx, peerMtx)
		return player, nil
	case *pb.GameMessage_Discover:
		HandleDiscover(event, gameCtx, player, gameMtx)
		return player, nil
	case *pb.GameMessage_Join:
		HandleJoin(uSock, event, gameCtx.PlayerID, player, peers, peerMtx)
		return player, nil
	case *pb.GameMessage_Error:
		HandleError(uSock, event, gameCtx.PlayerID, peers, peerMtx)
		return player, nil
	case *pb.GameMessage_RoleChange:
		return HandleRoleChange(uSock, event, gameCtx.PlayerID, player, peers, peerMtx), nil
	default:
		return nil, ErrUnknownMessageType
	}
}

func HandlePing(uSock *transport.UnicastSocket, event domain.Event, senderId int32,
	peers *transport.Peers, peerMtx *sync.Mutex) {
	fmt.Println("Handle ping")
	peerMtx.Lock()
	peer := peers.PeersInfo[event.From]
	peerMtx.Unlock()

	err := uSock.SendAck(event.GameMessage, event.From, senderId, transport.AssignedId,
		peer, peerMtx)
	if err != nil {
		fmt.Println("err in handle ping!", err)
	}
}

func HandleSteer(uSock *transport.UnicastSocket, event domain.Event, senderId int32,
	peers *transport.Peers, peerMtx *sync.Mutex) {
	peerMtx.Lock()
	peer := peers.PeersInfo[event.From]
	peerMtx.Unlock()

	err := uSock.SendAck(event.GameMessage, event.From, senderId, transport.AssignedId,
		peer, peerMtx)
	if err != nil {
		fmt.Println("err in handle steer!", err)
	}
}

func HandleAck(event domain.Event, player RolePlayer, peers *transport.Peers, peerMtx *sync.Mutex) {
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
			d := player.(*Deputy)
			d.AddMasterInfo(event.From, *event.GameMessage.SenderId)
		}
		return
	}
	var err error
	var ok bool

	peerMtx.Lock()
	peer := peers.PeersInfo[event.From]
	peerMtx.Unlock()
	//наврядли акноледж придет nil но напишу пометку на всякий случай
	peer.M.Lock()
	if peer.Acknowledges == nil {
		fmt.Println("Logic error Acknowledges")
		return
	}
	if ok, err = peer.Acknowledges.RemoveAcks(*event.GameMessage.MsgSeq); ok {
		fmt.Println("Remove Ack:", *event.GameMessage.MsgSeq)
		peer.M.Unlock()
		return
	}
	peer.M.Unlock()
	fmt.Println("Remove Ack error - message already delete:", err)
}

func HandleState(uSock *transport.UnicastSocket, event domain.Event, senderId int32,
	peers *transport.Peers, peerMtx *sync.Mutex) {
	peerMtx.Lock()
	peer := peers.PeersInfo[event.From]
	peerMtx.Unlock()

	err := uSock.SendAck(event.GameMessage, event.From, senderId, transport.AssignedId,
		peer, peerMtx)
	if err != nil {
		fmt.Println("err in handle state!", err)
	}
}

func HandleAnnouncement(uSock *transport.UnicastSocket, event domain.Event,
	gameCtx *domain.GameCtx, peers *transport.Peers, gameMtx *sync.Mutex, peerMtx *sync.Mutex) {
	//fmt.Println("Receive Announcement")
	//fmt.Println("my id", gameCtx.PlayerID)
	gameMtx.Lock()
	if gameCtx.PlayerID == *event.GameMessage.SenderId {
		//fmt.Println("It's my message, skip")
		gameMtx.Unlock()
		return
	}
	gameCtx.GameName = *event.GameMessage.GetAnnouncement().Games[0].GameName

	fmt.Println("Announcement", gameCtx.GameName)
	if event.Transport == domain.Multicast {
		gameMtx.Unlock()
		return
	}
	fmt.Println("role join", gameCtx.Role)
	ctx := *gameCtx
	gameMtx.Unlock()

	peerMtx.Lock()
	if peers.PeersInfo[event.From] == nil {
		fmt.Println("nil peer and new player(correct)")
		peers.PeersInfo[event.From] = &transport.PeerInfo{
			Acknowledges: &transport.PendingAcks{Acks: make(map[int64]*transport.PendingAckMsg)},
			LastRecv:     time.Time{},
			LastSend:     time.Time{},
		}
	}

	peer := peers.PeersInfo[event.From]
	peerMtx.Unlock()

	err := uSock.SendJoin(event.From, ctx, peer, peerMtx)
	if err != nil {
		fmt.Println("Join err! ", err)
	}
}

func HandleDiscover(event domain.Event, gameCtx *domain.GameCtx,
	player RolePlayer, gameMtx *sync.Mutex) {
	gameMtx.Lock()
	fmt.Println("Recieve Discover:", gameCtx.GameName, "from", event.From)
	fmt.Println(gameCtx.PlayerID, *event.GameMessage.SenderId)
	if gameCtx.PlayerID == *event.GameMessage.SenderId {
		fmt.Println("It's my message, skip")
		gameMtx.Unlock()
		return
	}
	gameMtx.Unlock()

	switch m := player.(type) {
	case *Master:
		//peerMtx.Lock()
		//peer := peers.PeersInfo[event.From]
		//if peer == nil {
		//	fmt.Println("peer is nil(correct)")
		//	peer = &transport.PeerInfo{
		//		Acknowledges: &transport.PendingAcks{Acks: make(map[int64]*transport.PendingAckMsg)},
		//		LastRecv:     time.Time{},
		//		LastSend:     time.Time{},
		//	}
		//	peers.PeersInfo[event.From] = peer
		//}
		//peerMtx.Unlock()

		m.SendAnnouncement(event.From)
	default:
		fmt.Println("DISCOVER FROM NOT MASTER")
	}
}

func HandleJoin(uSock *transport.UnicastSocket, event domain.Event,
	senderId int32, player RolePlayer, peers *transport.Peers, peerMtx *sync.Mutex) {
	fmt.Println("Receive Join")
	m := player.(*Master)
	if m.Contains(event.From) {
		fmt.Println("This player exist!!")
		peerMtx.Lock()
		//если игрок существкет то и пир его уже существует
		if peers.PeersInfo[event.From] == nil {
			fmt.Println("nil peer and exist player(incorrect)")
		}
		//peers.PeersInfo[event.From] = &transport.PeerInfo{
		//	Acknowledges: &transport.PendingAcks{Acks: make(map[int64]*transport.PendingAckMsg)},
		//	LastRecv:     time.Time{},
		//	LastSend:     time.Time{},
		//}
		//
		peer := peers.PeersInfo[event.From]
		peerMtx.Unlock()

		err := m.Engine.USock.SendAck(event.GameMessage, event.From, senderId, transport.AssignedId,
			peer, peerMtx)
		if err != nil {
			fmt.Println("In handle join ", err)
		}
		return
	}
	switch *event.GameMessage.GetJoin().RequestedRole {
	case pb.NodeRole_NORMAL:
		if !m.PlaceSnake() {
			errorMsg := "Cannot join game: no place for snake"
			peerMtx.Lock()
			//а вот тут надо создать
			if peers.PeersInfo[event.From] == nil {
				fmt.Println("nil peer and new player(correct)")
				peers.PeersInfo[event.From] = &transport.PeerInfo{
					Acknowledges: &transport.PendingAcks{Acks: make(map[int64]*transport.PendingAckMsg)},
					LastRecv:     time.Time{},
					LastSend:     time.Time{},
				}
			}

			peer := peers.PeersInfo[event.From]
			peerMtx.Unlock()

			err := uSock.SendError(event.From, senderId, errorMsg, peer, peerMtx)
			if err != nil {
				fmt.Println("Error err! ", err)
			}
			return
		}
		fmt.Println("Master placed snake!")
	}
	peerMtx.Lock()
	if peers.PeersInfo[event.From] == nil {
		peers.PeersInfo[event.From] = &transport.PeerInfo{
			Acknowledges: &transport.PendingAcks{Acks: make(map[int64]*transport.PendingAckMsg)},
			LastRecv:     time.Time{},
			LastSend:     time.Time{},
		}
	}
	peer := peers.PeersInfo[event.From]
	peerMtx.Unlock()

	err := m.SendJoinAck(event.GameMessage, event.From, peer, peerMtx)
	if err != nil {
		fmt.Println("In handle join ", err)
	}

}

func HandleError(uSock *transport.UnicastSocket, event domain.Event, senderId int32,
	peers *transport.Peers, peerMtx *sync.Mutex) {
	peerMtx.Lock()
	peer := peers.PeersInfo[event.From]
	peerMtx.Unlock()

	err := uSock.SendAck(event.GameMessage, event.From, senderId, transport.AssignedId, peer, peerMtx)
	if err != nil {
		fmt.Println("In handle error ", err)
	}
}

func HandleRoleChange(uSock *transport.UnicastSocket, event domain.Event,
	senderId int32, player RolePlayer, peers *transport.Peers, peerMtx *sync.Mutex) RolePlayer {
	fmt.Println("Handle RoleChange")
	peerMtx.Lock()
	peer := peers.PeersInfo[event.From]
	peerMtx.Unlock()

	err := uSock.SendAck(event.GameMessage, event.From, senderId, transport.AssignedId, peer, peerMtx)
	if err != nil {
		fmt.Println("In handle role change ", err)
	}
	roleCh := event.GameMessage.GetRoleChange()
	//тут пир( с ним ничего не делается) не нужен но пока оставлю
	return player.HandleRoleChange(*roleCh.SenderRole, *roleCh.ReceiverRole, peer)
}

type RolePlayer interface {
	HandleRoleChange(senderRole, receiverRole pb.NodeRole, peer *transport.PeerInfo) RolePlayer
	Start()
	SetEngine(engine *Engine)
	SetId(id int32)
	SetGameContext()
	StartNetTicker()
	CheckTimeoutInteraction(interval, recvInterval int32) RolePlayer
}
