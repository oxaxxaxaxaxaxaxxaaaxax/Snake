package application

import (
	"errors"
	"time"

	pb "github.com/oxaxxaxaxaxaxaxxaaaxax/snake/proto"
)

var ErrMessageNotFound = errors.New("message not found")

type PendingAckMsg struct {
	RequestMsg *pb.GameMessage
	SendTime   time.Time
}

type PendingAcks struct {
	Acks []PendingAckMsg
}

func NewPendingAcks() *PendingAcks {
	return &PendingAcks{}
}

func (p *PendingAcks) GetPendingAckById(id int32) (PendingAckMsg, error) {
	for _, msg := range p.Acks {
		if *msg.RequestMsg.ReceiverId == id {
			return msg, nil
		}
	}
	return PendingAckMsg{}, ErrMessageNotFound
}

func (p *PendingAcks) AppendAcks(ack PendingAckMsg) {
	p.Acks = append(p.Acks, ack)
}

func (p *PendingAcks) RemoveAcks(ack PendingAckMsg) {
	for idx, ackMsg := range p.Acks {
		if ackMsg == ack {
			p.Acks[idx] = p.Acks[len(p.Acks)-1]
			p.Acks = p.Acks[:len(p.Acks)-1]
			break
		}
	}
}
