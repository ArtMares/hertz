package main

import "testing"

func TestYoutubePlaylistParser(t *testing.T){
	var youtubePlaylistUrl = "https://www.youtube.com/playlist?list=PLM32QYSlkwzg3dzrvWHr7MdN_MF1LMjVN"
	var parsedUrl = parseYoutubePlaylistLink(youtubePlaylistUrl)
	if parsedUrl != "PLM32QYSlkwzg3dzrvWHr7MdN_MF1LMjVN" {
		t.Error("Exepected youtube playlist link parser to return PLM32QYSlkwzg3dzrvWHr7MdN_MF1LMjVN, but got ", parsedUrl)
	}
}