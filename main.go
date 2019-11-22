package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
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
	bot.OnMessage = func(p Payload) {
		if p.T == GwEvMessageCreate {
			if p.D["content"] == "!s" {
				_ = bot.Socket.Close(websocket.StatusNormalClosure, "PROGRAM_END")
				bot.WG.Done()
				os.Exit(0)
			}
		}
	}
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
	GwOpcDispatch            = 0x0 // R
	GwOpcHeartbeat           = 0x1 // s
	GwOpcIdentify            = 0x2 // s
	GwOpcStatusUpdate        = 0x3 // s
	GwOpcVoiceStateUpdate    = 0x4 // s
	GwOpcResume              = 0x6 // s
	GwOpcReconnect           = 0x7 // R
	GwOpcRequestGuildMembers = 0x8 // s
	GwOpcInvalidSession      = 0x9 // R
	GwOpcHello               = 0xa // R
	GwOpcHeartbeatACK        = 0xb // R

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
)

// Types
type Bot struct {
	Id        string
	Username  string
	Token     string
	Guilds    []Guild
	Socket    *websocket.Conn
	Context   context.Context
	Heartbeat float64
	WG        sync.WaitGroup
	Ready     bool
	OnMessage func(p Payload)
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
		return err
	}
	err = b.connectGateway()
	if err != nil {
		return err
	}
	if b.Ready {
		b.WG.Add(1)
		go b.readMessages()
		b.WG.Add(1)
		go b.handleHeartbeat()
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
	b.Socket, _, err = websocket.Dial(b.Context, fmt.Sprint(gateway["url"]), nil)
	if err != nil {
		return err
	}
	var bytes []byte
	if _, bytes, err = b.Socket.Read(b.Context); err != nil {
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
		err = b.Socket.Write(b.Context, websocket.MessageText, bytes)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(os.Stdout, "[Bot] Identify (%d bytes)\n", len(bytes))
		_, bytes, err = b.Socket.Read(b.Context)
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
		if _, msg, err = b.Socket.Read(b.Context); err != nil && err != io.EOF {
			panic(err)
		}
		var p Payload
		err = json.Unmarshal(msg, &p)
		if err != nil {
			panic(err)
		}
		b.OnMessage(p)
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
		err = b.Socket.Write(b.Context, websocket.MessageText, bytes)
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