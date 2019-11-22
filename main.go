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
	bot.OnMessage = func(msg string) {
		fmt.Println("[Discord] ", msg, len(msg), " bytes")
	}
	err := bot.New(*token)
	if err != nil {
		panic(err)
	}
	return
}

const (
	DiscordApiRoot               = "https://discordapp.com/api/v"
	DiscordApiVersion            = "6"
	DiscordApiBase               = DiscordApiRoot + DiscordApiVersion
	DiscordHttpUserAgent         = "DiscordBot (" + DiscordApiRoot + ", " + DiscordApiVersion + ")"
	DiscordUserInfoEndpoint      = "/users/@me"
	DiscordUserGuildsEndpoint    = "/users/@me/guilds"
	DiscordGuildChannelsEndpoint = "/guilds/%s/channels"
	DiscordGatewayBotEndpoint    = "/gateway/bot"
	DiscordGatewayEndpoint       = "wss://gateway.discord.gg/?v=%s&encoding=json"

	DiscordChannelTypeText     = 0x0
	DiscordChannelTypeDM       = 0x1
	DiscordChannelTypeVoice    = 0x2
	DiscordChannelTypeGroupDM  = 0x3
	DiscordChannelTypeCategory = 0x4
	DiscordChannelTypeNews     = 0x5
	DiscordChannelTypeStore    = 0x6

	DiscordGwOpcDispatch            = 0x0 // R
	DiscordGwOpcHeartbeat           = 0x1 // s
	DiscordGwOpcIdentify            = 0x2 // s
	DiscordGwOpcStatusUpdate        = 0x3 // s
	DiscordGwOpcVoiceStateUpdate    = 0x4 // s
	DiscordGwOpcResume              = 0x6 // s
	DiscordGwOpcReconnect           = 0x7 // R
	DiscordGwOpcRequestGuildMembers = 0x8 // s
	DiscordGwOpcInvalidSession      = 0x9 // R
	DiscordGwOpcHello               = 0xa // R
	DiscordGwOpcHeartbeatACK        = 0xb // R

	HttpHeaderUserAgent     = "User-Agent"
	HttpHeaderAuthorization = "Authorization"

	ErrorBase                      = "Error: "
	ErrorInvalidAuthorizationToken = "Invalid authentication token"
	ErrorGatewayConnection         = "Gateway connection error"
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
	OnMessage func(mgs string)
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
	T  string `json:"t"`
	S  string `json:"s"`
	Op int8 `json:"op"`
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
		return berr(ErrorInvalidAuthorizationToken)
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
		return berr(ErrorInvalidAuthorizationToken)
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
			return berr(ErrorInvalidAuthorizationToken)
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
		return berr(ErrorInvalidAuthorizationToken)
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
	b.Socket, _, err = websocket.Dial(b.Context,fmt.Sprint(gateway["url"]), nil)
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
	if payload.Op == DiscordGwOpcHello {
		b.Heartbeat = payload.D["heartbeat_interval"].(float64)
		var payload Payload
		payload.D = make(map[string]interface{})
		payload.D["token"] = b.Token
		payload.Op = DiscordGwOpcIdentify
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
		if payload.Op == DiscordGwOpcDispatch && payload.T == "READY" {
			b.Ready = true
		}
		return nil
	} else {
		return berr(ErrorGatewayConnection)
	}

	return nil
}

func (b *Bot) readMessages() {
	for {
		var msg = make([]byte, 1e5)
		var err error
		if _, msg, err = b.Socket.Read(b.Context); err != nil && err != io.EOF {
			panic(err)
		}
		b.OnMessage(string(msg))
	}
}

func (b *Bot) handleHeartbeat() {
	for {
		hearbeat := Payload{Op: DiscordGwOpcHeartbeat}
		bytes, err := json.Marshal(hearbeat)
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

// Helper functions
func berr(error string) error {
	return errors.New(ErrorBase + error)
}
