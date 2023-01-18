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

const imageFile = "data/img.png"
const imageURL = "https://oblenergo.cv.ua/shutdowns/GPV.png"
const imageSyncInterval = 5 * time.Minute

var imageSHA = ""

var store = NewBoltDBStore("data/app.db")

var mx sync.Mutex

func main() {
	pref := tele.Settings{
		Token:  os.Getenv("TOKEN"),
		Poller: &tele.LongPoller{Timeout: 5 & time.Second},
	}

	if pref.Token == "" {
		log.Fatal("TOKEN environment variable is missing")
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
		defer mx.Unlock()
		if store.Size() >= 10 {
			return c.Send("Too many subscribers. Please contact administrator")
		}
		if store.AddSubscriber(c) {
			chat := c.Chat()
			s := c.Sender()
			log.Printf("New subscriber: chat=\"%s %s %d\", byUser=\"%s %s\"",
				chat.FirstName, chat.LastName, chat.ID, s.FirstName, s.LastName)
			return c.Send("Subscribed!")
		}
		return c.Send("You are already subscribed")
	})

	go refreshImageTask()
	go syncAndSendImageTask(b)

	log.Println("Starting app")
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

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download image: status=%d", resp.StatusCode)
	}

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
	originalSHA := imageSHA
	imageSHA = hex.EncodeToString(hasher.Sum(nil))

	out, err := os.Create(imageFile)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, &buf)

	if originalSHA != imageSHA {
		log.Printf("image updated: %s", imageSHA)
	}
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
	if imageSHA == "" {
		return
	}

	for _, id := range store.GetWithDifferentHash(imageSHA) {
		f := &tele.Photo{File: tele.FromDisk(imageFile)}
		if _, err := b.Send(id, f); err != nil {
			log.Printf("failed to send image to %d: %v", id, err)
			continue
		}
		store.UpdateHash(id, imageSHA)
	}
}
