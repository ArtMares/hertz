package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"nhooyr.io/websocket"
	"os"
	"sync"
	"time"
)

// Program entry point
func main() {
	// Command line inputs
	var token = flag.String("token", "", "Discord bot authentication token")
	var bot Bot
	flag.Parse()
	err := bot.New(*token)
	if err != nil {
		panic(err)
	}
	return
}

const (

	// ULRs
	DiscordApiRoot               = "https://discordapp.com/api/v"
	DiscordApiVersion            = "6"
	DiscordApiBase               = DiscordApiRoot + DiscordApiVersion
	DiscordHttpUserAgent         = "DiscordBot (" + DiscordApiRoot + ", " + DiscordApiVersion + ")"
	DiscordUserInfoEndpoint      = "/users/@me"
	DiscordUserGuildsEndpoint    = "/users/@me/guilds"
	DiscordGuildChannelsEndpoint = "/guilds/%s/channels"
	DiscordGatewayBotEndpoint    = "/gateway/bot"
	DiscordGatewayEndpoint       = "wss://gateway.discord.gg/?v=%s&encoding=json"

	// Channel types
	ChannelTypeText     = 0x0
	ChannelTypeDM       = 0x1
	ChannelTypeVoice    = 0x2
	ChannelTypeGroupDM  = 0x3
	ChannelTypeCategory = 0x4
	ChannelTypeNews     = 0x5
	ChannelTypeStore    = 0x6

	// Gateway opcodes
	GwOpcDispatch            int8 = 0x0 // R
	GwOpcHeartbeat           int8 = 0x1 // s
	GwOpcIdentify            int8 = 0x2 // s
	GwOpcStatusUpdate        int8 = 0x3 // s
	GwOpcVoiceStateUpdate    int8 = 0x4 // s
	GwOpcResume              int8 = 0x6 // s
	GwOpcReconnect           int8 = 0x7 // R
	GwOpcRequestGuildMembers int8 = 0x8 // s
	GwOpcInvalidSession      int8 = 0x9 // R
	GwOpcHello               int8 = 0xa // R
	GwOpcHeartbeatACK        int8 = 0xb // R

	// Gateway Events
	GwEvHello                    = "HELLO"
	GwEvReady                    = "READY"
	GwEvResumed                  = "RESUMED"
	GwEvReconnect                = "RECONNECT"
	GwEvInvalidSession           = "INVALID_SESSION"
	GwEvChannelCreate            = "CHANNEL_CREATE"
	GwEvChannelUpdate            = "CHANNEL_UPDATE"
	GwEvChannelDelete            = "CHANNEL_DELETE"
	GwEvChannelPinsUpdate        = "CHANNEL_PINS_UPDATE"
	GwEvGuildCreate              = "GUILD_CREATE"
	GwEvGuildUpdate              = "GUILD_UPDATE"
	GwEvGuildDelete              = "GUILD_DELETE"
	GwEvGuildBanAdd              = "GUILD_BAN_ADD"
	GwEvGuildBanRemove           = "GUILD_BAN_REMOVE"
	GwEvGuildEmojisUpdate        = "GUILD_EMOJIS_UPDATE"
	GwEvGuildIntegrationsUpdate  = "GUILD_INTEGRATION_UPDATE"
	GwEvGuildMemberAdd           = "GUILD_MEMBER_ADD"
	GwEvGuildMemberRemove        = "GUILD_MEMBER_REMOVE"
	GwEvGuildMemberUpdate        = "GUILD_MEMBER_UPDATE"
	GwEvGuildMembersChunk        = "GUILD_MEMBERS_CHUNK"
	GwEvGuildRoleCreate          = "GUILD_ROLE_CREATE"
	GwEvGuildRoleUpdate          = "GUILD_ROLE_UPDATE"
	GwEvGuildRoleDelete          = "GUILD_ROLE_DELETE"
	GwEvMessageCreate            = "MESSAGE_CREATE"
	GwEvMessageUpdate            = "MESSAGE_UPDATE"
	GwEvMessageDelete            = "MESSAGE_DELETE"
	GwEvMessageDeleteBulk        = "MESSAGE_DELETE_BULK"
	GwEvMessageReactionAdd       = "MESSAGE_REACTION_ADD"
	GwEvMessageReactionRemove    = "MESSAGE_REACTION_REMOVE"
	GwEvMessageReactionRemoveAll = "MESSAGE_REACTION_REMOVE_ALL"
	GwEvPresenceUpdate           = "PRESENCE_UPDATE"
	GwEvTypingStart              = "TYPING_START"
	GwEvUserUpdate               = "USER_UPDATE"
	GwEvVoiceStateUpdate         = "VOICE_STATE_UPDATE"
	GwEvVoiceServerUpdate        = "VOICE_SERVER_UPDATE"
	GwEvWebhooksUpdate           = "WEBHOOKS_UPDATE"

	// HTTP headers
	HttpHeaderUserAgent     = "User-Agent"
	HttpHeaderAuthorization = "Authorization"

	// Errors
	ErrorInvalidAuthorizationToken = "invalid authentication token"
	ErrorGatewayConnection         = "gateway connection error"

	// Program error codes
	ErrorInvalidSession = 0x1
	ErrorNetwork        = 0x2
)

// Types
type Bot struct {
	Id             string
	Username       string
	Token          string
	Guilds         []Guild
	Conn           *websocket.Conn
	Context        context.Context
	Heartbeat      float64
	WG             sync.WaitGroup
	Ready          bool
	OnEvent        func(p Payload)
	EventChan      chan Payload
	VoiceStates    []VoiceState
	Voices         []Voice
	Voice          *websocket.Conn
	VoiceSession   string
	VoiceHeartbeat float64
	OnVoiceData    func(p Payload)
}

type Guild struct {
	Id          string
	Name        string
	Permissions int
	Channels    []Channel
	Unavailable bool
}

type Channel struct {
	Id      string
	Type    int
	Bitrate int
}

type Voice struct {
	GuildId   string
	ChannelId string
	Conn      *websocket.Conn
	Heartbeat float64
	OnEvent   func(p Payload)
	Context   context.Context
	Kill      context.CancelFunc
	Endpoint  string
	Session   string
	IP        net.IP
	Port      uint16
	Token     string
	UdpConn   *net.UDPConn
}

type VoiceState struct {
	UserId    string `json:"user_id"`
	ChannelId string `json:"channel_id"`
}

type Payload struct {
	T  string                 `json:"t"`
	S  int                    `json:"s"`
	Op int8                   `json:"op"`
	D  map[string]interface{} `json:"d"`
}

// Methods
func (b *Bot) New(token string) error {
	b.Token = token
	return b.init()
}

func (b *Bot) init() error {
	err := b.getBotInfo()
	if err != nil {
		return err
	}
	err = b.getBotGuilds()
	if err != nil {
		return err
	}
	err = b.getGuildsChannels()
	if err != nil {
		panic(err)
	}
	err = b.connectGateway()
	if err != nil {
		return err
	}
	b.OnEvent = b.handleEvents
	if b.Ready {
		b.WG.Add(1)
		go b.readMessages()
		b.WG.Add(1)
		go b.handleHeartbeat()
		b.WG.Add(1)
		b.WG.Wait()
	}
	return nil
}

func (b *Bot) http(method string, url string, body io.ReadCloser) (http.Client, http.Request) {
	client := http.Client{}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		panic(err)
	}
	req.Header.Add(HttpHeaderUserAgent, DiscordHttpUserAgent)
	req.Header.Add(HttpHeaderAuthorization, "Bot "+b.Token)
	return client, *req
}

func (b *Bot) getBotInfo() error {
	client, req := b.http(http.MethodGet, DiscordApiBase+DiscordUserInfoEndpoint, nil)
	resp, err := client.Do(&req)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return errors.New(ErrorInvalidAuthorizationToken)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	err = resp.Body.Close()
	if err != nil {
		return err
	}
	err = json.Unmarshal(body, &b)
	if err != nil {
		return err
	}
	return nil
}

func (b *Bot) getBotGuilds() error {
	client, req := b.http(http.MethodGet, DiscordApiBase+DiscordUserGuildsEndpoint, nil)
	resp, err := client.Do(&req)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return errors.New(ErrorInvalidAuthorizationToken)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	err = resp.Body.Close()
	if err != nil {
		return err
	}
	err = json.Unmarshal(body, &b.Guilds)
	if err != nil {
		return err
	}
	return nil
}

func (b *Bot) getGuildsChannels() error {
	for index, guild := range b.Guilds {
		client, req := b.http(http.MethodGet, DiscordApiBase+fmt.Sprintf(DiscordGuildChannelsEndpoint, guild.Id), nil)
		resp, err := client.Do(&req)
		if err != nil {
			return err
		}
		if resp.StatusCode == http.StatusUnauthorized {
			return errors.New(ErrorInvalidAuthorizationToken)
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		err = resp.Body.Close()
		if err != nil {
			return err
		}
		err = json.Unmarshal(body, &b.Guilds[index].Channels)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *Bot) connectGateway() error {
	client, req := b.http(http.MethodGet, DiscordApiBase+DiscordGatewayBotEndpoint, nil)
	resp, err := client.Do(&req)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return errors.New(ErrorInvalidAuthorizationToken)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	err = resp.Body.Close()
	if err != nil {
		return err
	}
	var gateway map[string]interface{}
	err = json.Unmarshal(body, &gateway)
	b.Context = context.Background()
	b.Conn, _, err = websocket.Dial(b.Context, fmt.Sprint(gateway["url"]), nil)
	if err != nil {
		return err
	}
	var bytes []byte
	if _, bytes, err = b.Conn.Read(b.Context); err != nil {
		return err
	}
	var payload Payload
	err = json.Unmarshal(bytes, &payload)
	if err != nil {
		return err
	}
	if payload.Op == GwOpcHello {
		b.Heartbeat = payload.D["heartbeat_interval"].(float64)
		var payload Payload
		payload.D = make(map[string]interface{})
		payload.D["token"] = b.Token
		payload.Op = GwOpcIdentify
		properties := make(map[string]interface{})
		properties["$os"] = "linux"
		properties["$browser"] = "hertz"
		properties["$device"] = "hertz"
		payload.D["properties"] = properties
		bytes, err := json.Marshal(&payload)
		if err != nil {
			return err
		}
		err = b.Conn.Write(b.Context, websocket.MessageText, bytes)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(os.Stdout, "[Bot] Identify (%d bytes)\n", len(bytes))
		_, bytes, err = b.Conn.Read(b.Context)
		if err != nil {
			return err
		}
		payload = Payload{}
		err = json.Unmarshal(bytes, &payload)
		if payload.Op == GwOpcDispatch && payload.T == "READY" {
			b.Ready = true
		}
		return nil
	} else {
		return errors.New(ErrorGatewayConnection)
	}
}

func (b *Bot) readMessages() {
	for {
		var msg = make([]byte, 1e5)
		var err error
		if _, msg, err = b.Conn.Read(b.Context); err != nil && err != io.EOF {
			panic(err)
		}
		var p Payload
		err = json.Unmarshal(msg, &p)
		if err != nil {
			panic(err)
		}
		b.OnEvent(p)
	}
}

func (b *Bot) handleEvents(p Payload) {
	if p.T == GwEvGuildCreate {
		var voiceStates []VoiceState
		bytes, err := json.Marshal(p.D["voice_states"])
		if err != nil {
			panic(err)
		}
		err = json.Unmarshal(bytes, &voiceStates)
		if err != nil {
			panic(err)
		}
		b.VoiceStates = voiceStates
	} else if p.T == GwEvMessageCreate {
		if p.D["content"] == "!s" {
			_ = b.Conn.Close(websocket.StatusNormalClosure, "PROGRAM_END")
			b.WG.Done()
			os.Exit(0)
		} else if p.D["content"] == "!j" {
			// Connect to voice channel
			var voice Voice
			voice.Context, voice.Kill = context.WithCancel(b.Context)
			voice.GuildId = fmt.Sprintf("%s", p.D["guild_id"])
			for _, state := range b.VoiceStates {
				author := p.D["author"].(map[string]interface{})
				if state.UserId == author["id"] {
					voice.ChannelId = state.ChannelId
				}
			}
			b.Voices = append(b.Voices, voice)
			b.startVoiceConnection(&voice)
		}
	} else if p.T == GwEvInvalidSession {
		_ = b.Conn.Close(websocket.StatusAbnormalClosure, "INVALID_SESSION")
		b.WG.Done()
		os.Exit(ErrorInvalidSession)
	} else if p.T == GwEvVoiceServerUpdate {
		for index, v := range b.Voices {
			if v.GuildId == p.D["guild_id"] {
				v.Endpoint = fmt.Sprintf("ws://%s", p.D["endpoint"])
				v.Token = fmt.Sprintf("%s", p.D["token"])
				v.connect(b.Id)
				b.Voices[index] = v
			}
		}
	} else if p.T == GwEvVoiceStateUpdate {
		for index, v := range b.Voices {
			if p.D["channel_id"] == v.ChannelId {
				v.Session = p.D["session_id"].(string)
				b.Voices[index] = v
			}
		}
	}
}

func (b *Bot) handleHeartbeat() {
	for {
		heartbeat := Payload{Op: GwOpcHeartbeat}
		bytes, err := json.Marshal(heartbeat)
		if err != nil {
			b.WG.Done()
			panic(err)
		}
		err = b.Conn.Write(b.Context, websocket.MessageText, bytes)
		if err != nil {
			b.WG.Done()
			panic(err)
		}
		_, err = fmt.Fprintf(os.Stdout, "[Bot] Heartbeat (%d bytes)\n", len(bytes))
		if err != nil {
			panic(err)
		}
		time.Sleep(time.Duration(b.Heartbeat) * time.Millisecond)
	}
}

func (v *Voice) handleVoiceHeartbeat() {
	for {

		heartbeat := map[string]interface{}{
			"d":  1501184119561,
			"op": 3,
		}
		bytes, err := json.Marshal(heartbeat)
		if err != nil {
			//v.WG.Done()
			panic(err)
		}
		err = v.Conn.Write(v.Context, websocket.MessageText, bytes)
		if err != nil {
			//b.WG.Done()
			panic(err)
		}
		_, err = fmt.Fprintf(os.Stdout, "[Bot] Voice Heartbeat (Session %s) (%d bytes)\n", v.Session, len(bytes))
		if err != nil {
			panic(err)
		}
		///if (b.Voice.)
		time.Sleep(time.Duration(v.Heartbeat) * time.Millisecond)
	}
}

func (b *Bot) startVoiceConnection(v *Voice) {
	var payload Payload
	payload.Op = GwOpcVoiceStateUpdate
	payload.D = map[string]interface{}{
		"guild_id":   v.GuildId,
		"channel_id": v.ChannelId,
		"self_mute":  false,
		"self_deaf":  true,
	}
	bytes, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	err = b.Conn.Write(b.Context, websocket.MessageText, bytes)
	if err != nil {
		panic(err)
	}
}

func (v *Voice) connect(botId string) {
	// Send identify to voice server
	var err error
	v.Conn, _, err = websocket.Dial(v.Context, v.Endpoint, nil)

	if err != nil {
		panic(err)
	}

	var payload Payload
	payload.Op = GwOpcDispatch
	payload.D = map[string]interface{}{
		"server_id":  v.GuildId,
		"user_id":    botId,
		"session_id": v.Session,
		"token":      v.Token,
	}
	bytes, err := json.Marshal(payload)

	if err != nil {
		panic(err)
	}

	err = v.Conn.Write(v.Context, websocket.MessageText, bytes)

	if err != nil {
		panic(err)
	}

	_, bytes, err = v.Conn.Read(v.Context)

	if err != nil {
		panic(err)
	}

	var pa Payload
	err = json.Unmarshal(bytes, &pa)

	if err != nil {
		panic(err)
	}

	v.Heartbeat = pa.D["heartbeat_interval"].(float64)
	v.OnEvent = v.handleVoiceEvents
	go v.handleVoiceHeartbeat()
	go v.readVoiceData()
}

func (v *Voice) readVoiceData() {
	for {
		var msg = make([]byte, 1e5)
		var err error
		if _, msg, err = v.Conn.Read(v.Context); err != nil && err != io.EOF {
			panic(err)
		}
		var p Payload
		err = json.Unmarshal(msg, &p)
		v.OnEvent(p)
	}
}

func (v *Voice) handleVoiceEvents(p Payload) {
	if p.Op == 0x2 {

		ip := net.ParseIP(p.D["ip"].(string))
		port := p.D["port"].(float64)
		ssrc := p.D["ssrc"].(float64)
		// Open UDP connection
		var err error
		v.UdpConn, err = net.DialUDP("udp", nil, &net.UDPAddr{IP: ip, Port: int(port)})

		if err != nil {
			panic(err)
		}

		bytes := make([]byte, 70)
		binary.BigEndian.PutUint32(bytes, uint32(ssrc))
		_, err = v.UdpConn.Write(bytes)

		if err != nil {
			panic(err)
		}

		var payload Payload
		payload.Op = 0x1
		payload.D = map[string]interface{}{
			"protocol": "udp",
			"data": map[string]interface{}{
				"address": string(bytes[4:15]),
				"port":    binary.BigEndian.Uint16(bytes[68:70]),
				"mode":    "xsalsa20_poly1305_lite",
			},
		}

		bytes, err = json.Marshal(payload)

		err = v.Conn.Write(v.Context, websocket.MessageText, bytes)

		if err != nil {
			panic(err)
		}

		fmt.Println("[Bot] UDP Connection established to", v.UdpConn.RemoteAddr().String())
	}
}
