package main

//Native libraries
import "fmt"
import "strings"

//External libraries
import "github.com/bwmarrin/discordgo"

//Defining voice structure
type Voice struct {
	VoiceConnection *discordgo.VoiceConnection
	Channel         string
	Guild           string
}

var token string = "BOT_TOKEN"
var prefix string = "PREFIX_CHAR"
var voiceConnections []Voice

func main() {
	//Creating a new Discord Go instance
	bot, err := discordgo.New("Bot " + token)

	//Listening for possible error on bot creation
	if err != nil {
		fmt.Println("Error launching bot")
		fmt.Println(err)
		return
	}

	//Adding the handler that will listen for users commands
	bot.AddHandler(commandHandler)

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

	//Checking the prefix
	if splittedCommand[0] == prefix {

		if command == "connect" {
			voiceConnections = append(voiceConnections, connectToVoiceChannel(s, "GUILD_ID", "CHANNEL_ID"))
		} else if command == "disconnect" {
			disconnectFromVoiceChannel("GUILD_ID", "CHANNEL_ID")
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

func connectToVoiceChannel(bot *discordgo.Session, guild string, channel string) Voice {
	vs, err := bot.ChannelVoiceJoin(guild, channel, false, true)
	if err != nil {
		fmt.Println("Error connecting to the voice channel")
		fmt.Println(err)

	}
	return Voice{
		VoiceConnection: vs,
		Channel:         channel,
		Guild:           guild,
	}

}
