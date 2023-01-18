package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	tele "gopkg.in/telebot.v3"
)

const imageFile = "img.png"
const imageURL = "https://oblenergo.cv.ua/shutdowns/GPV.png"
const imageSyncInterval = 5 * time.Minute

var latestSentImageSHA = ""
var currentImageSHA = ""

var subscribers = make(map[tele.ChatID]bool)

var mx sync.Mutex

func main() {
	pref := tele.Settings{
		Token:  os.Getenv("TOKEN"),
		Poller: &tele.LongPoller{Timeout: 5 & time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		log.Fatal(err)
		return
	}

	subscribeMarkup := &tele.ReplyMarkup{}
	subscribeBtn := subscribeMarkup.Data("Subscribe", "subscribe")
	subscribeMarkup.Inline(subscribeMarkup.Row(subscribeBtn))

	b.Handle("/start", func(c tele.Context) error {
		return c.Send("Hello! Do you want to subscribe?", subscribeMarkup)
	})

	b.Handle(&subscribeBtn, func(c tele.Context) error {
		mx.Lock()
		subscribers[tele.ChatID(c.Chat().ID)] = true
		mx.Unlock()
		return c.Send("Subscribed!")
	})

	go refreshImageTask()
	go syncAndSendImageTask(b)

	b.Start()
}

func refreshImageTask() {
	for true {
		if err := refreshImage(); err != nil {
			log.Printf("failed to refresh image: %v", err)
		}
		time.Sleep(imageSyncInterval)
	}
}

func refreshImage() error {
	mx.Lock()
	defer mx.Unlock()

	resp, err := http.Get(imageURL)
	if err != nil {
		return fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to copy image to file: %w", err)
	}

	hasher := sha256.New()
	hasher.Write(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to calculate hash: %w", err)
	}
	hash := hex.EncodeToString(hasher.Sum(nil))

	if currentImageSHA == hash {
		return nil
	}

	out, err := os.Create(imageFile)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, &buf)
	currentImageSHA = hash

	log.Printf("image updated: %s", hash)
	return nil
}

func syncAndSendImageTask(b *tele.Bot) {
	for true {
		sendImageIfUpdated(b)
		time.Sleep(5 * time.Second)
	}
}

func sendImageIfUpdated(b *tele.Bot) {
	mx.Lock()
	defer mx.Unlock()
	if currentImageSHA == latestSentImageSHA || len(subscribers) == 0 {
		return
	}

	for id := range subscribers {
		f := &tele.Photo{File: tele.FromDisk(imageFile)}
		if _, err := b.Send(id, f); err != nil {
			log.Printf("failed to send image to %d: %v", id, err)
		}
	}
	latestSentImageSHA = currentImageSHA
}
