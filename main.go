package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
	"github.com/onrik/logrus/filename"
	"github.com/pkg/errors"
	"github.com/rs/cors"
	log "github.com/sirupsen/logrus"
)

func worker(handle string, client *twitter.Client, wg *sync.WaitGroup, gifs chan string) {
	defer wg.Done()

	tweets, _, err := client.Timelines.UserTimeline(&twitter.UserTimelineParams{
		ScreenName: handle,
		Count:      500,
	})

	if err != nil {
		errors.Wrap(err, "err requesting timeline")
		log.Errorf("%v", err)
		return
	}

	for _, t := range tweets {
		if t.ExtendedEntities != nil {
			for _, me := range t.ExtendedEntities.Media {

				if (me.Type == "animated_gif") &&
					(len(me.VideoInfo.Variants) > 0) {

					gifs <- me.VideoInfo.Variants[0].URL
				}
			}
		}
	}
}

func main() {

	port := os.Getenv("PORT")
	consumerKey := os.Getenv("CONSUMER_KEY")
	consumerSecret := os.Getenv("CONSUMER_SECRET")
	accessToken := os.Getenv("ACCESS_TOKEN")
	accessTokenSecret := os.Getenv("ACCESS_TOKEN_SECRET")
	loglvl := os.Getenv("LOG_LEVEL")

	logLevel, err := log.ParseLevel(loglvl)
	if err != nil {
		log.Fatal(err)
	}
	log.SetLevel(logLevel)
	log.AddHook(filename.NewHook())

	config := oauth1.NewConfig(consumerKey, consumerSecret)
	token := oauth1.NewToken(accessToken, accessTokenSecret)
	httpClient := config.Client(oauth1.NoContext, token)

	client := twitter.NewClient(httpClient)

	mux := http.NewServeMux()

	mux.HandleFunc("/gifs.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if _, err := os.Stat("/tmp/gifs.json"); err == nil {
			f, err := os.Open("/tmp/gifs.json")
			if err != nil {
				errors.Wrap(err, "err opening cache")
				log.Errorf("%v", err)
				return
			}
			_, err = io.Copy(w, f)
			if err != nil {
				errors.Wrap(err, "err copying from cache to response writer")
				log.Errorf("%v", err)
				return
			}
			return
		}

		handles := []string{"etiennejcb", "KangarooPhysics", "jn3008", "satoshi_aizawa"}

		gifchan := make(chan string)
		var wg sync.WaitGroup

		for _, handle := range handles {
			wg.Add(1)
			go worker(handle, client, &wg, gifchan)
		}

		var gifs []string
		go func() {
			for g := range gifchan {
				gifs = append(gifs, g)
			}
		}()

		wg.Wait()
		close(gifchan)

		json.NewEncoder(w).Encode(gifs)

		// cache
		f, err := os.Create("/tmp/gifs.json")
		if err != nil {
			errors.Wrap(err, "err opening cache")
			log.Errorf("%v", err)
			return
		}
		json.NewEncoder(f).Encode(gifs)
	})

	// clear cache
	ticker := time.NewTicker(6 * time.Hour)
	go func() {
		for range ticker.C {
			os.Remove("/tmp/gifs.json")
		}
	}()

	handler := cors.Default().Handler(mux)

	log.Debugf("listening for http on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}
