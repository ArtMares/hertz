package main

//Native libraries
import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

//External libraries
import (
	"github.com/bwmarrin/dgvoice"
	"github.com/bwmarrin/discordgo"
	"github.com/franela/goreq"
	"github.com/otium/ytdl"
)

var token string
var prefix string
var soundcloudToken string
var youtubeToken string
var voiceConnections []Voice
var queue []Song

func main() {
	//Creating a new Discord Go instance

	if len(os.Args) >= 4 {
		token = os.Args[1]
		soundcloudToken = os.Args[2]
		youtubeToken = os.Args[3]
		prefix = os.Args[4]
		fmt.Println("Configuration loaded from params")
	} else if loadConfiguration() {
		fmt.Println("Configuration loaded from JSON config file")
	} else {
		fmt.Println("Please enter a token, a soundcloud token a youtube token and a prefix or add a config.json file")
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

	if err != nil {
		fmt.Println(err)
		return
	}

	//Informing the user the bot has started and wating for a channel return to prevent the program to stop
	fmt.Println("The bot is launched, to stop it, press CTRL+C")
	<-make(chan int)
	bot.Close()
	return
}

func loadConfiguration() bool {
	file, err := ioutil.ReadFile("./config.json")
	if err != nil {
		fmt.Println(err)
		return false
	}
	var config Configuration
	json.Unmarshal(file, config)
	token = config.Token
	prefix = config.Prefix
	soundcloudToken = config.SoundcloudToken
	youtubeToken = config.YoutubeToken
	return true
}

func commandHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	var commandArgs []string = strings.Split(m.Content, " ")
	channel, err := s.State.Channel(m.ChannelID)
	if err != nil {
		fmt.Println(err)
	}
	guild, err := s.State.Guild(channel.GuildID)
	if err != nil {
		fmt.Println(err)
	}
	voiceChannel := findVoiceChannelID(guild, m)
	if commandArgs[0] == prefix+"connect" {
		voiceConnections = append(voiceConnections, connectToVoiceChannel(s, channel.GuildID, voiceChannel))
	} else if commandArgs[0] == prefix+"disconnect" {
		disconnectFromVoiceChannel(channel.GuildID, voiceChannel)
	} else if commandArgs[0] == prefix+"play" {
		go playAudioFile(sanitizeLink(commandArgs[1]), channel.GuildID, voiceChannel, "web")
	} else if commandArgs[0] == prefix+"stop" {
		stopAudioFile(channel.GuildID, voiceChannel)
	} else if commandArgs[0] == prefix+"youtube" {
		go playYoutubeLink(sanitizeLink(commandArgs[1]), channel.GuildID, voiceChannel)
	} else if commandArgs[0] == prefix+"soundcloud" {
		go playSoundcloudLink(sanitizeLink(commandArgs[1]), channel.GuildID, voiceChannel)
	} else if commandArgs[0] == prefix+"playlist" {
		go playYoutubePlaylist(commandArgs[1], channel.GuildID, voiceChannel)
	}
}

func disconnectFromVoiceChannel(guild string, channel string) {
	for index, voice := range voiceConnections {
		if voice.Guild == guild {
			_ = voice.VoiceConnection.Disconnect()
			dgvoice.KillPlayer()
			voiceConnections = append(voiceConnections[:index], voiceConnections[index+1:]...)
		}
	}
}

func sanitizeLink(link string) string {
	firstTreatment := strings.Replace(link, "<", "", 1)
	return strings.Replace(firstTreatment, ">", "", 1)
}

func parseYoutubePlaylistLink(link string) string {
	standardPlaylistSanitize := strings.Replace(link, "https://www.youtube.com/playlist?list=", "", 1)
	return standardPlaylistSanitize
}

func findVoiceConnection(guild string, channel string) (Voice, int) {
	var voiceConnection Voice
	var index int
	for i, vc := range voiceConnections {
		if vc.Guild == guild {
			voiceConnection = vc
			index = i
		}
	}
	return voiceConnection, index

}

func nextSong() {
	if len(queue) > 0 {
		go playAudioFile(queue[0].Link, queue[0].Guild, queue[0].Channel, queue[0].Type)
		queue = append(queue[:0], queue[1:]...)
	} else {
		return
	}
}

func addSong(song Song) {
	queue = append(queue, song)
}

func playAudioFile(file string, guild string, channel string, linkType string) {
	voiceConnection, index := findVoiceConnection(guild, channel)
	switch voiceConnection.PlayerStatus {
	case IS_NOT_PLAYING:
		voiceConnections[index].PlayerStatus = IS_PLAYING
		dgvoice.PlayAudioFile(voiceConnection.VoiceConnection, file)
		voiceConnections[index].PlayerStatus = IS_NOT_PLAYING
		nextSong()
	case IS_PLAYING:
		addSong(Song{
			Link:    file,
			Type:    linkType,
			Guild:   guild,
			Channel: channel,
		})
	}
}

func stopAudioFile(guild string, channel string) {
	_, index := findVoiceConnection(guild, channel)
	voiceConnections[index].PlayerStatus = IS_NOT_PLAYING
	dgvoice.KillPlayer()
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

	checkForDoubleVoiceConnection(guild, channel)

	if err != nil {
		fmt.Println(err)
	}
	return Voice{
		VoiceConnection: vs,
		Channel:         channel,
		Guild:           guild,
		PlayerStatus:    IS_NOT_PLAYING,
	}

}

func checkForDoubleVoiceConnection(guild string, channel string) {
	for index, voice := range voiceConnections {
		if voice.Guild == guild {
			voiceConnections = append(voiceConnections[:index], voiceConnections[index+1:]...)
		}
	}
}

func playYoutubeLink(link string, guild string, channel string) {
	video, err := ytdl.GetVideoInfo(link)

	if err != nil {
		fmt.Println(err)
		return // Returning to avoid crash when video informations could not be found
	}

	for _, format := range video.Formats {
		if format.AudioEncoding == "opus" || format.AudioEncoding == "aac" || format.AudioEncoding == "vorbis" {
			data, err := video.GetDownloadURL(format)
			if err != nil {
				fmt.Println(err)
			}
			url := data.String()
			go playAudioFile(url, guild, channel, "youtube")
			return
		}
	}

}

func playSoundcloudLink(link string, guild string, channel string) {
	var scRequestUri string = "https://api.soundcloud.com/resolve?url=" + link + "&client_id=" + soundcloudToken
	res, err := goreq.Request{
		Uri:          scRequestUri,
		MaxRedirects: 2,
		Timeout:      5000 * time.Millisecond,
	}.Do()
	if err != nil {
		fmt.Println(err)
	}
	var soundcloudData SoundcloudResponse
	res.Body.FromJsonTo(&soundcloudData)
	soundcloudData.Link += "&client_id=" + soundcloudToken
	go playAudioFile(soundcloudData.Link, guild, channel, "soundcloud")
}

func playYoutubePlaylist(link string, guild string, channel string) {
	var youtubeRequestLink string = "https://www.googleapis.com/youtube/v3/playlistItems?part=snippet&maxResults=50&playlistId=" + link + "&key=" + youtubeToken
	res, err := goreq.Request{
		Uri:          youtubeRequestLink,
		MaxRedirects: 2,
		Timeout:      5000 * time.Millisecond,
	}.Do()
	if err != nil {
		fmt.Println(err)
	}
	var youtubeData YoutubeRoot
	res.Body.FromJsonTo(&youtubeData)
	for _, youtubeVideo := range youtubeData.Items {
		var videoURL string = "https://www.youtube.com/watch?v=" + youtubeVideo.Snippet.Resource.VideoID
		go playYoutubeLink(videoURL, guild, channel)
	}

}
