package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/schollz/progressbar/v3"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2/clientcredentials"
)

type Stream struct {
	Ts                            time.Time   `json:"ts"`
	Username                      string      `json:"username"`
	Platform                      string      `json:"platform"`
	MSPlayed                      int64       `json:"ms_played"`
	ConnCountry                   string      `json:"conn_country"`
	IPAddrDecrypted               string      `json:"ip_addr_decrypted"`
	UserAgentDecrypted            *string     `json:"user_agent_decrypted"`
	MasterMetadataTrackName       string      `json:"master_metadata_track_name"`
	MasterMetadataAlbumArtistName string      `json:"master_metadata_album_artist_name"`
	MasterMetadataAlbumAlbumName  string      `json:"master_metadata_album_album_name"`
	SpotifyTrackURI               string      `json:"spotify_track_uri"`
	EpisodeName                   *string     `json:"episode_name"`
	EpisodeShowName               *string     `json:"episode_show_name"`
	SpotifyEpisodeURI             *string     `json:"spotify_episode_uri"`
	ReasonStart                   ReasonStart `json:"reason_start"`
	ReasonEnd                     ReasonEnd   `json:"reason_end"`
	Shuffle                       bool        `json:"shuffle"`
	Skipped                       *bool       `json:"skipped"`
	Offline                       bool        `json:"offline"`
	OfflineTimestamp              int64       `json:"offline_timestamp"`
	IncognitoMode                 bool        `json:"incognito_mode"`

	ArtworkURL *string `json:"artwork_url"`
}

type ReasonStart string

const (
	Appload               ReasonStart = "appload"
	Clickrow              ReasonStart = "clickrow"
	Playbtn               ReasonStart = "playbtn"
	ReasonStartBackbtn    ReasonStart = "backbtn"
	ReasonStartFwdbtn     ReasonStart = "fwdbtn"
	ReasonStartRemote     ReasonStart = "remote"
	ReasonStartTrackdone  ReasonStart = "trackdone"
	ReasonStartTrackerror ReasonStart = "trackerror"
)

type ReasonEnd string

const (
	Endplay                   ReasonEnd = "endplay"
	Logout                    ReasonEnd = "logout"
	ReasonEndBackbtn          ReasonEnd = "backbtn"
	ReasonEndFwdbtn           ReasonEnd = "fwdbtn"
	ReasonEndRemote           ReasonEnd = "remote"
	ReasonEndTrackdone        ReasonEnd = "trackdone"
	ReasonEndTrackerror       ReasonEnd = "trackerror"
	UnexpectedExit            ReasonEnd = "unexpected-exit"
	UnexpectedExitWhilePaused ReasonEnd = "unexpected-exit-while-paused"
	Unknown                   ReasonEnd = "unknown"
)

func prettyPrint(i interface{}) {
	s, _ := json.MarshalIndent(i, "", "\t")
	fmt.Println(string(s))
}

func readEndsongFiles() []Stream {
	var allStreams []Stream

	fileIndex := 0
	for {
		var fileStreams []Stream

		fileName := fmt.Sprintf("endsong_%d.json", fileIndex)
		if _, err := os.Stat(fileName); err == nil {
			// fileName exists

			content, err := ioutil.ReadFile(fileName)
			if err != nil {
				log.Fatal("Error when opening file: ", err)
			}

			err = json.Unmarshal(content, &fileStreams)
			if err != nil {
				log.Fatal("Error during Unmarshal(): ", err)
			}

			allStreams = append(allStreams, fileStreams...)

			fmt.Printf("%s done!\n", fileName)
			fileIndex++
		} else if errors.Is(err, os.ErrNotExist) {
			// fileName does *not* exist
			if fileIndex == 0 {
				log.Fatal("No endsong_0.json file found")
			}
			break
		} else {
			// Schrodinger: file may or may not exist. See err for details.
			// Therefore, do *NOT* use !os.IsNotExist(err) to test for file existence
			log.Fatal("Error when opening file: ", err)
		}
	}

	return allStreams
}

func addStreamArtworks(allStreams []Stream) []Stream {
	godotenv.Load()
	ctx := context.Background()
	config := &clientcredentials.Config{
		ClientID:     os.Getenv("SPOTIFY_ID"),
		ClientSecret: os.Getenv("SPOTIFY_SECRET"),
		TokenURL:     spotifyauth.TokenURL,
	}
	token, err := config.Token(ctx)
	if err != nil {
		log.Fatalf("couldn't get token: %v", err)
	}

	httpClient := spotifyauth.New().Client(ctx, token)
	client := spotify.New(httpClient, spotify.WithRetry(true))

	bar := progressbar.Default(int64(len(allStreams)))
	artworkByID := make(map[string]string)
	for i := 0; i < len(allStreams); i++ {
		splitTrackURI := strings.Split(allStreams[i].SpotifyTrackURI, ":")
		if splitTrackURI[0] != "spotify" || splitTrackURI[1] != "track" || len(splitTrackURI) < 3 {
			log.Printf("SpotifyTrackURI = %q | ts = %q | %q by %q\n", allStreams[i].SpotifyTrackURI, allStreams[i].Ts.Format(time.RFC3339), allStreams[i].MasterMetadataTrackName, allStreams[i].MasterMetadataAlbumArtistName)
			continue
		}

		trackID := splitTrackURI[2]

		trackArtwork, ok := artworkByID[trackID]
		if !ok {
			track, err := client.GetTrack(ctx, spotify.ID(trackID))
			if err != nil {
				log.Fatal("Error when getting Spotify track: ", err)
			}
			trackArtwork = track.Album.Images[0].URL
			artworkByID[trackID] = trackArtwork
		}
		allStreams[i].ArtworkURL = &trackArtwork

		bar.Add(1)
	}
	fmt.Printf("%d artworks total.\n", len(artworkByID))

	return allStreams
}

func writeSortedFile(allStreams []Stream) {
	sortedFile, _ := os.Create("sorted_streams.json")
	defer sortedFile.Close()
	enc := json.NewEncoder(sortedFile)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "    ")
	if err := enc.Encode(allStreams); err != nil {
		log.Fatal("Error when encoding file: ", err)
	}
	fmt.Printf("%d streams sorted!\n", len(allStreams))
}

func main() {
	// Read unsorted streams files
	allStreams := readEndsongFiles()
	allStreamsCount := len(allStreams)
	fmt.Printf("%d streams total.\n", allStreamsCount)
	if allStreamsCount == 0 {
		return
	}

	// Sort streams
	sort.SliceStable(allStreams, func(i, j int) bool {
		return allStreams[i].Ts.Before(allStreams[j].Ts)
	})

	// Add artwork URL to streams
	allStreams = addStreamArtworks(allStreams)

	// Write sorted streams file
	writeSortedFile(allStreams)
}
