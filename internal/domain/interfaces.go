package domain

import (
	pb "github.com/oxaxxaxaxaxaxaxxaaaxax/snake/proto"
)

type Transport int

const (
	Unicast Transport = iota
	Multicast
)

type Event struct {
	GameMessage *pb.GameMessage
	From        string
	Transport
}

type GameCtx struct {
	Role       pb.NodeRole
	PlayerType pb.PlayerType
	PlayerID   int32

	PlayerName string
	GameName   string
	MasterInfo MasterInfo
	DeputyInfo DeputyInfo
}
type MasterInfo struct {
	MasterId   int32
	MasterAddr string
}

type DeputyInfo struct {
	DeputyId   int32
	DeputyAddr string
}

type Player struct {
	PlayerName string
	PlayerId   int32
	PlayerAddr string
	Role       pb.NodeRole
	PlayerType pb.PlayerType
	Score      int32
}
