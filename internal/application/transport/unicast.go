package transport

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oxaxxaxaxaxaxaxxaaaxax/snake/internal/domain"
	pb "github.com/oxaxxaxaxaxaxaxxaaaxax/snake/proto"
	"google.golang.org/protobuf/proto"
	_ "google.golang.org/protobuf/proto"
)

const AssignedId = -1

var ErrAddressIsEmpty = errors.New("address is empty")

type PeerInfo struct {
	Acknowledges *PendingAcks
	LastRecv     time.Time
	LastSend     time.Time
	Dead         bool

	M sync.Mutex
}

type Peers struct {
	PeersInfo map[string]*PeerInfo
}

//func (p *Peers) Get(addr string) *PeerInfo {
//	if p.PeersInfo == nil {
//		p.PeersInfo = make(map[string]*PeerInfo)
//	}
//
//	if p.PeersInfo[addr] == nil {
//		p.PeersInfo[addr] = &PeerInfo{
//			Acknowledges: &PendingAcks{Acks: make(map[int64]*PendingAckMsg)},
//			LastRecv:     time.Time{},
//			LastSend:     time.Time{},
//		}
//	}
//
//	return p.PeersInfo[addr]
//}

type UnicastSocket struct {
	Conn net.PacketConn
	Seq  atomic.Int64
}

func NewUnicastSocket() (UnicastSocket, error) {
	conn, err := net.ListenPacket("udp", ":0")
	if err != nil {
		fmt.Println("Error creating socket:", err)
		return UnicastSocket{}, err
	}
	fmt.Println("NewUnicastSocket created")
	return UnicastSocket{Conn: conn}, nil
}

func (u *UnicastSocket) ListenMessage(evChan chan domain.Event, peers *Peers, peerMtx *sync.Mutex) {
	buf := make([]byte, 1024)

	for {
		n, addr, err := u.Conn.ReadFrom(buf)
		if err != nil {
			//fmt.Println("Error reading from socket:", err)
			return
		}
		//peers.PeersInfo[addr.String()].LastRecv = time.Now()
		//peer := peers.Get(addr.String())

		//peerMtx.Lock()
		//peer := peers.PeersInfo[addr.String()]
		//if peer == nil {
		//	peer = &PeerInfo{
		//		Acknowledges: &PendingAcks{Acks: make(map[int64]*PendingAckMsg)},
		//		LastRecv:     time.Time{},
		//		LastSend:     time.Time{},
		//	}
		//	peers.PeersInfo[addr.String()] = peer
		//}
		//peerMtx.Unlock()
		peer := peers.PeersInfo[addr.String()]
		if peer != nil {
			fmt.Println("NOT DISCOVER/ANNOUNCE MSG")
			peer.M.Lock()
			peer.LastRecv = time.Now()
			peer.M.Unlock()
		}

		message := &pb.GameMessage{}

		err = proto.Unmarshal(buf[:n], message)
		if err != nil {
			fmt.Println("Error unmarshalling from socket:", err)
			continue
		}

		//fmt.Printf("recv n=%d from=%s first=% x\n", n, addr.String(), buf[:min(n, 16)])
		evChan <- domain.Event{
			From:        addr.String(),
			GameMessage: message,
			Transport:   domain.Unicast,
		}
	}
}

func (u *UnicastSocket) SendMessage(message []byte, address string, peer *PeerInfo, peerMtx *sync.Mutex) error {
	recvAddr, err := net.ResolveUDPAddr("udp", address)
	_, err = u.Conn.WriteTo(message, recvAddr)
	if err != nil {
		return err
	}

	if peer == nil {
		fmt.Println("NIL")
	}
	//peerMtx.Lock()
	//if peer == nil {
	//	peer = &PeerInfo{
	//		Acknowledges: &PendingAcks{Acks: make(map[int64]*PendingAckMsg)},
	//		LastRecv:     time.Time{},
	//		LastSend:     time.Time{},
	//	}
	//}
	//peerMtx.Unlock()

	gMsg := &pb.GameMessage{}
	err = proto.Unmarshal(message, gMsg)
	if err != nil {
		fmt.Println("Error unmarshalling from socket:", err)
		return err
	}

	peer.M.Lock()
	peer.LastSend = time.Now()
	peer.M.Unlock()

	switch gMsg.Type.(type) {
	case *pb.GameMessage_Announcement, *pb.GameMessage_Discover, *pb.GameMessage_Ack:
		return nil
	default:
		msgSeq := *gMsg.MsgSeq
		peer.M.Lock()
		peer.Acknowledges.AppendAcks(PendingAckMsg{
			RequestMsg: message,
			SendTime:   time.Now(),
		}, msgSeq)
		peer.M.Unlock()
	}

	return nil
}

//остановилась на сенд месандж

func (u *UnicastSocket) SendAck(receiveMsg *pb.GameMessage, address string, senderId,
	receiverId int32, peer *PeerInfo, peerMtx *sync.Mutex) error {
	var msg []byte
	var err error
	if receiverId == AssignedId {
		id := int32(AssignedId)
		msg, err = proto.Marshal(&pb.GameMessage{
			MsgSeq:   receiveMsg.MsgSeq,
			SenderId: &senderId,
			//ReceiverId: receiveMsg.SenderId,
			ReceiverId: &id,
			Type:       &pb.GameMessage_Ack{},
		})
	} else {
		msg, err = proto.Marshal(&pb.GameMessage{
			MsgSeq:     receiveMsg.MsgSeq,
			SenderId:   &senderId,
			ReceiverId: &receiverId,
			Type:       &pb.GameMessage_Ack{},
		})
	}
	if err != nil {
		return err
	}
	recvAddr, err := net.ResolveUDPAddr("udp", address)
	fmt.Println("Send Ack to", address, receiverId)
	_, err = u.Conn.WriteTo(msg, recvAddr)
	if err != nil {
		return err
	}

	//peerMtx.Lock()
	if peer == nil {
		//этот кейс должен быть недостижимым
		fmt.Println("peer is nil (incorrect)")
		//peerMtx.Unlock()
		return nil
		//peer = &PeerInfo{
		//	Acknowledges: &PendingAcks{Acks: make(map[int64]*PendingAckMsg)},
		//	LastRecv:     time.Time{},
		//	LastSend:     time.Time{},
		//}
	}
	peer.M.Lock()
	peer.LastSend = time.Now()
	peer.M.Unlock()
	//peerMtx.Unlock()
	return nil
}

func (u *UnicastSocket) SendPing(address string, senderId int32, peer *PeerInfo, peerMtx *sync.Mutex) error {
	msgSeq := u.Seq.Add(1) - 1
	msg, err := proto.Marshal(&pb.GameMessage{
		MsgSeq:   &msgSeq,
		SenderId: &senderId,
		Type:     &pb.GameMessage_Ping{},
	})
	if err != nil {
		return err
	}
	recvAddr, err := net.ResolveUDPAddr("udp", address)
	_, err = u.Conn.WriteTo(msg, recvAddr)
	if err != nil {
		return err
	}

	//peerMtx.Lock()
	if peer == nil {
		//этот кейс должен быть недостижимым
		fmt.Println("peer is nil (incorrect)")
		//peerMtx.Unlock()
		return nil
		//peer = &PeerInfo{
		//	Acknowledges: &PendingAcks{Acks: make(map[int64]*PendingAckMsg)},
		//	LastRecv:     time.Time{},
		//	LastSend:     time.Time{},
		//}
	}
	peer.M.Lock()
	peer.LastSend = time.Now()
	peer.Acknowledges.AppendAcks(PendingAckMsg{
		RequestMsg: msg,
		SendTime:   time.Now(),
	}, msgSeq)
	peer.M.Unlock()
	//peerMtx.Unlock()
	return nil
}

func (u *UnicastSocket) SendSteer(address string, senderId int32, direction pb.Direction,
	peer *PeerInfo, peerMtx *sync.Mutex) error {
	msgSeq := u.Seq.Add(1) - 1
	msg, err := proto.Marshal(&pb.GameMessage{
		MsgSeq:   &msgSeq,
		SenderId: &senderId,
		Type: &pb.GameMessage_Steer{
			Steer: &pb.GameMessage_SteerMsg{
				Direction: &direction,
			},
		},
	})
	if err != nil {
		return err
	}
	recvAddr, err := net.ResolveUDPAddr("udp", address)
	_, err = u.Conn.WriteTo(msg, recvAddr)
	if err != nil {
		return err
	}

	//peerMtx.Lock()
	if peer == nil {
		//этот кейс должен быть недостижимым
		fmt.Println("peer is nil (incorrect)")
		//peerMtx.Unlock()
		return nil
		//peer = &PeerInfo{
		//	Acknowledges: &PendingAcks{Acks: make(map[int64]*PendingAckMsg)},
		//	LastRecv:     time.Time{},
		//	LastSend:     time.Time{},
		//}
	}
	peer.M.Lock()
	peer.LastSend = time.Now()
	peer.Acknowledges.AppendAcks(PendingAckMsg{
		RequestMsg: msg,
		SendTime:   time.Now(),
	}, msgSeq)
	peer.M.Unlock()
	//peerMtx.Unlock()
	return nil
}

func (u *UnicastSocket) SendState(address string, senderId int32, state *pb.GameState,
	peer *PeerInfo, peerMtx *sync.Mutex) error {
	msgSeq := u.Seq.Add(1) - 1
	msg, err := proto.Marshal(&pb.GameMessage{
		MsgSeq:   &msgSeq,
		SenderId: &senderId,
		Type: &pb.GameMessage_State{
			State: &pb.GameMessage_StateMsg{
				State: state,
			},
		},
	})
	if err != nil {
		return err
	}
	recvAddr, err := net.ResolveUDPAddr("udp", address)
	_, err = u.Conn.WriteTo(msg, recvAddr)
	if err != nil {
		return err
	}

	//peerMtx.Lock()
	if peer == nil {
		//этот кейс должен быть недостижимым
		fmt.Println("peer is nil (incorrect)")
		//peerMtx.Unlock()
		return nil
		//peer = &PeerInfo{
		//	Acknowledges: &PendingAcks{Acks: make(map[int64]*PendingAckMsg)},
		//	LastRecv:     time.Time{},
		//	LastSend:     time.Time{},
		//}
	}
	peer.M.Lock()
	peer.LastSend = time.Now()
	peer.Acknowledges.AppendAcks(PendingAckMsg{
		RequestMsg: msg,
		SendTime:   time.Now(),
	}, msgSeq)
	peer.M.Unlock()
	//peerMtx.Unlock()
	return nil
}

func (u *UnicastSocket) SendAnnouncement(address string, senderId int32,
	ann []*pb.GameAnnouncement) error {
	fmt.Println("address", address)
	msgSeq := u.Seq.Add(1) - 1
	msg, err := proto.Marshal(&pb.GameMessage{
		MsgSeq:   &msgSeq,
		SenderId: &senderId,
		Type: &pb.GameMessage_Announcement{
			Announcement: &pb.GameMessage_AnnouncementMsg{
				Games: ann,
			},
		},
	})
	if err != nil {
		return err
	}
	recvAddr, err := net.ResolveUDPAddr("udp", address)
	_, err = u.Conn.WriteTo(msg, recvAddr)
	if err != nil {
		return err
	}

	//пиров нет в announcement

	//peerMtx.Lock()
	//if peer == nil {
	//	//этот кейс должен быть недостижимым
	//	fmt.Println("peer is nil (incorrect)")
	//	peerMtx.Unlock()
	//	return nil
	//	//peer = &PeerInfo{
	//	//	Acknowledges: &PendingAcks{Acks: make(map[int64]*PendingAckMsg)},
	//	//	LastRecv:     time.Time{},
	//	//	LastSend:     time.Time{},
	//	//}
	//}
	//peer.M.Lock()
	//peer.LastSend = time.Now()
	//peer.M.Unlock()
	//peerMtx.Unlock()
	return nil
}

func (u *UnicastSocket) SendAnnouncementToMulticast(senderId int32, ann []*pb.GameAnnouncement) error {
	fmt.Println("ID Multicast from master", senderId)
	msgSeq := u.Seq.Add(1) - 1
	msg, err := proto.Marshal(&pb.GameMessage{
		MsgSeq:   &msgSeq,
		SenderId: &senderId,
		Type: &pb.GameMessage_Announcement{
			Announcement: &pb.GameMessage_AnnouncementMsg{
				Games: ann,
			},
		},
	})
	if err != nil {
		return err
	}
	multicastAddr := fmt.Sprintf("%v:%v", Addr, Port)
	recvAddr, err := net.ResolveUDPAddr("udp", multicastAddr)
	_, err = u.Conn.WriteTo(msg, recvAddr)
	if err != nil {
		return err
	}
	return nil
}

func (u *UnicastSocket) SendDiscover(senderId int32) error {
	msgSeq := u.Seq.Add(1) - 1
	msg, err := proto.Marshal(&pb.GameMessage{
		MsgSeq:   &msgSeq,
		SenderId: &senderId,
		Type:     &pb.GameMessage_Discover{},
	})
	if err != nil {
		fmt.Println(err)
		return err
	}
	address := fmt.Sprintf("%v:%v", Addr, Port)
	fmt.Println(address)
	recvAddr, err := net.ResolveUDPAddr("udp", address)
	if u.Conn == nil {
		return fmt.Errorf("unicast socket conn is nil (NewUnicastSocket not called?)")
	}

	_, err = u.Conn.WriteTo(msg, recvAddr)
	if err != nil {
		return err
	}
	fmt.Println("Send Discover!")
	return nil
}

func (u *UnicastSocket) SendJoin(address string, ctx domain.GameCtx, peer *PeerInfo, peerMtx *sync.Mutex) error {
	msgSeq := u.Seq.Add(1) - 1
	msg, err := proto.Marshal(&pb.GameMessage{
		MsgSeq:   &msgSeq,
		SenderId: &ctx.PlayerID,
		Type: &pb.GameMessage_Join{
			Join: &pb.GameMessage_JoinMsg{
				PlayerType:    &ctx.PlayerType,
				PlayerName:    &ctx.PlayerName,
				RequestedRole: &ctx.Role,
				GameName:      &ctx.GameName,
			},
		},
	})
	if err != nil {
		return err
	}
	recvAddr, err := net.ResolveUDPAddr("udp", address)
	_, err = u.Conn.WriteTo(msg, recvAddr)
	if err != nil {
		return err
	}

	peerMtx.Lock()
	if peer == nil {
		fmt.Println("peer is nil in join(incorrect)")
		return nil
		//fmt.Println("peer is nil (correct)")
		//peer = &PeerInfo{
		//	Acknowledges: &PendingAcks{Acks: make(map[int64]*PendingAckMsg)},
		//	LastRecv:     time.Time{},
		//	LastSend:     time.Time{},
		//}
	}
	fmt.Println("peer is not nil in join ")

	peer.LastSend = time.Now()
	peer.Acknowledges.AppendAcks(PendingAckMsg{
		RequestMsg: msg,
		SendTime:   time.Now(),
	}, msgSeq)
	peerMtx.Unlock()
	return nil
}

func (u *UnicastSocket) SendError(address string, senderId int32, errorMsg string, peer *PeerInfo, peerMtx *sync.Mutex) error {
	msgSeq := u.Seq.Add(1) - 1
	msg, err := proto.Marshal(&pb.GameMessage{
		MsgSeq:   &msgSeq,
		SenderId: &senderId,
		Type: &pb.GameMessage_Error{
			Error: &pb.GameMessage_ErrorMsg{
				ErrorMessage: &errorMsg,
			},
		},
	})
	if err != nil {
		return err
	}
	recvAddr, err := net.ResolveUDPAddr("udp", address)
	_, err = u.Conn.WriteTo(msg, recvAddr)
	if err != nil {
		return err
	}

	//peerMtx.Lock()
	if peer == nil {
		//этот кейс должен быть недостижимым
		fmt.Println("peer is nil (incorrect)")
		//peerMtx.Unlock()
		return nil
		//peer = &PeerInfo{
		//	Acknowledges: &PendingAcks{Acks: make(map[int64]*PendingAckMsg)},
		//	LastRecv:     time.Time{},
		//	LastSend:     time.Time{},
		//}
	}
	//peerMtx.Unlock()
	peer.M.Lock()
	peer.LastSend = time.Now()
	peer.Acknowledges.AppendAcks(PendingAckMsg{
		RequestMsg: msg,
		SendTime:   time.Now(),
	}, msgSeq)
	peer.M.Unlock()
	//peerMtx.Unlock()

	return nil
}

func (u *UnicastSocket) SendRoleChange(address string, senderId int32, recvRole,
	sendRole pb.NodeRole, peer *PeerInfo, peerMtx *sync.Mutex) error {
	msgSeq := u.Seq.Add(1) - 1
	msg, err := proto.Marshal(&pb.GameMessage{
		MsgSeq:   &msgSeq,
		SenderId: &senderId,
		Type: &pb.GameMessage_RoleChange{
			RoleChange: &pb.GameMessage_RoleChangeMsg{
				ReceiverRole: &recvRole,
				SenderRole:   &sendRole,
			},
		},
	})
	if err != nil {
		return err
	}
	recvAddr, err := net.ResolveUDPAddr("udp", address)
	_, err = u.Conn.WriteTo(msg, recvAddr)
	if err != nil {
		return err
	}

	//peerMtx.Lock()
	if peer == nil {
		//этот кейс должен быть недостижимым
		fmt.Println("peer is nil (incorrect)")
		//peerMtx.Unlock()
		return nil
		//peer = &PeerInfo{
		//	Acknowledges: &PendingAcks{Acks: make(map[int64]*PendingAckMsg)},
		//	LastRecv:     time.Time{},
		//	LastSend:     time.Time{},
		//}
	}
	peer.M.Lock()
	peer.LastSend = time.Now()
	peer.Acknowledges.AppendAcks(PendingAckMsg{
		RequestMsg: msg,
		SendTime:   time.Now(),
	}, msgSeq)
	peer.M.Unlock()
	//peerMtx.Unlock()
	return nil
}
