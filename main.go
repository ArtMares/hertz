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

// Defining bot global variables
var token string
var prefix string
var soundcloudToken string
var youtubeToken string
var voiceConnections []Voice
var queue []Song
var stopChannel chan bool

/**********************
* Program entry point *
***********************/
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

	stopChannel = make(chan bool)

	//Creating a new instance of the bot
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

// This function will load the configuration of the bot from a JSON file
// If the file is not found or another error occur during the file load
// the function will return false so the program knows it has to load the
// configuration in a different way. If the file is loaded, it returns true
func loadConfiguration() bool {
	file, err := ioutil.ReadFile("./config.json")
	if err != nil {
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

// On every new message, this function will be called. This function will read the
// message and if a command is detected, will call the corresponding method
// For exemple, if the play command is found in the message, the function will
// call the playAudioFile function in a new goroutine
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
		stopChannel <- true
	} else if commandArgs[0] == prefix+"youtube" {
		go playYoutubeLink(sanitizeLink(commandArgs[1]), channel.GuildID, voiceChannel)
	} else if commandArgs[0] == prefix+"soundcloud" {
		go playSoundcloudLink(sanitizeLink(commandArgs[1]), channel.GuildID, voiceChannel)
	} else if commandArgs[0] == prefix+"playlist" {
		go playYoutubePlaylist(commandArgs[1], channel.GuildID, voiceChannel)
	}
}

// This function is used to close the connection to a voiceChannel. When called, it will crawl
// the list of opened voice connections and if one is found corresponding to the parameters, it
// will be closed
func disconnectFromVoiceChannel(guild string, channel string) {
	for index, voice := range voiceConnections {
		if voice.Guild == guild {
			_ = voice.VoiceConnection.Disconnect()
			stopChannel <- true
			voiceConnections = append(voiceConnections[:index], voiceConnections[index+1:]...)
		}
	}
}

// This function will sanitize a link that contains < and >, this is used to handle links with
// disabled embed in Discord
func sanitizeLink(link string) string {
	firstTreatment := strings.Replace(link, "<", "", 1)
	return strings.Replace(firstTreatment, ">", "", 1)
}

// This function is used to extract the id of a playlist given a youtube plaulist link
func parseYoutubePlaylistLink(link string) string {
	standardPlaylistSanitize := strings.Replace(link, "https://www.youtube.com/playlist?list=", "", 1)
	return standardPlaylistSanitize
}

// This function will crawl the voice connections and try to find and return a voice connection
// and its index if one is found
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

// This function will call the playAudioFile function in a new goroutine if songs are remaining in the
// queue. If there is no song left in the queue, the function return false
func nextSong() bool {
	if len(queue) > 0 {
		go playAudioFile(queue[0].Link, queue[0].Guild, queue[0].Channel, queue[0].Type)
		queue = append(queue[:0], queue[1:]...)
		return true
	} else {
		return false
	}
}

// This function is used to add an item to the queue
func addSong(song Song) {
	queue = append(queue, song)
}

// This function is used to play every audio files, if the program is already playing, the function
// will add the song to the queue and call the nextSonng function when the current song is over
func playAudioFile(file string, guild string, channel string, linkType string) {
	voiceConnection, index := findVoiceConnection(guild, channel)
	switch voiceConnection.PlayerStatus {
	case IS_NOT_PLAYING:
		voiceConnections[index].PlayerStatus = IS_PLAYING
		dgvoice.PlayAudioFile(voiceConnection.VoiceConnection, file, stopChannel)
		voiceConnections[index].PlayerStatus = IS_NOT_PLAYING
	case IS_PLAYING:
		addSong(Song{
			Link:    file,
			Type:    linkType,
			Guild:   guild,
			Channel: channel,
		})
	}
}

// This function allow the user to stop the current playing file
func stopAudioFile(guild string, channel string) {
	_, index := findVoiceConnection(guild, channel)
	voiceConnections[index].PlayerStatus = IS_NOT_PLAYING
	//dgvoice.KillPlayer()
}

// This function allow the bot to find the voice channel id of the user who called the connect command
func findVoiceChannelID(guild *discordgo.Guild, message *discordgo.MessageCreate) string {
	var channelID string

	for _, vs := range guild.VoiceStates {
		if vs.UserID == message.Author.ID {
			channelID = vs.ChannelID
		}
	}
	return channelID
}

// This function allow the user to connect the bot to a channel. It will ask the voice channel id
// of the user to the findVoiceChannelID function and will then call the ChannelVoiceJoin
// of the discordgo.Session instance. Then it checks if the voice connection already exist and
// return a new Voice object
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

// This function check if there is already an existing voice connection for the givent params
func checkForDoubleVoiceConnection(guild string, channel string) {
	for index, voice := range voiceConnections {
		if voice.Guild == guild {
			voiceConnections = append(voiceConnections[:index], voiceConnections[index+1:]...)
		}
	}
}

// This function is used to play the audio of a youtube video. It use the ytdl pacakge to get
// the video informations and then look for the good format. When a link is found, it calls the
// playAudioFile function in a new goroutine
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

// This function is used to play a soundcloud link. It make a call to the API and to get the stream url
// it then call the playAudioFile function in a new goroutine
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

// This function is used to play a youtube playlist, it will make a call to the youtube API to get the
// link for every video in the playlist. When the items are found, it will iterate and call the playYoutubeLink
// function for every link, they will automatically be added to the queue
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
