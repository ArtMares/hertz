package main

//Native libraries
import "fmt"
import "strings"
import "os"

//External libraries
import "github.com/bwmarrin/discordgo"

//Defining voice structure
type Voice struct {
	VoiceConnection *discordgo.VoiceConnection
	Channel         string
	Guild           string
}

var token string
var prefix string
var voiceConnections []Voice

func main() {
	//Creating a new Discord Go instance

	if len(os.Args) >= 3 {
		token = os.Args[1]
		prefix = os.Args[2]
	} else {
		fmt.Println("Please enter a token and a prefix")
		return
	}

	bot, err := discordgo.New("Bot " + token)

	//Listening for possible error on bot creation
	if err != nil {
		fmt.Println(err)
		return
	}

	//Adding the handler that will listen for users commands
	go bot.AddHandler(commandHandler)

	//Connection the bot to the Discord API and listening for errors
	err = bot.Open()

	//Informing the user the bot has started and wating for a channel return to prevent the program to stop
	fmt.Println("The bot is launched, to stop it, press CTRL+C")
	<-make(chan int)
	return
}

func commandHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	var splittedCommand []string = strings.Split(m.Content, "")
	var command string = strings.Join(splittedCommand[1:], "")
	channel, err := s.State.Channel(m.ChannelID)
	if err != nil {
		fmt.Println(err)
	}
	guild, err := s.State.Guild(channel.GuildID)
	if err != nil {
		fmt.Println(err)
	}

	//Checking the prefix
	if splittedCommand[0] == prefix {
		voiceChannel := findVoiceChannelID(guild, m)
		if command == "connect" {
			voiceConnections = append(voiceConnections, connectToVoiceChannel(s, channel.GuildID, voiceChannel))
		} else if command == "disconnect" {
			disconnectFromVoiceChannel(channel.GuildID, voiceChannel)
		}
	}
}

func disconnectFromVoiceChannel(guild string, channel string) {
	for _, voice := range voiceConnections {
		if voice.Channel == channel && voice.Guild == guild {
			_ = voice.VoiceConnection.Disconnect()
		}
	}
}

func findVoiceChannelID(guild *discordgo.Guild, message *discordgo.MessageCreate) string {
	var channelID string
	for _, vs := range guild.VoiceStates {
		if vs.UserID == message.Author.ID {
				channelID = vs.ChannelID
			}
	}
	return channelID
}

func connectToVoiceChannel(bot *discordgo.Session, guild string, channel string) Voice {
	vs, err := bot.ChannelVoiceJoin(guild, channel, false, true)
	if err != nil {
		fmt.Println(err)
	}
	return Voice{
		VoiceConnection: vs,
		Channel:         channel,
		Guild:           guild,
	}

}
