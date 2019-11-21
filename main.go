package main

import (
	"encoding/json"
	"errors"
	"flag"
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

// Discord API
const (
	DISCORD_API_ROOT = "https://discordapp.com/api/v"
	DISCORD_API_VERSION  = "6"
	DISCORD_API_BASE = DISCORD_API_ROOT + DISCORD_API_VERSION
	DISCORD_HTTP_USER_AGENT = "DiscordBot (" + DISCORD_API_ROOT + ", " + DISCORD_API_VERSION + ")"

	HTTP_METHOD_GET = "GET"
	HTTP_METHOD_POST = "POST"
	HTTP_METHOD_DELETE = "DELETE"
	HTTP_METHOD_PUT = "PUT"

	HTTP_HEADER_USER_AGENT = "User-Agent"
	HTTP_HEADER_AUTHORIZATION = "Authorization"

	ERROR_BASE = "Error: "
	ERROR_INVALID_AUTHORIZATION_TOKEN = "Invalid authentication token"
)

type Bot struct {
	Id string
	Username string
	Token string
}

func (b *Bot) New(token string) error {
	b.Token = token
	return b.init()
}

func (b *Bot) init() error {
	client := http.Client{}
	req, err := http.NewRequest(HTTP_METHOD_GET, DISCORD_API_BASE + "/users/@me", nil)
	if err != nil {
		return err
	}
	req.Header.Add(HTTP_HEADER_USER_AGENT, DISCORD_HTTP_USER_AGENT)
	req.Header.Add(HTTP_HEADER_AUTHORIZATION, "Bot " + b.Token)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return berr(ERROR_INVALID_AUTHORIZATION_TOKEN)
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

// Helper functions
func berr(error string) error {
	return errors.New(ERROR_BASE + error)
}