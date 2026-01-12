package application

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/oxaxxaxaxaxaxaxxaaaxax/snake/internal/domain"
	pb "github.com/oxaxxaxaxaxaxaxxaaaxax/snake/proto"
)

var (
	ErrAvailableIdNotFound = errors.New("id not found")
	ErrAddressNotFound     = errors.New("address not found")
)

type Master struct {
	PlayerCount int32
	Players     []domain.Player

	Engine *Engine

	//пока просто данные дя тестинга сети
	GameConfig *pb.GameConfig

	lastSend []time.Time
	lastRecv []time.Time
}

func NewMaster(e *Engine) *Master {
	var addr string
	if e.USock.Conn == nil {
		addr = "0.0.0.0"
	} else {
		addr = e.USock.Conn.LocalAddr().String()
	}
	players := make([]domain.Player, 0)
	players = append(players, domain.Player{
		PlayerName: "kriper2004",
		PlayerId:   0,
		PlayerAddr: addr,
		Role:       pb.NodeRole_MASTER,
		PlayerType: pb.PlayerType_HUMAN,
		Score:      0,
	})
	return &Master{Engine: e, Players: players, PlayerCount: 1}
}

func (m *Master) SetEngine(e *Engine) {
	m.Engine = e
	fmt.Println("addr before")
	fmt.Println(m.Players[0].PlayerAddr)
	m.Players[0].PlayerAddr = e.USock.Conn.LocalAddr().String()
	fmt.Println("addr after")
	fmt.Println(m.Players[0].PlayerAddr)

	//по сути это 0
	//m.Engine.GameCtx.PlayerID = m.Players[0].PlayerId
}

func (m *Master) SetId(id int32) {
	m.Players[0].PlayerId = id
}

func (m *Master) AddPlayer(address string, id int32) []domain.Player {
	m.Players = append(m.Players, domain.Player{
		PlayerId:   id,
		PlayerAddr: address,
	})
	return m.Players
}

func (m *Master) SendJoinAck(receiveMsg *pb.GameMessage, address string) {
	fmt.Println("role", receiveMsg.GetJoin().RequestedRole)
	m.Engine.USock.SendAck(receiveMsg, address, m.Engine.GameCtx.PlayerID, m.PlayerCount)
	m.Players = m.AddPlayer(address, m.PlayerCount)
	m.PlayerCount++
}

func (m *Master) SendPing() {
	for _, player := range m.Players {
		m.Engine.USock.SendPing(player.PlayerAddr, m.Engine.GameCtx.PlayerID)
	}
}

func (m *Master) SetGameName() {
	var name string
	fmt.Println("Enter Game Name")
	fmt.Scanf("%s", &name)
	m.Engine.GameCtx.GameName = name
}

func (m *Master) SetGameContext() {
	m.Engine.GameCtx = &domain.GameCtx{}
	m.Engine.GameCtx.PlayerID = m.Players[0].PlayerId
	m.Engine.GameCtx.PlayerName = "master_name"
	m.Engine.GameCtx.PlayerType = pb.PlayerType_HUMAN
	m.Engine.GameCtx.Role = pb.NodeRole_MASTER
}

func (m *Master) SetGameConfig() {
	var width int32 = 40
	var height int32 = 30
	var foodStatic int32 = 1
	var stateDelayMs int32 = 1000

	m.GameConfig = &pb.GameConfig{
		Width:        &width,
		Height:       &height,
		FoodStatic:   &foodStatic,
		StateDelayMs: &stateDelayMs,
	}
	fmt.Println("Game Config", m.GameConfig)

}

func SplitAddress(address string) (string, int32) {
	fmt.Println("Enter Address: ", address)
	sl := strings.Split(address, ":")
	ip := sl[0]
	port, err := strconv.Atoi(sl[1])
	if err != nil {
		fmt.Println(err)
	}
	return ip, int32(port)
}

func PlayerToProto(player domain.Player) *pb.GamePlayer {
	//ip, port := SplitAddress(player.PlayerAddr)
	return &pb.GamePlayer{
		Name:  &player.PlayerName,
		Id:    &player.PlayerId,
		Role:  &player.Role,
		Type:  &player.PlayerType,
		Score: &player.Score,
	}
}

func PlayersToProto(players []domain.Player) *pb.GamePlayers {
	protoPlayers := &pb.GamePlayers{}
	for _, player := range players {
		protoPlayers.Players = append(protoPlayers.Players, PlayerToProto(player))
	}
	return protoPlayers
}

func (m *Master) SendAnnouncement(address string) {
	ctx := m.Engine.GameCtx

	canJoin := true
	//fmt.Println("game config in m send", m.GameConfig)
	announce := []*pb.GameAnnouncement{{
		GameName: &ctx.GameName,
		//заполнить данными на будущее!
		Config:  m.GameConfig,
		CanJoin: &canJoin,
		Players: PlayersToProto(m.Players),
	}}
	//fmt.Println("Send Announcement To Multicast from master")
	err := m.Engine.USock.SendAnnouncement(address, ctx.PlayerID, announce)
	if err != nil {
		fmt.Println("Err in SendAnnouncementToMulticast", err)
	}
}

func (m *Master) SendAnnouncementToMulticast() {
	ctx := m.Engine.GameCtx

	canJoin := true
	//fmt.Println("game config in m send", m.GameConfig)
	announce := []*pb.GameAnnouncement{{
		GameName: &ctx.GameName,
		//заполнить данными на будущее!
		Config:  m.GameConfig,
		CanJoin: &canJoin,
		Players: PlayersToProto(m.Players),
	}}
	//fmt.Println("Send Announcement To Multicast from master")
	err := m.Engine.USock.SendAnnouncementToMulticast(ctx.PlayerID, announce)
	if err != nil {
		fmt.Println("Err in SendAnnouncementToMulticast", err)
	}
}

func (m *Master) GetPlayerAddrById(id int32) (string, error) {
	for _, player := range m.Players {
		if player.PlayerId == id {
			return player.PlayerAddr, nil
		}
	}
	return "", ErrAddressNotFound
}

func (m *Master) ChangeDeputyInfo() domain.GameCtx {
	ctx := m.Engine.GameCtx

	//newDeputyId := m.Rand.GetRandomId(m.PlayerCount)
	newDeputyId, err := m.GetAvailableId(m.PlayerCount)
	if err != nil {
		fmt.Println(err)
		return domain.GameCtx{}
	}
	ctx.DeputyInfo.DeputyId = newDeputyId

	newDeputyAddr, err := m.GetPlayerAddrById(newDeputyId)
	if err != nil {
		fmt.Println(err)
		return domain.GameCtx{}
	}
	ctx.DeputyInfo.DeputyAddr = newDeputyAddr
	return *ctx
}

func (m *Master) GetAvailableId(playerCount int32) (int32, error) {
	for id := range playerCount {
		if m.Players[id].Role == pb.NodeRole_NORMAL {
			return id, nil
		}
	}
	return -1, ErrAvailableIdNotFound
}

func (m *Master) SelectNewDeputy() {
	*m.Engine.GameCtx = m.ChangeDeputyInfo()
	ctx := m.Engine.GameCtx
	m.Engine.USock.SendRoleChange(ctx.DeputyInfo.DeputyAddr, ctx.PlayerID, pb.NodeRole_DEPUTY, pb.NodeRole_MASTER)
}

func (m *Master) DeputyToMaster() {
	*m.Engine.GameCtx = m.ChangeDeputyInfo()
	ctx := m.Engine.GameCtx
	for _, player := range m.Players {
		if player.PlayerId == ctx.DeputyInfo.DeputyId {
			m.Engine.USock.SendRoleChange(ctx.DeputyInfo.DeputyAddr, ctx.PlayerID, pb.NodeRole_DEPUTY, pb.NodeRole_MASTER)
		} else {
			m.Engine.USock.SendRoleChange(player.PlayerAddr, m.Engine.GameCtx.PlayerID, pb.NodeRole_NORMAL, pb.NodeRole_MASTER)
		}
	}
}

func (m *Master) NotifyDeadPlayer(id int32) {
	ctx := m.Engine.GameCtx
	player := m.Players[id]
	m.Engine.USock.SendRoleChange(player.PlayerAddr, ctx.PlayerID, pb.NodeRole_VIEWER, pb.NodeRole_MASTER)
}

func (m *Master) LeaveMaster() {
	ctx := m.Engine.GameCtx
	viewer := NewViewer(m.Engine)
	viewer.MasterToViewer(ctx.DeputyInfo.DeputyAddr)
}

func (m *Master) HandleRoleChange(senderRole, receiverRole pb.NodeRole) RolePlayer {
	//пока что я вижу тут только один случай - от осознанно выхдящего игрока
	switch senderRole {
	case pb.NodeRole_VIEWER:
		m.CreateZombieSnake()
	}
	return m
}

func (m *Master) CreateZombieSnake() {
	fmt.Println("Create Zombie Snake")
}

func (m *Master) PlaceSnake() bool {
	fmt.Println("Place snake!")
	return true
}

func (m *Master) Start() {
	fmt.Println("master Start game")
	m.SetGameName()
	m.SetGameConfig()
	fmt.Println("Game:", m.Engine.GameCtx.GameName)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.SendAnnouncementToMulticast()
		}
	}
}
