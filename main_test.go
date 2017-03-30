package main

import "testing"
import "fmt"

// --------------------------------------------------------------------
// TESTED FUNCTION : parseYoutubePlaylistLink
// --------------------------------------------------------------------
func TestYoutubePlaylistParser(t *testing.T){

	// Check if the if the playlist ID is extracted from the link
	var youtubePlaylistUrl = "https://www.youtube.com/playlist?list=PLM32QYSlkwzg3dzrvWHr7MdN_MF1LMjVN"
	var parsedUrl = parseYoutubePlaylistLink(youtubePlaylistUrl)
	if parsedUrl != "PLM32QYSlkwzg3dzrvWHr7MdN_MF1LMjVN" {
		t.Error("Exepected youtube playlist link parser to return PLM32QYSlkwzg3dzrvWHr7MdN_MF1LMjVN, but got ", parsedUrl)
	}
}

// --------------------------------------------------------------------
// TESTED FUNCTION : sanitizeLink
// --------------------------------------------------------------------
func TestLinkSanitaze(t *testing.T){	

	// Check if the < and > characters are removed from the link
	var embedLink = "<https://www.youtube.com/watch?v=mJLINJYUdnQ>"
	var parsedLink = sanitizeLink(embedLink)
	if parsedLink != "https://www.youtube.com/watch?v=mJLINJYUdnQ" {
		t.Error("Exepected sanitizeLink function to return https://www.youtube.com/watch?v=mJLINJYUdnQ, but got", parsedLink)
	}


}