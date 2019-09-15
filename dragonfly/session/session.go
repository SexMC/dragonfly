package session

import (
	"fmt"
	"github.com/dragonfly-tech/dragonfly/dragonfly/player/chat"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/cmd"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/text"
	"github.com/sirupsen/logrus"
	"net"
	"sync/atomic"
)

// Session handles incoming packets from connections and sends outgoing packets by providing a thin layer
// of abstraction over direct packets. A Session basically 'controls' an entity.
type Session struct {
	log *logrus.Logger

	c                  Controllable
	controllableClosed atomic.Value
	conn               *minecraft.Conn

	cmdOrigin protocol.CommandOrigin
}

// New returns a new session using a controllable entity. The session will control this entity using the
// packets that it receives.
// New takes the connection from which to accept packets. It will start handling these packets after a call to
// Session.Handle().
func New(c Controllable, conn *minecraft.Conn, log *logrus.Logger) *Session {
	s := &Session{c: c, conn: conn, log: log}
	s.controllableClosed.Store(false)

	yellow := text.Yellow()
	chat.Global.Println(yellow(s.conn.IdentityData().DisplayName, "has joined the game"))

	return s
}

// Handle makes the session start handling incoming packets from the client.
func (s *Session) Handle() {
	go s.handlePackets()
	s.SendAvailableCommands()
}

// Close closes the session, which in turn closes the controllable and the connection that the session
// manages.
func (s *Session) Close() error {
	_ = s.c.Close()
	_ = s.conn.Close()

	yellow := text.Yellow()
	chat.Global.Println(yellow(s.conn.IdentityData().DisplayName, "has left the game"))
	return nil
}

// handlePackets continuously handles incoming packets from the connection. It processes them accordingly.
// Once the connection is closed, handlePackets will return.
func (s *Session) handlePackets() {
	defer func() {
		_ = s.Close()
	}()
	for {
		pk, err := s.conn.ReadPacket()
		if err != nil {
			return
		}
		if s.controllableClosed.Load().(bool) {
			// The controllable closed itself, so we need to stop handling packets and close the session.
			return
		}
		if err := s.handlePacket(pk); err != nil {
			// An error occurred during the handling of a packet. Print the error and stop handling any more
			// packets.
			s.log.Errorf("error processing packet from %v: %v\n", s.conn.RemoteAddr(), err)
			return
		}
	}
}

// handlePacket handles an incoming packet, processing it accordingly. If the packet had invalid data or was
// otherwise not valid in its context, an error is returned.
func (s *Session) handlePacket(pk packet.Packet) error {
	switch pk := pk.(type) {
	case *packet.Text:
		return s.handleText(pk)
	case *packet.CommandRequest:
		return s.handleCommandRequest(pk)
	default:
		s.log.Debugf("unhandled packet %T%v from %v\n", pk, fmt.Sprintf("%+v", pk)[1:], s.conn.RemoteAddr())
	}
	return nil
}

// handleText ...
func (s *Session) handleText(pk *packet.Text) error {
	if pk.TextType != packet.TextTypeChat {
		return fmt.Errorf("text packet can only contain text type of type chat (%v) but got %v", packet.TextTypeChat, pk.TextType)
	}
	if pk.SourceName != s.conn.IdentityData().DisplayName {
		return fmt.Errorf("text packet source name must be equal to display name")
	}
	s.c.Chat(pk.Message)
	return nil
}

// handleCommandRequest ...
func (s *Session) handleCommandRequest(pk *packet.CommandRequest) error {
	if pk.Internal {
		return fmt.Errorf("command request packet must never have the internal field set to true")
	}
	s.cmdOrigin = pk.CommandOrigin
	s.c.ExecuteCommand(pk.CommandLine)
	return nil
}

// SendMessage ...
func (s *Session) SendMessage(message string) {
	_ = s.conn.WritePacket(&packet.Text{
		TextType: packet.TextTypeRaw,
		Message:  message,
	})
}

// SendTip ...
func (s *Session) SendTip(message string) {
	_ = s.conn.WritePacket(&packet.Text{
		TextType: packet.TextTypePopup,
		Message:  message,
	})
}

// SendAnnouncement ...
func (s *Session) SendAnnouncement(message string) {
	_ = s.conn.WritePacket(&packet.Text{
		TextType: packet.TextTypeAnnouncement,
		Message:  message,
	})
}

// SendPopup ...
func (s *Session) SendPopup(message string) {
	_ = s.conn.WritePacket(&packet.Text{
		TextType: packet.TextTypePopup,
		Message:  message,
	})
}

// SendJukeBoxPopup ...
func (s *Session) SendJukeBoxPopup(message string) {
	_ = s.conn.WritePacket(&packet.Text{
		TextType: packet.TextTypeJukeboxPopup,
		Message:  message,
	})
}

// SendTitle ...
func (s *Session) SendTitle(text string, fadeInDuration int32, remainDuration int32, fadeOutDuration int32){
	_ = s.conn.WritePacket(&packet.SetTitle{
		ActionType:      packet.TitleActionSetTitle,
		Text:            text,
		FadeInDuration:  fadeInDuration,
		RemainDuration:  remainDuration,
		FadeOutDuration: fadeOutDuration,
	})
}

// SendSubTitle ...
func (s *Session) SendSubTitle(text string, fadeInDuration int32, remainDuration int32, fadeOutDuration int32) {
	_ = s.conn.WritePacket(&packet.SetTitle{
		ActionType:      packet.TitleActionSetSubtitle,
		Text:            text,
		FadeInDuration:  fadeInDuration,
		RemainDuration:  remainDuration,
		FadeOutDuration: fadeOutDuration,
	})
}

// SendActionbarMessage ...
func (s *Session) SendActionBarMessage(text string, fadeInDuration int32, remainDuration int32, fadeOutDuration int32) {
	_ = s.conn.WritePacket(&packet.SetTitle{
		ActionType:      packet.TitleActionSetActionBar,
		Text:            text,
		FadeInDuration:  fadeInDuration,
		RemainDuration:  remainDuration,
		FadeOutDuration: fadeOutDuration,
	})
}

// SendNetherDimension sends the player to the nether dimension
func (s *Session) SendNetherDimension(){
	_ = s.conn.WritePacket(&packet.ChangeDimension{
		Dimension: packet.DimensionNether,
		Position:  mgl32.Vec3{},
		Respawn:   false,
	})
}

// SendEndDimension sends the player to the end dimension
func (s *Session) SendEndDimension(){
	_ = s.conn.WritePacket(&packet.ChangeDimension{
		Dimension: packet.DimensionEnd,
		Position:  mgl32.Vec3{},
		Respawn:   false,
	})
}

// SendNetherDimension sends the player to the overworld dimension
func (s *Session) SendOverworldDimension(){
	_ = s.conn.WritePacket(&packet.ChangeDimension{
		Dimension: packet.DimensionOverworld,
		Position:  mgl32.Vec3{},
		Respawn:   false,
	})
}

// Disconnect disconnects the client and ultimately closes the session. If the message passed is non-empty,
// it will be shown to the client.
func (s *Session) Disconnect(message string) {
	_ = s.conn.WritePacket(&packet.Disconnect{
		HideDisconnectionScreen: message == "",
		Message:                 message,
	})
	s.controllableClosed.Store(true)
}

// Transfer transfers the player to a server with the IP and port passed.
func (s *Session) Transfer(ip net.IP, port int) {
	_ = s.conn.WritePacket(&packet.Transfer{
		Address: ip.String(),
		Port:    uint16(port),
	})
}

// SendCommandOutput sends the output of a command to the player. It will be shown to the caller of the
// command, which might be the player or a websocket server.
func (s *Session) SendCommandOutput(output *cmd.Output) {
	messages := make([]protocol.CommandOutputMessage, 0, output.MessageCount()+output.ErrorCount())
	for _, message := range output.Messages() {
		messages = append(messages, protocol.CommandOutputMessage{
			Success: true,
			Message: message,
		})
	}
	for _, err := range output.Errors() {
		messages = append(messages, protocol.CommandOutputMessage{
			Success: false,
			Message: err.Error(),
		})
	}

	_ = s.conn.WritePacket(&packet.CommandOutput{
		CommandOrigin:  s.cmdOrigin,
		OutputType:     3,
		SuccessCount:   uint32(output.MessageCount()),
		OutputMessages: messages,
	})
}

// SendAvailableCommands sends all available commands of the server. Once sent, they will be visible in the
// /help list and will be auto-completed.
func (s *Session) SendAvailableCommands() {
	commands := cmd.Commands()
	pk := &packet.AvailableCommands{}
	for alias, c := range commands {
		if c.Name() != alias {
			// Don't add duplicate entries for aliases.
			continue
		}
		params := c.Params()
		overloads := make([]protocol.CommandOverload, len(params))
		for i, params := range params {
			for _, paramInfo := range params {
				t, enum := valueToParamType(paramInfo.Value)
				t |= protocol.CommandArgValid
				overloads[i].Parameters = append(overloads[i].Parameters, protocol.CommandParameter{
					Name:                paramInfo.Name,
					Type:                t,
					Optional:            paramInfo.Optional,
					CollapseEnumOptions: paramInfo.Value == false || paramInfo.Value == true,
					Enum:                enum,
					Suffix:              paramInfo.Suffix,
				})
			}
		}
		pk.Commands = append(pk.Commands, protocol.Command{
			Name:        c.Name(),
			Description: c.Description(),
			Aliases:     c.Aliases(),
			Overloads:   overloads,
		})
	}
	_ = s.conn.WritePacket(pk)
}

// valueToParamType finds the command argument type of a value passed and returns it, in addition to creating
// an enum if applicable.
func valueToParamType(i interface{}) (t uint32, enum protocol.CommandEnum) {
	switch i.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return protocol.CommandArgTypeInt, enum
	case float32, float64:
		return protocol.CommandArgTypeFloat, enum
	case string:
		return protocol.CommandArgTypeString, enum
	case bool:
		return 0, protocol.CommandEnum{
			Type:    "bool",
			Options: []string{"true", "1", "false", "0"},
		}
	case mgl32.Vec3:
		return protocol.CommandArgTypePosition, enum
	}
	if param, ok := i.(cmd.Parameter); ok && param.Type() == "player" || param.Type() == "target" {
		return protocol.CommandArgTypeTarget, enum
	}
	if enum, ok := i.(cmd.Enum); ok {
		return 0, protocol.CommandEnum{
			Type:    enum.Type(),
			Options: enum.Options(),
		}
	}
	return protocol.CommandArgTypeValue, enum
}
