package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"log"
	"net/http"
	"reflect"
)

var (
	auth = spotifyauth.New(
		spotifyauth.WithRedirectURL("http://localhost:3000/callback"),
		spotifyauth.WithScopes(spotifyauth.ScopeUserReadCurrentlyPlaying,
			spotifyauth.ScopeUserReadPlaybackState),
	)
	ch    = make(chan *spotify.Client)
	state = "abc123"
)

type CurrentlyPlaying spotify.CurrentlyPlaying

func (currentlyPlaying CurrentlyPlaying) getArtistFieldString(field string) []string {
	var data []string
	for _, artist := range currentlyPlaying.Item.SimpleTrack.Artists {
		r := reflect.ValueOf(artist)
		f := reflect.Indirect(r).FieldByName(field)
		data = append(data, f.String())
	}
	return data
}

func main() {
	flag.Parse()
	var (
		client      *spotify.Client
		playerState *spotify.PlayerState
	)

	http.HandleFunc("/callback", completeAuth)

	http.HandleFunc("/current_song/", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		currentlyPlaying, err := client.PlayerCurrentlyPlaying(ctx)
		if err != nil {
			log.Fatal(err)
		}
		var songJson []byte
		if currentlyPlaying.Item != nil {
			var songData = (*CurrentlyPlaying)(currentlyPlaying)
			track := Track{
				Title:   songData.Item.Name,
				Album:   songData.Item.Album.Name,
				Artists: songData.getArtistFieldString("Name"),
			}
			songJson, err = json.Marshal(track)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			songInterface := map[string]interface{}{
				"title":   "Nothing playing",
				"album":   "",
				"artists": []string{""},
			}
			songJson, err = json.Marshal(songInterface)
			if err != nil {
				log.Fatal(err)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err = fmt.Fprint(w, string(songJson))
		if err != nil {
			log.Fatal(err)
		}
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Got request for:", r.URL.String())
	})
	go func() {
		url := auth.AuthURL(state)
		fmt.Println("Please log in to Spotify by visiting the following page in your browser:", url)

		// wait for auth to complete
		client = <-ch

		// use the client to make calls that require authorization
		user, err := client.CurrentUser(context.Background())
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("You are logged in as:", user.ID)

		playerState, err = client.PlayerState(context.Background())
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Found your %s (%s)\n", playerState.Device.Type, playerState.Device.Name)
	}()

	err := http.ListenAndServe(":3000", nil)
	if err != nil {
		log.Fatal(err)
	}
}

func completeAuth(w http.ResponseWriter, r *http.Request) {
	tok, err := auth.Token(r.Context(), state, r)
	if err != nil {
		http.Error(w, "Couldn't get token", http.StatusForbidden)
		log.Fatal(err)
	}

	if st := r.FormValue("state"); st != state {
		http.NotFound(w, r)
		log.Fatalf("state mismatch: %s != %s\n", st, state)
	}

	client := spotify.New(auth.Client(r.Context(), tok))
	w.Header().Set("Content-Type", "text/html")
	_, err = fmt.Fprintf(w, "Login completed ✅")
	if err != nil {
		log.Fatal(err)
	}
	ch <- client
}
