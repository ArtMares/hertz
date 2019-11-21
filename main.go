package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
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
	DiscordUserGuildsEndpoint	 = "/users/@me/guilds"
	DiscordGuildChannelsEndpoint = "/guilds/%s/channels"

	DiscordChannelTypeText  = 0x0
	DiscordChannelTypeDM    = 0x1
	DiscordChannelTypeVoice = 0x2
	DiscordChannelTypeGroupDM = 0x3
	DiscordChannelTypeCategory = 0x4
	DiscordChannelTypeNews = 0x5
	DiscordChannelTypeStore = 0x6

	HttpHeaderUserAgent     = "User-Agent"
	HttpHeaderAuthorization = "Authorization"

	ErrorBase                      = "Error: "
	ErrorInvalidAuthorizationToken = "Invalid authentication token"
)

// Types
type Bot struct {
	Id string
	Username string
	Token string
	Guilds []Guild
}

type Guild struct {
	Id string
	Name string
	Permissions int
	Channels []Channel
	Unavailable bool
}

type Channel struct {
	Id string
	Type int
	Bitrate int
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
	return nil
}

func (b *Bot) http(method string, url string, body io.ReadCloser) (http.Client, http.Request) {
	client := http.Client{}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		panic(err)
	}
	req.Header.Add(HttpHeaderUserAgent, DiscordHttpUserAgent)
	req.Header.Add(HttpHeaderAuthorization, "Bot " + b.Token)
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
		client, req := b.http(http.MethodGet, DiscordApiBase + fmt.Sprintf(DiscordGuildChannelsEndpoint, guild.Id), nil)
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

// Helper functions
func berr(error string) error {
	return errors.New(ErrorBase + error)
}