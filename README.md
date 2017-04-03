<p align="center">
    <img src="http://imgur.com/download/FaZTpoJ" width="600">
</p>

<p align="center">
    <img src="https://travis-ci.org/romainisnel/hertz.svg?branch=master">
    <img src="https://img.shields.io/badge/version-1-blue.svg">
    <img src="https://goreportcard.com/badge/github.com/romainisnel/hertz">
</p>

### Installation

The bot is really simple to install, just follow this steps.

+ Download the executable corresponding to your system in the [release](https://github.com/romainisnel/hertz/releases) section
+ Configurate your bot, create a config.json file in the same folder as the bot, it must have the following structure : 
```json
{
	"token" : "USER_TOKEN", 
	"prefix" : "PREFIX_CHAR",
	"youtube_token" : "YOUTUBE_API_KEY",
	"souncloud_token" : "SOUNDCLOUD_API_KEY"
}
```
**token** : The token is a unique key that allow you to connect your bot account to the Discord servers. To get one you must go to the [discord developper site](https://discordapp.com/developers/applications/me) and register a new application.

![Create Application](http://i.imgur.com/2MZBEqp.png)
On this screen, click on "New Application"

![Create Application Next](http://i.imgur.com/JygNCUx.png)
Enter a name and a description, and even an avatar if you want, then click on "Create Application"

![Make a Bot Account](http://i.imgur.com/eSZDtqp.png)
This should apear on your screen. At this point, you just have to click on the "Create a Bot User" button and confirm the action

![Get to the token](http://i.imgur.com/Fh3K0cm.png)
Finally you should have this screen. The last thing you must do is copy and paste the **Token** value in the config.json file, in the field token.

**prefix** : This is the character that will come in front of the command words. It is used to recognize commands among all the messages. (Exemple : if prefix is **!** you'll have to write **!play** to use the play command)

**youtube_token** : This is a Youtube Data API (v3) key. You can get one on the Google Cloud Console. It is used to get the informations of youtube playlists in order to play them.

**souncloud_token** : This is a Soundcloud API key. You can get one on the developers section of Soundcloud. It is used to get the informations of soundcloud tracks in order to play them.

+ Launch the executable, and that's it, up and running