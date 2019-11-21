package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"golang.org/x/net/websocket"
	"io"
	"io/ioutil"
	"net/http"
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
	DiscordGwOpcHeartbeat           = 0x1 // S
	DiscordGwOpcIdentify            = 0x2 // S
	DiscordGwOpcStatusUpdate        = 0x3 // S
	DiscordGwOpcVoiceStateUpdate    = 0x4 // S
	DiscordGwOpcResume              = 0x6 // S
	DiscordGwOpcReconnect           = 0x7 // R
	DiscordGwOpcRequestGuildMembers = 0x8 // S
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
	Id         string
	Username   string
	Token      string
	Guilds     []Guild
	Socket     *websocket.Conn
	Heartbeart float64
	WG         sync.WaitGroup
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
	T  string
	S  string
	Op int8
	D  map[string]interface{}
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
	/*b.WG.Add(1)
	go b.handleHeartbeat()
	b.WG.Wait()*/
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
	b.Socket, err = websocket.Dial(fmt.Sprint(gateway["url"]), "", "http://localhost")
	if err != nil {
		return err
	}
	var msg = make([]byte, 512)
	var n int
	if n, err = b.Socket.Read(msg); err != nil {
		return err
	}
	var payload Payload
	err = json.Unmarshal(msg[:n], &payload)
	if err != nil {
		return err
	}
	if payload.Op == DiscordGwOpcHello {
		b.Heartbeart = payload.D["heartbeat_interval"].(float64)
		return nil
	} else {
		return berr(ErrorGatewayConnection)
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
		n, err := b.Socket.Write(bytes)
		if err != nil {
			b.WG.Done()
			panic(err)
		}
		_, err = fmt.Fprintf(os.Stdout, "WebSocket Payload : %d bytes sent with Opcode %x\n", n, hearbeat.Op)
		if err != nil {
			panic(err)
		}
		time.Sleep(time.Duration(b.Heartbeart) * time.Millisecond)
	}
}

// Helper functions
func berr(error string) error {
	return errors.New(ErrorBase + error)
}
