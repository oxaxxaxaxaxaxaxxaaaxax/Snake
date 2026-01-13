package transport

import (
	"errors"
	"fmt"
	"time"
)

var ErrMessageNotFound = errors.New("message not found")

type PendingAckMsg struct {
	//RequestMsg *pb.GameMessage
	RequestMsg []byte
	SendTime   time.Time
}

type PendingAcks struct {
	Acks map[int64]*PendingAckMsg
}

func NewPendingAcks() *PendingAcks {
	return &PendingAcks{Acks: make(map[int64]*PendingAckMsg)}
}

//func (p *PendingAcks) DeletePendingAckByIdAndSeq(id int32, seq int64) (bool, error) {
//	//for _, msg := range p.Acks {
//	//	//if *msg.RequestMsg.ReceiverId == id {
//	//	//	return msg, nil
//	//	//}
//	//
//	//	if
//	//}
//
//	if _, ok := p.Acks[id][seq]; ok {
//		delete(p.Acks[id], seq)
//		return true, nil
//	}
//	return false, ErrMessageNotFound
//}

func (p *PendingAcks) AppendAcks(ack PendingAckMsg, seq int64) {
	//p.Acks = append(p.Acks, ack)
	//p.Acks[id] = make(map[int64]*PendingAckMsg)
	fmt.Println("Append ", seq)
	p.Acks[seq] = &ack
}

func (p *PendingAcks) RemoveAcks(seq int64) (bool, error) {
	//for idx, ackMsg := range p.Acks {
	//	if ackMsg == ack {
	//		p.Acks[idx] = p.Acks[len(p.Acks)-1]
	//		p.Acks = p.Acks[:len(p.Acks)-1]
	//		break
	//	}
	//}
	fmt.Println("Remove ", seq)
	if _, ok := p.Acks[seq]; ok {
		delete(p.Acks, seq)
		return true, nil
	}
	return false, ErrMessageNotFound
}
