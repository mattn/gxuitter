package main

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"github.com/google/gxui"
	"github.com/google/gxui/drivers/gl"
	"github.com/google/gxui/math"
	"github.com/google/gxui/themes/dark"
	"image"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

type Viewer struct {
	c gxui.Control
}

func (v Viewer) View(t gxui.Theme) gxui.Control {
	return v.c
}

func getImage(cacheDir, u string) image.Image {
	x := fmt.Sprintf("%X", md5.Sum([]byte(u)))
	cacheFile := filepath.Join(cacheDir, x)
	black := MustAsset("data/black.png")

	var img image.Image
	if _, err := os.Stat(cacheFile); err != nil {
		res, err := http.Get(u)
		if err != nil {
			log.Println(err)
			ioutil.WriteFile(cacheFile, black, 0644)
		} else {
			defer res.Body.Close()
			f, err := os.Create(cacheFile)
			if err != nil {
				log.Println(err)
				return nil
			}
			io.Copy(f, res.Body)
			f.Close()
		}
	}
	f, err := os.Open(cacheFile)
	if err == nil {
		defer f.Close()
		img, _, err = image.Decode(f)
	}
	if img == nil {
		img, _, _ = image.Decode(bytes.NewReader(MustAsset("data/black.png")))
	}
	return img
}

func appMain(driver gxui.Driver) {
	/*
		defer func() {
			err := recover()
			if err != nil {
				log.Println(err)
				os.Exit(1)
				return
			}
		}()
	*/

	file, config := getConfig()
	cacheDir := filepath.Join(filepath.Dir(file), "cache")
	if _, err := os.Stat(cacheDir); err != nil {
		os.MkdirAll(cacheDir, 0700)
	}

	token, authorized, err := getAccessToken(config)
	if err != nil {
		log.Fatal("faild to get access token:", err)
	}
	if authorized {
		b, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			log.Fatal("failed to store file:", err)
		}
		err = ioutil.WriteFile(file, b, 0700)
		if err != nil {
			log.Fatal("failed to store file:", err)
		}
	}

	theme := dark.CreateTheme(driver)
	font, err := driver.CreateFont(MustAsset(`data/RictyDiminished-Regular.ttf`), 12)
	if err != nil {
		log.Fatal(err)
	}
	theme.SetDefaultFont(font)

	window := theme.CreateWindow(500, 500, "gxuitter")
	window.SetPadding(math.Spacing{L: 10, T: 10, R: 10, B: 10})
	window.OnClose(driver.Terminate)

	layout := theme.CreateSplitterLayout()
	window.AddChild(layout)

	list := theme.CreateList()
	adapter := gxui.CreateDefaultAdapter()
	list.SetAdapter(adapter)
	layout.AddChild(list)

	row := theme.CreateLinearLayout()
	row.SetOrientation(gxui.Horizontal)
	text := theme.CreateTextBox()
	row.AddChild(text)
	button := theme.CreateButton()
	button.SetText("Update")
	row.AddChild(button)
	layout.AddChild(row)
	layout.SetChildWeight(row, 0.1) // 10% of the full height

	updateTimeline := func() {
		adapter.SetData([]string{})
		if false {
			return
		}
		tweets, err := getTweets(token, HOME_TIMELINE_ENDPOINT, nil)
		if err != nil {
			log.Println(err)
			return
		}
		var items []*Viewer
		adapter.SetData([]string{})

		makeStatus := func(tweet Tweet) func() {
			return func() {
				container := theme.CreateLinearLayout()

				pict := theme.CreateImage()
				texture := driver.CreateTexture(getImage(cacheDir, tweet.User.ProfileImageURL), 96)
				texture.SetFlipY(true)
				pict.SetTexture(texture)
				pict.SetExplicitSize(math.Size{32, 32})
				container.AddChild(pict)

				user := theme.CreateLabel()
				user.SetText(tweet.User.ScreenName)
				container.AddChild(user)

				text := theme.CreateLabel()
				text.SetText(tweet.Text)
				container.AddChild(text)

				items = append(items, &Viewer{container})
				adapter.SetData(items)
				adapter.SetItemSizeAsLargest(theme)
			}
		}
		for _, tweet := range tweets {
			driver.Events() <- makeStatus(tweet)
		}
	}

	driver.Events() <- updateTimeline

	button.OnClick(func(ev gxui.MouseEvent) {
		status := text.Text()
		if status == "" {
			return
		}
		err = postTweet(token, POST_TWEET_ENDPOINT, option{
			"status":                status,
			"in_reply_to_status_id": ""})
		if err != nil {
			log.Println(err)
		} else {
			text.SetText("")
			driver.Events() <- updateTimeline
		}
	})

	gxui.EventLoop(driver)
}

func main() {
	gl.StartDriver("", appMain)
}
