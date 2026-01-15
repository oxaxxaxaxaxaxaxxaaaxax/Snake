package transport

import (
	"fmt"
	"net"

	"github.com/oxaxxaxaxaxaxaxxaaaxax/snake/internal/domain"
	pb "github.com/oxaxxaxaxaxaxaxxaaaxax/snake/proto"
	"google.golang.org/protobuf/proto"
)

const (
	Addr string = "239.192.0.4"
	Port int    = 9192
)

type MulticastSocket struct {
	Conn net.UDPConn
}

func NewMulticastSocket() (MulticastSocket, error) {
	address := fmt.Sprintf("%v:%v", Addr, Port)
	sockAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return MulticastSocket{}, err
	}
	conn, err := net.ListenMulticastUDP("udp", nil, sockAddr)
	if err != nil {
		return MulticastSocket{}, err
	}
	fmt.Println("MulticastSocket created")
	return MulticastSocket{Conn: *conn}, nil
}

func (m MulticastSocket) ReadFromMulticastSocket(evChan chan domain.Event) {
	msgBuffer := make([]byte, 1024)

	for {
		n, addr, err := m.Conn.ReadFrom(msgBuffer)
		if err != nil {
			fmt.Println("MulticastSocket ReadFromMulticastSocket err:", err)
		}

		message := &pb.GameMessage{}
		err = proto.Unmarshal(msgBuffer[:n], message)
		if err != nil {
			fmt.Println("!", err)
			continue
		}
		fmt.Println("send new message to event chan")
		evChan <- domain.Event{
			From:        addr.String(),
			GameMessage: message,
			Transport:   domain.Multicast,
		}
	}
}
