package natneg

import (
	"encoding/binary"
	"net"
	"owfc/common"
	"owfc/logging"
	"time"
)

func (session *Session) sendConnectRequests(moduleName string) {
	if session.Connecting {
		return
	}
	if session.Host == nil || !session.Host.isMapped() {
		return
	}
	if session.Guest == nil || !session.Guest.isMapped() {
		return
	}

	logging.Notice(moduleName, "Exchange connect requests")

	session.Connecting = true
	session.Host.ConnectAck = false
	session.Guest.ConnectAck = false

	go func() {
		defer func() { session.Connecting = false }()
		for range time.NewTicker(time.Millisecond * 500).C {
			if !session.Open || session.Guest.Result != -1 || session.Host.Result != -1 {
				return
			}

			var check bool
			if !session.Host.ConnectAck {
				check = true
				session.Guest.sendConnectRequestPacket(natnegConn, session.Host, session.Version)
			}
			if !session.Guest.ConnectAck {
				check = true
				session.Host.sendConnectRequestPacket(natnegConn, session.Guest, session.Version)
			}
			if !check {
				return
			}
		}
	}()
}

func (client *Peer) sendConnectRequestPacket(conn net.PacketConn, destination *Peer, version byte) {
	connectHeader := createPacketHeader(version, NNConnectRequest, destination.Cookie)
	connectHeader = append(connectHeader, client.ServerAddr.IP...)
	connectHeader = binary.BigEndian.AppendUint16(connectHeader, uint16(client.ServerAddr.Port))
	// Two bytes: "gotyourdata" and "finished"
	connectHeader = append(connectHeader, 0x42, 0x00)

	conn.WriteTo(connectHeader, destination.NegotiateAddr)
}

func (session *Session) handleConnectReply(conn net.PacketConn, addr *net.UDPAddr, buffer []byte, moduleName string, version byte) {
	common.MaybeUnused(conn, addr, buffer, moduleName, version)

	// portType := buffer[0]
	clientIndex := buffer[1]
	// useGamePort := buffer[2]
	// localIP, ok := netip.AddrFromSlice(buffer[3:7])

	client := session.Guest
	if clientIndex == 1 {
		client = session.Host
	}
	if client == nil {
		return
	}

	client.ConnectAck = true
}
