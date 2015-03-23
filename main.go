package main

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"github.com/garyburd/go-oauth/oauth"
	"github.com/google/gxui"
	"github.com/google/gxui/drivers/gl"
	"github.com/google/gxui/math"
	"github.com/google/gxui/themes/dark"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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

	var img image.Image
	if _, err := os.Stat(cacheFile); err != nil {
		res, err := http.Get(u)
		if err != nil {
			log.Println(u, err)
		} else {
			defer res.Body.Close()
			if res.Header.Get("Content-Type") == "image/jpeg" {
				img, err = jpeg.Decode(res.Body)
			} else {
				img, _, err = image.Decode(res.Body)
			}
			if err == nil {
				rect := image.Rect(0, 0, 32, 32)
				tmp := image.NewRGBA(rect)
				draw.Draw(tmp, rect, img, image.Point{0, 0}, draw.Src)
				img = tmp
				f, err := os.Create(cacheFile)
				if err == nil {
					defer f.Close()
					png.Encode(f, img)
				}
			} else {
				log.Println(u, err)
			}
		}
	} else {
		f, err := os.Open(cacheFile)
		if err == nil {
			defer f.Close()
			img, _, _ = image.Decode(f)
		} else {
			log.Println(u, err)
		}
	}
	if img == nil {
		img, _, _ = image.Decode(bytes.NewReader(MustAsset("data/black.png")))
	}
	return img
}

type gxuitter struct {
	file     string
	config   map[string]string
	cacheDir string
	token    *oauth.Credentials
}

func (g *gxuitter) ConfigString(k string) string {
	return g.config[k]
}

func (g *gxuitter) ConfigInt(k string) int {
	i, _ := strconv.Atoi(g.config[k])
	return i
}

func (g *gxuitter) LoadConfig() {
	g.file, g.config = getConfig()
	g.cacheDir = filepath.Join(filepath.Dir(g.file), "cache")
	if _, err := os.Stat(g.cacheDir); err != nil {
		os.MkdirAll(g.cacheDir, 0700)
	}

	var err error
	var authorlized bool
	g.token, authorlized, err = getAccessToken(g.config)
	if err != nil {
		log.Fatal("faild to get access token:", err)
	}
	if authorlized {
		b, err := json.MarshalIndent(g.config, "", "  ")
		if err != nil {
			log.Fatal("failed to store file:", err)
		}
		err = ioutil.WriteFile(g.file, b, 0700)
		if err != nil {
			log.Fatal("failed to store file:", err)
		}
		log.Println(`if you don't see the tweets in broken fonts, set "FontFile" to the path of truetype font file in`, g.file)
	}
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
	g := new(gxuitter)
	g.LoadConfig()

	theme := dark.CreateTheme(driver)

	fontFile := g.ConfigString("FontFile")
	fontSize := g.ConfigInt("FontSize")
	if fontSize <= 0 {
		fontSize = 12
	}
	if fontFile != "" {
		b, err := ioutil.ReadFile(fontFile)
		font, err := driver.CreateFont(b, fontSize)
		if err == nil {
			theme.SetDefaultFont(font)
		}
	}

	window := theme.CreateWindow(500, 500, "gxuitter")
	window.SetPadding(math.Spacing{L: 10, T: 10, R: 10, B: 10})
	window.OnClose(driver.Terminate)

	layout := theme.CreateSplitterLayout()
	window.AddChild(layout)

	list := theme.CreateList()
	adapter := gxui.CreateDefaultAdapter()
	list.SetAdapter(adapter)
	layout.AddChild(list)

	row := theme.CreateSplitterLayout()
	row.SetOrientation(gxui.Horizontal)
	text := theme.CreateTextBox()
	row.AddChild(text)
	button := theme.CreateButton()
	button.SetText("Update")
	button.SetPadding(math.Spacing{L: 10, T: 10, R: 10, B: 10})
	row.AddChild(button)
	row.SetChildWeight(button, 0.2) // 20% of the full width
	layout.AddChild(row)
	layout.SetChildWeight(row, 0.1) // 10% of the full height

	updateTimeline := func() {
		tweets, err := getTweets(g.token, HOME_TIMELINE_ENDPOINT, nil)
		if err != nil {
			log.Println(err)
			return
		}
		var items []*Viewer
		adapter.SetItems([]string{})

		makeStatus := func(tweet Tweet) func() {
			return func() {
				container := theme.CreateLinearLayout()

				pict := theme.CreateImage()
				texture := driver.CreateTexture(getImage(g.cacheDir, tweet.User.ProfileImageURL), 96)
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
				adapter.SetItems(items)
				adapter.SetSizeAsLargest(theme)
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
		err := postTweet(g.token, POST_TWEET_ENDPOINT, option{
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
	gl.StartDriver(appMain)
}
