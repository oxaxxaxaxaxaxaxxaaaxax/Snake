package transport

import (
	"errors"
	"fmt"
	"net"
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
}

type Peers struct {
	PeersInfo map[string]*PeerInfo
}

func (p *Peers) Get(addr string) *PeerInfo {
	if p.PeersInfo == nil {
		p.PeersInfo = make(map[string]*PeerInfo)
	}

	if p.PeersInfo[addr] == nil {
		p.PeersInfo[addr] = &PeerInfo{
			Acknowledges: &PendingAcks{Acks: make(map[int64]*PendingAckMsg)},
			LastRecv:     time.Time{},
			LastSend:     time.Time{},
		}
	}

	return p.PeersInfo[addr]
}

type UnicastSocket struct {
	Conn net.PacketConn
	Seq  int64
}

func NewUnicastSocket() UnicastSocket {
	conn, err := net.ListenPacket("udp", ":0")
	if err != nil {
		fmt.Println("Error creating socket:", err)
		return UnicastSocket{}
	}
	fmt.Println("NewUnicastSocket created")
	return UnicastSocket{Conn: conn}
}

func (u *UnicastSocket) ListenMessage(evChan chan domain.Event, peers *Peers) {
	buf := make([]byte, 1024)

	for {
		n, addr, err := u.Conn.ReadFrom(buf)
		if err != nil {
			//fmt.Println("Error reading from socket:", err)
			return
		}
		//peers.PeersInfo[addr.String()].LastRecv = time.Now()
		//peer := peers.Get(addr.String())

		peer := peers.PeersInfo[addr.String()]

		if peer == nil {
			peer = &PeerInfo{
				Acknowledges: &PendingAcks{Acks: make(map[int64]*PendingAckMsg)},
				LastRecv:     time.Time{},
				LastSend:     time.Time{},
			}
		}

		peer.LastRecv = time.Now()
		message := &pb.GameMessage{}

		err = proto.Unmarshal(buf[:n], message)
		if err != nil {
			fmt.Println("Error unmarshalling from socket:", err)
			continue
		}

		fmt.Printf("recv n=%d from=%s first=% x\n", n, addr.String(), buf[:min(n, 16)])
		evChan <- domain.Event{
			From:        addr.String(),
			GameMessage: message,
			Transport:   domain.Unicast,
		}
	}
}

func (u *UnicastSocket) SendMessage(message []byte, address string, peer *PeerInfo) error {
	recvAddr, err := net.ResolveUDPAddr("udp", address)
	_, err = u.Conn.WriteTo(message, recvAddr)
	if err != nil {
		return err
	}

	if peer == nil {
		peer = &PeerInfo{
			Acknowledges: &PendingAcks{Acks: make(map[int64]*PendingAckMsg)},
			LastRecv:     time.Time{},
			LastSend:     time.Time{},
		}
	}

	gMsg := &pb.GameMessage{}
	err = proto.Unmarshal(message, gMsg)
	if err != nil {
		fmt.Println("Error unmarshalling from socket:", err)
		return err
	}

	peer.LastSend = time.Now()

	switch gMsg.Type.(type) {
	case *pb.GameMessage_Announcement, *pb.GameMessage_Discover, *pb.GameMessage_Ack:
		return nil
	default:
		peer.Acknowledges.AppendAcks(PendingAckMsg{
			RequestMsg: message,
			SendTime:   time.Now(),
		}, u.Seq)
		u.Seq++
	}

	return nil
}

func (u *UnicastSocket) SendAck(receiveMsg *pb.GameMessage, address string, senderId,
	receiverId int32, peer *PeerInfo) error {
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
	fmt.Println("Send Ack to", address)
	_, err = u.Conn.WriteTo(msg, recvAddr)
	if err != nil {
		return err
	}

	if peer == nil {
		peer = &PeerInfo{
			Acknowledges: &PendingAcks{Acks: make(map[int64]*PendingAckMsg)},
			LastRecv:     time.Time{},
			LastSend:     time.Time{},
		}
	}

	peer.LastSend = time.Now()
	return nil
}

func (u *UnicastSocket) SendPing(address string, senderId int32, peer *PeerInfo) error {
	msg, err := proto.Marshal(&pb.GameMessage{
		MsgSeq:   &u.Seq,
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

	if peer == nil {
		peer = &PeerInfo{
			Acknowledges: &PendingAcks{Acks: make(map[int64]*PendingAckMsg)},
			LastRecv:     time.Time{},
			LastSend:     time.Time{},
		}
	}

	peer.LastSend = time.Now()
	peer.Acknowledges.AppendAcks(PendingAckMsg{
		RequestMsg: msg,
		SendTime:   time.Now(),
	}, u.Seq)
	u.Seq++
	return nil
}

func (u *UnicastSocket) SendSteer(address string, senderId int32, direction pb.Direction, peer *PeerInfo) error {
	msg, err := proto.Marshal(&pb.GameMessage{
		MsgSeq:   &u.Seq,
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

	if peer == nil {
		peer = &PeerInfo{
			Acknowledges: &PendingAcks{Acks: make(map[int64]*PendingAckMsg)},
			LastRecv:     time.Time{},
			LastSend:     time.Time{},
		}
	}

	peer.LastSend = time.Now()
	peer.Acknowledges.AppendAcks(PendingAckMsg{
		RequestMsg: msg,
		SendTime:   time.Now(),
	}, u.Seq)
	u.Seq++
	return nil
}

func (u *UnicastSocket) SendState(address string, senderId int32, state *pb.GameState, peer *PeerInfo) error {
	msg, err := proto.Marshal(&pb.GameMessage{
		MsgSeq:   &u.Seq,
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

	if peer == nil {
		peer = &PeerInfo{
			Acknowledges: &PendingAcks{Acks: make(map[int64]*PendingAckMsg)},
			LastRecv:     time.Time{},
			LastSend:     time.Time{},
		}
	}

	peer.LastSend = time.Now()
	peer.Acknowledges.AppendAcks(PendingAckMsg{
		RequestMsg: msg,
		SendTime:   time.Now(),
	}, u.Seq)
	u.Seq++
	return nil
}

func (u *UnicastSocket) SendAnnouncement(address string, senderId int32,
	ann []*pb.GameAnnouncement, peer *PeerInfo) error {
	fmt.Println("address", address)
	msg, err := proto.Marshal(&pb.GameMessage{
		MsgSeq:   &u.Seq,
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

	if peer == nil {
		peer = &PeerInfo{
			Acknowledges: &PendingAcks{Acks: make(map[int64]*PendingAckMsg)},
			LastRecv:     time.Time{},
			LastSend:     time.Time{},
		}
	}

	peer.LastSend = time.Now()
	u.Seq++
	return nil
}

func (u *UnicastSocket) SendAnnouncementToMulticast(senderId int32, ann []*pb.GameAnnouncement) error {
	fmt.Println("ID Multicast from master", senderId)
	msg, err := proto.Marshal(&pb.GameMessage{
		MsgSeq:   &u.Seq,
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
	u.Seq++
	return nil
}

func (u *UnicastSocket) SendDiscover(senderId int32) error {
	msg, err := proto.Marshal(&pb.GameMessage{
		MsgSeq:   &u.Seq,
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
	u.Seq++
	return nil
}

func (u *UnicastSocket) SendJoin(address string, ctx domain.GameCtx, peer *PeerInfo) error {
	msg, err := proto.Marshal(&pb.GameMessage{
		MsgSeq:   &u.Seq,
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

	if peer == nil {
		peer = &PeerInfo{
			Acknowledges: &PendingAcks{Acks: make(map[int64]*PendingAckMsg)},
			LastRecv:     time.Time{},
			LastSend:     time.Time{},
		}
	}

	peer.LastSend = time.Now()
	peer.Acknowledges.AppendAcks(PendingAckMsg{
		RequestMsg: msg,
		SendTime:   time.Now(),
	}, u.Seq)
	u.Seq++
	return nil
}

func (u *UnicastSocket) SendError(address string, senderId int32, errorMsg string, peer *PeerInfo) error {
	msg, err := proto.Marshal(&pb.GameMessage{
		MsgSeq:   &u.Seq,
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

	if peer == nil {
		peer = &PeerInfo{
			Acknowledges: &PendingAcks{Acks: make(map[int64]*PendingAckMsg)},
			LastRecv:     time.Time{},
			LastSend:     time.Time{},
		}
	}

	peer.LastSend = time.Now()
	peer.Acknowledges.AppendAcks(PendingAckMsg{
		RequestMsg: msg,
		SendTime:   time.Now(),
	}, u.Seq)
	u.Seq++
	return nil
}

func (u *UnicastSocket) SendRoleChange(address string, senderId int32, recvRole,
	sendRole pb.NodeRole, peer *PeerInfo) error {
	msg, err := proto.Marshal(&pb.GameMessage{
		MsgSeq:   &u.Seq,
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

	if peer == nil {
		peer = &PeerInfo{
			Acknowledges: &PendingAcks{Acks: make(map[int64]*PendingAckMsg)},
			LastRecv:     time.Time{},
			LastSend:     time.Time{},
		}
	}

	peer.LastSend = time.Now()
	peer.Acknowledges.AppendAcks(PendingAckMsg{
		RequestMsg: msg,
		SendTime:   time.Now(),
	}, u.Seq)
	u.Seq++
	return nil
}
