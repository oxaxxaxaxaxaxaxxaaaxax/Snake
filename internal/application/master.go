package application

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oxaxxaxaxaxaxaxxaaaxax/snake/internal/application/transport"
	"github.com/oxaxxaxaxaxaxaxxaaaxax/snake/internal/domain"
	pb "github.com/oxaxxaxaxaxaxaxxaaaxax/snake/proto"
)

var (
	ErrAvailableIdNotFound = errors.New("id not found")
	ErrAddressNotFound     = errors.New("address not found")
	ErrPlayerNotFound      = errors.New("player not found")
)

type Master struct {
	PlayerCount atomic.Int32

	Players   []domain.Player
	PlayerMtx sync.Mutex

	Engine *Engine

	//пока просто данные дя тестинга сети
	GameConfig *pb.GameConfig

	//lastSend []time.Time
	//lastRecv []time.Time
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
	m := &Master{Engine: e, Players: players}
	m.PlayerCount.Store(1)
	return m
}

func (m *Master) SetEngine(e *Engine) {
	m.Engine = e
	//fmt.Println("addr before")
	//fmt.Println(m.Players[0].PlayerAddr)
	m.PlayerMtx.Lock()
	m.Players[0].PlayerAddr = e.USock.Conn.LocalAddr().String()
	m.PlayerMtx.Unlock()
	//fmt.Println("addr after")
	//fmt.Println(m.Players[0].PlayerAddr)

	//по сути это 0
	//m.Engine.GameCtx.PlayerID = m.Players[0].PlayerId
}

func (m *Master) SetId(id int32) {
	m.PlayerMtx.Lock()
	m.Players[0].PlayerId = id
	m.PlayerMtx.Unlock()
}

func (m *Master) AddPlayer(address string, id int32, playerRole pb.NodeRole) {
	fmt.Println("add player, dep id:", m.Engine.GameCtx.DeputyInfo.DeputyId)

	m.PlayerMtx.Lock()
	m.Players = append(m.Players, domain.Player{
		PlayerId:   id,
		PlayerAddr: address,
		Role:       playerRole,
	})
	m.PlayerMtx.Unlock()

}

func (m *Master) DeletePlayer(playerAddr string) error {
	m.PlayerMtx.Lock()
	players := m.Players
	for idx, player := range players {
		if player.PlayerAddr == playerAddr {
			m.Players[idx] = m.Players[len(m.Players)-1]
			m.Players = m.Players[:len(m.Players)-1]
			m.PlayerCount.Add(-1)
			m.PlayerMtx.Unlock()
			return nil
		}
	}
	m.PlayerMtx.Unlock()
	return ErrPlayerNotFound
}

func (m *Master) Contains(playerAddr string) bool {
	m.PlayerMtx.Lock()
	players := m.Players
	for _, player := range players {
		if player.PlayerAddr == playerAddr {
			m.PlayerMtx.Unlock()
			return true
		}
	}
	m.PlayerMtx.Unlock()
	return false
}

func (m *Master) SendJoinAck(receiveMsg *pb.GameMessage, address string, peer *transport.PeerInfo, peerMtx *sync.Mutex) error {
	fmt.Println("role", receiveMsg.GetJoin().RequestedRole)
	m.Engine.GameMtx.Lock()
	playerId := m.Engine.GameCtx.PlayerID
	m.Engine.GameMtx.Unlock()

	playerRole := *receiveMsg.GetJoin().RequestedRole

	playerCount := m.PlayerCount.Add(1) - 1
	err := m.Engine.USock.SendAck(receiveMsg, address, playerId, playerCount, peer, peerMtx)
	if err != nil {
		return err
	}
	m.AddPlayer(address, playerCount, playerRole)
	//m.PlayerCount++
	//m.PlayerCount.Add(1)

	m.Engine.GameMtx.Lock()
	if m.Engine.GameCtx.DeputyInfo.DeputyId != UninitializedId {
		m.Engine.GameMtx.Unlock()
		return nil
	}
	fmt.Println("create new deputy!", m.Engine.GameCtx.DeputyInfo)
	m.SelectNewDeputyLocked()
	fmt.Println("add new deputy!", m.Engine.GameCtx.DeputyInfo)
	deputyAddr := m.Engine.GameCtx.DeputyInfo.DeputyAddr
	m.Engine.GameMtx.Unlock()

	m.Engine.PeerMtx.Lock()
	peerInfo := m.Engine.Peers.PeersInfo[deputyAddr]
	m.Engine.PeerMtx.Unlock()
	err = m.Engine.USock.SendRoleChange(deputyAddr, playerId, pb.NodeRole_DEPUTY, pb.NodeRole_MASTER,
		peerInfo, &m.Engine.PeerMtx)
	if err != nil {
		fmt.Println("Err in SendRoleChange", err)
	}

	//m.Engine.PeerMtx.Lock()
	//m.Engine.Peers.PeersInfo[address] = peer
	//m.Engine.PeerMtx.Unlock()
	return nil
}

func (m *Master) SendPing(player domain.Player) {
	//for _, player := range m.Players {
	m.Engine.GameMtx.Lock()
	playerId := m.Engine.GameCtx.PlayerID
	m.Engine.GameMtx.Unlock()

	m.Engine.PeerMtx.Lock()
	peerInfo := m.Engine.Peers.PeersInfo[player.PlayerAddr]
	m.Engine.PeerMtx.Unlock()

	err := m.Engine.USock.SendPing(player.PlayerAddr, playerId, peerInfo, &m.Engine.PeerMtx)
	if err != nil {
		fmt.Println("in master send ping error", err)
	}
	//}
}

func (m *Master) SetGameName() {
	var name string
	fmt.Println("Enter Game Name")
	fmt.Scanf("%s", &name)
	m.Engine.GameMtx.Lock()
	m.Engine.GameCtx.GameName = name
	m.Engine.GameMtx.Unlock()
}

func (m *Master) SetGameContext() {
	m.Engine.GameMtx.Lock()
	m.Engine.GameCtx = &domain.GameCtx{}
	m.Engine.GameCtx.DeputyInfo.DeputyId = UninitializedId
	fmt.Println(m.Engine.GameCtx.DeputyInfo)
	m.PlayerMtx.Lock()
	m.Engine.GameCtx.PlayerID = m.Players[0].PlayerId
	m.PlayerMtx.Unlock()
	m.Engine.GameCtx.PlayerName = "master_name"
	m.Engine.GameCtx.PlayerType = pb.PlayerType_HUMAN
	m.Engine.GameCtx.Role = pb.NodeRole_MASTER
	m.Engine.GameCtx.StateMsDelay = DefaultStateMsDelay
	m.Engine.GameMtx.Unlock()
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
	m.Engine.GameMtx.Lock()
	gameName := m.Engine.GameCtx.GameName
	playerId := m.Engine.GameCtx.PlayerID
	m.Engine.GameMtx.Unlock()

	canJoin := true
	//fmt.Println("game config in m send", m.GameConfig)
	announce := []*pb.GameAnnouncement{{
		GameName: &gameName,
		//заполнить данными на будущее!
		Config:  m.GameConfig,
		CanJoin: &canJoin,
		Players: PlayersToProto(m.Players),
	}}
	//fmt.Println("Send Announcement To Unicast from master")
	err := m.Engine.USock.SendAnnouncement(address, playerId, announce)
	if err != nil {
		fmt.Println("Err in SendAnnouncementToUnicast", err)
	}
}

func (m *Master) SendAnnouncementToMulticast() {
	m.Engine.GameMtx.Lock()
	gameName := m.Engine.GameCtx.GameName
	playerId := m.Engine.GameCtx.PlayerID
	m.Engine.GameMtx.Unlock()

	canJoin := true
	//fmt.Println("game config in m send", m.GameConfig)
	announce := []*pb.GameAnnouncement{{
		GameName: &gameName,
		//заполнить данными на будущее!
		Config:  m.GameConfig,
		CanJoin: &canJoin,
		Players: PlayersToProto(m.Players),
	}}
	//fmt.Println("Send Announcement To Multicast from master")
	err := m.Engine.USock.SendAnnouncementToMulticast(playerId, announce)
	if err != nil {
		fmt.Println("Err in SendAnnouncementToMulticast", err)
	}
}

func (m *Master) GetPlayerAddrById(id int32) (string, error) {
	m.PlayerMtx.Lock()
	players := m.Players
	for _, player := range players {
		if player.PlayerId == id {
			m.PlayerMtx.Unlock()
			return player.PlayerAddr, nil
		}
	}
	m.PlayerMtx.Unlock()
	return "", ErrAddressNotFound
}

func (m *Master) ChangeDeputyInfoLocked() error {
	newDeputyId, err := m.GetAvailableId(&m.PlayerCount)
	if err != nil {
		fmt.Println(err)
		return err
	}

	m.Engine.GameCtx.DeputyInfo.DeputyId = newDeputyId

	newDeputyAddr, err := m.GetPlayerAddrById(newDeputyId)
	if err != nil {
		fmt.Println(err)
		return err
	}

	m.Engine.GameCtx.DeputyInfo.DeputyAddr = newDeputyAddr
	return nil
}

func (m *Master) GetAvailableId(playerCount *atomic.Int32) (int32, error) {
	m.PlayerMtx.Lock()
	plCount := playerCount.Load()
	defer m.PlayerMtx.Unlock()
	for id := range plCount {
		if m.Players[id].Role == pb.NodeRole_NORMAL {
			fmt.Println("Found Normal Player", m.Players[id].PlayerId)
			return m.Players[id].PlayerId, nil
		}
	}
	return -1, ErrAvailableIdNotFound
}

func (m *Master) SelectNewDeputyLocked() {
	var err error
	err = m.ChangeDeputyInfoLocked()
	if err != nil {
		fmt.Println("No available players for deputy(just 1 master)")
		return
	}

	return
	//отпустить ту лочку

	//m.Engine.PeerMtx.Lock()
	//peerInfo := m.Engine.Peers.PeersInfo[deputyAddr]
	//m.Engine.PeerMtx.Unlock()
	//err = m.Engine.USock.SendRoleChange(deputyAddr, playerId, pb.NodeRole_DEPUTY, pb.NodeRole_MASTER,
	//	peerInfo, &m.Engine.PeerMtx)
	//if err != nil {
	//	fmt.Println("Err in SendRoleChange", err)
	//}
}

func (m *Master) SetDeputyInfo(newDeputyAddr string, newDeputyId int32) error {
	m.Engine.GameMtx.Lock()
	defer m.Engine.GameMtx.Unlock()

	m.Engine.GameCtx.DeputyInfo.DeputyId = newDeputyId
	m.Engine.GameCtx.DeputyInfo.DeputyAddr = newDeputyAddr
	return nil
}

func (m *Master) DeputyToMaster() RolePlayer {
	var err error

	fmt.Println("DEPUTY TO MASTER")
	m.Engine.GameMtx.Lock()
	err = m.ChangeDeputyInfoLocked()
	m.Engine.GameMtx.Unlock()

	if err != nil {
		fmt.Println("No available players for deputy(just 1 master)")
		m.Engine.PeerMtx.Lock()
		peerInfo := m.Engine.Peers.PeersInfo[m.Engine.GameCtx.MasterInfo.MasterAddr]
		m.Engine.PeerMtx.Unlock()
		peerInfo.M.Lock()
		peerInfo.Dead = true
		peerInfo.M.Unlock()
		return m

	}
	//Я не знаю насколько это нормально
	//думаю нормально если при получении сообщения нормалом что он новый депутя он проставит инфу о депути
	//upd инфу о мастере депутя новый не положит
	//юерем из сокета

	m.Engine.GameMtx.Lock()
	ctx := m.Engine.GameCtx
	//!!!! возсожно тут полетит хз
	ctx.MasterInfo.MasterId = m.Engine.GameCtx.PlayerID
	ctx.MasterInfo.MasterAddr = m.Engine.USock.Conn.LocalAddr().String()
	senderId := ctx.PlayerID
	deputyAddr := ctx.DeputyInfo.DeputyAddr
	deputyId := ctx.DeputyInfo.DeputyId
	m.Engine.GameMtx.Unlock()

	type target struct {
		addr string
		id   int32
	}

	playerData := make([]target, 0)

	m.PlayerMtx.Lock()
	for _, player := range m.Players {
		playerData = append(playerData, target{
			addr: player.PlayerAddr,
			id:   player.PlayerId,
		})
	}
	m.PlayerMtx.Unlock()

	for _, plData := range playerData {
		m.Engine.PeerMtx.Lock()
		peerInfo := m.Engine.Peers.PeersInfo[plData.addr]
		m.Engine.PeerMtx.Unlock()

		if peerInfo == nil {
			continue
		}

		if plData.id == deputyId {
			err = m.Engine.USock.SendRoleChange(deputyAddr, senderId, pb.NodeRole_DEPUTY,
				pb.NodeRole_MASTER, peerInfo, &m.Engine.PeerMtx)
			if err != nil {
				fmt.Println("Err in SendRoleChange", err)
			}
		} else {
			err = m.Engine.USock.SendRoleChange(plData.addr, senderId, pb.NodeRole_NORMAL,
				pb.NodeRole_MASTER, peerInfo, &m.Engine.PeerMtx)
			if err != nil {
				fmt.Println("Err in SendRoleChange", err)
			}
		}
	}
	return m
}

func (m *Master) NotifyDeadPlayer(id int32, peerInfo *transport.PeerInfo) {
	m.Engine.GameMtx.Lock()
	senderId := m.Engine.GameCtx.PlayerID
	m.Engine.GameMtx.Unlock()

	playerAddr, err := m.GetPlayerAddrById(id)
	if err != nil {
		fmt.Println(err)
	}
	err = m.Engine.USock.SendRoleChange(playerAddr, senderId, pb.NodeRole_VIEWER, pb.NodeRole_MASTER,
		peerInfo, &m.Engine.PeerMtx)
	if err != nil {
		fmt.Println("Err in NotifyDeadPlayer", err)
	}
}

func (m *Master) LeaveMaster(peerInfo *transport.PeerInfo) {
	m.Engine.GameMtx.Lock()
	deputyAddr := m.Engine.GameCtx.DeputyInfo.DeputyAddr
	m.Engine.GameMtx.Unlock()

	viewer := NewViewer(m.Engine)
	viewer.MasterToViewer(deputyAddr, peerInfo)
}

func (m *Master) HandleRoleChange(senderRole, receiverRole pb.NodeRole, peerInfo *transport.PeerInfo) RolePlayer {
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

func (m *Master) StartNetTicker() {
	interval := m.Engine.GameCtx.StateMsDelay / 10
	recvInterval := interval * 8

	ticker := time.NewTicker(time.Duration(interval) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.CheckTimeoutInteraction(interval, recvInterval)
		}
	}
}

func (m *Master) CheckTimeoutInteraction(interval, recvInterval int32) RolePlayer {
	//fmt.Println("Check Timeout Interaction")

	m.PlayerMtx.Lock()
	players := make([]domain.Player, len(m.Players))
	copy(players, m.Players)
	m.PlayerMtx.Unlock()

	m.Engine.GameMtx.Lock()
	masterAddr := m.Engine.GameCtx.MasterInfo.MasterAddr
	m.Engine.GameMtx.Unlock()

	now := time.Now()

	for _, player := range players {
		//ДОБАВИТЬ ИНИЦИАЛИЗАЦИЮ МАСТЕР ИНФОР У МАСТЕРА(или не добавлять)
		if player.PlayerAddr == masterAddr {
			continue
		}
		m.Engine.PeerMtx.Lock()
		peerInfo := m.Engine.Peers.PeersInfo[player.PlayerAddr]
		m.Engine.PeerMtx.Unlock()

		if peerInfo == nil {
			fmt.Println("Peer is nil!!", player.PlayerAddr)
			continue
		}
		peerInfo.M.Lock()
		lastSend := peerInfo.LastSend
		peerInfo.M.Unlock()
		if lastSend.IsZero() {
			continue
		}
		if now.Sub(lastSend) >= time.Duration(interval)*time.Millisecond {
			fmt.Println("Player send timeout - SEND PING")
			m.SendPing(player)
		}
	}

	for _, player := range players {
		if player.PlayerAddr == masterAddr {
			continue
		}
		m.Engine.PeerMtx.Lock()
		peerInfo := m.Engine.Peers.PeersInfo[player.PlayerAddr]
		m.Engine.PeerMtx.Unlock()
		if peerInfo == nil {
			continue
		}
		peerInfo.M.Lock()
		//acks := peerInfo.Acknowledges.Acks
		acks := CopyPendingAcks(peerInfo.Acknowledges)
		peerInfo.M.Unlock()
		if acks == nil {
			continue
		}
		for _, ack := range acks.Acks {
			if ack == nil {
				continue
			}
			if ack.SendTime.IsZero() {
				continue
			}
			if now.Sub(ack.SendTime) >= time.Duration(interval)*time.Millisecond {
				fmt.Println("Player ack timeout - RETRY")
				//m.Engine.PeerMtx.Lock()
				//if m.Engine.Peers.PeersInfo[player.PlayerAddr] == nil {
				//	m.Engine.Peers.PeersInfo[player.PlayerAddr] = &transport.PeerInfo{
				//		Acknowledges: &transport.PendingAcks{Acks: make(map[int64]*transport.PendingAckMsg)},
				//		LastRecv:     time.Time{},
				//		LastSend:     time.Time{},
				//	}
				//}
				//m.Engine.PeerMtx.Unlock()
				err := m.Engine.USock.SendMessage(ack.RequestMsg, player.PlayerAddr,
					peerInfo, &m.Engine.PeerMtx)
				if err != nil {
					fmt.Println("Err in SendMessage master loop", err)
				}
			}
		}
	}

	var ToDelete []string
	for _, player := range players {
		if player.PlayerAddr == masterAddr {
			continue
		}
		m.Engine.PeerMtx.Lock()
		peerInfo := m.Engine.Peers.PeersInfo[player.PlayerAddr]
		m.Engine.PeerMtx.Unlock()
		if peerInfo == nil {
			continue
		}
		peerInfo.M.Lock()
		lastRecv := peerInfo.LastRecv
		peerInfo.M.Unlock()
		if lastRecv.IsZero() {
			continue
		}
		if now.Sub(lastRecv) >= time.Duration(recvInterval)*time.Millisecond {
			fmt.Println("Player recieve timeout - DISCONNECT", player.PlayerAddr)
			switch player.Role {
			case pb.NodeRole_DEPUTY:
				m.CreateZombieSnake()
			case pb.NodeRole_NORMAL:
				//m.NotifyDeadPlayer(player.PlayerId, m.Engine.Peers.PeersInfo[player.PlayerAddr])
				m.CreateZombieSnake()
			}
			ToDelete = append(ToDelete, player.PlayerAddr)

		}
	}
	for _, playerAddr := range ToDelete {
		err := m.DeletePlayer(playerAddr)
		if err != nil {
			fmt.Println("Delete player error", err)
		}
	}

	m.Engine.GameMtx.Lock()
	curDeputyAddr := m.Engine.GameCtx.DeputyInfo.DeputyAddr
	NeedNewDeputy := false
	for _, addr := range ToDelete {
		if addr == curDeputyAddr {
			NeedNewDeputy = true
			break
		}
	}
	m.Engine.GameMtx.Unlock()

	if NeedNewDeputy {
		m.Engine.GameMtx.Lock()
		m.SelectNewDeputyLocked()
		deputyAddr := m.Engine.GameCtx.DeputyInfo.DeputyAddr
		playerId := m.Engine.GameCtx.PlayerID
		m.Engine.GameMtx.Unlock()

		m.Engine.PeerMtx.Lock()
		peerInfo := m.Engine.Peers.PeersInfo[deputyAddr]
		m.Engine.PeerMtx.Unlock()

		if peerInfo == nil {
			fmt.Println("Peer is nil!!", deputyAddr)
			return m
		}

		err := m.Engine.USock.SendRoleChange(deputyAddr, playerId, pb.NodeRole_DEPUTY, pb.NodeRole_MASTER,
			peerInfo, &m.Engine.PeerMtx)
		if err != nil {
			fmt.Println("Err in SendRoleChange", err)
		}

	}

	return m
}

func CopyPendingAcks(src *transport.PendingAcks) *transport.PendingAcks {
	if src == nil {
		return nil
	}

	dst := &transport.PendingAcks{
		Acks: make(map[int64]*transport.PendingAckMsg, len(src.Acks)),
	}

	for seq, ack := range src.Acks {
		if ack == nil {
			dst.Acks[seq] = nil
			continue
		}

		ackCopy := *ack
		dst.Acks[seq] = &ackCopy
	}

	return dst
}
