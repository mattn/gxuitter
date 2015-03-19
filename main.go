package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"github.com/daviddengcn/go-colortext"
	"github.com/garyburd/go-oauth/oauth"
	"github.com/google/gxui"
	"github.com/google/gxui/drivers/gl"
	"github.com/google/gxui/math"
	"github.com/google/gxui/themes/dark"
	"image"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

type Tweet struct {
	Text       string `json:"text"`
	Identifier string `json:"id_str"`
	User       struct {
		ScreenName      string `json:"screen_name"`
		ProfileImageURL string `json:"profile_image_url"`
	} `json:"user"`
}

var oauthClient = oauth.Client{
	TemporaryCredentialRequestURI: "https://api.twitter.com/oauth/request_token",
	ResourceOwnerAuthorizationURI: "https://api.twitter.com/oauth/authenticate",
	TokenRequestURI:               "https://api.twitter.com/oauth/access_token",
}

func clientAuth(requestToken *oauth.Credentials) (*oauth.Credentials, error) {
	cmd := "xdg-open"
	url_ := oauthClient.AuthorizationURL(requestToken, nil)

	args := []string{cmd, url_}
	if runtime.GOOS == "windows" {
		cmd = "rundll32.exe"
		args = []string{cmd, "url.dll,FileProtocolHandler", url_}
	} else if runtime.GOOS == "darwin" {
		cmd = "open"
		args = []string{cmd, url_}
	} else if runtime.GOOS == "plan9" {
		cmd = "plumb"
	}
	ct.ChangeColor(ct.Red, true, ct.None, false)
	fmt.Println("Open this URL and enter PIN.", url_)
	ct.ResetColor()
	cmd, err := exec.LookPath(cmd)
	if err == nil {
		p, err := os.StartProcess(cmd, args, &os.ProcAttr{Dir: "", Files: []*os.File{nil, nil, os.Stderr}})
		if err != nil {
			log.Fatal("failed to start command:", err)
		}
		defer p.Release()
	}

	fmt.Print("PIN: ")
	stdin := bufio.NewReader(os.Stdin)
	b, err := stdin.ReadBytes('\n')
	if err != nil {
		log.Fatal("canceled")
	}

	if b[len(b)-2] == '\r' {
		b = b[0 : len(b)-2]
	} else {
		b = b[0 : len(b)-1]
	}
	accessToken, _, err := oauthClient.RequestToken(http.DefaultClient, requestToken, string(b))
	if err != nil {
		log.Fatal("failed to request token:", err)
	}
	return accessToken, nil
}

func getAccessToken(config map[string]string) (*oauth.Credentials, bool, error) {
	oauthClient.Credentials.Token = config["ClientToken"]
	oauthClient.Credentials.Secret = config["ClientSecret"]

	authorized := false
	var token *oauth.Credentials
	accessToken, foundToken := config["AccessToken"]
	accessSecert, foundSecret := config["AccessSecret"]
	if foundToken && foundSecret {
		token = &oauth.Credentials{accessToken, accessSecert}
	} else {
		requestToken, err := oauthClient.RequestTemporaryCredentials(http.DefaultClient, "", nil)
		if err != nil {
			log.Print("failed to request temporary credentials:", err)
			return nil, false, err
		}
		token, err = clientAuth(requestToken)
		if err != nil {
			log.Print("failed to request temporary credentials:", err)
			return nil, false, err
		}

		config["AccessToken"] = token.Token
		config["AccessSecret"] = token.Secret
		authorized = true
	}
	return token, authorized, nil
}

func getConfig() (string, map[string]string) {
	home := os.Getenv("HOME")
	dir := filepath.Join(home, ".config")
	if runtime.GOOS == "windows" {
		home = os.Getenv("USERPROFILE")
		dir = os.Getenv("APPDATA")
		if dir == "" {
			dir = filepath.Join(home, "Application Data")
		}
	} else if runtime.GOOS == "plan9" {
		home = os.Getenv("home")
		dir = filepath.Join(home, ".config")
	}
	_, err := os.Stat(dir)
	if err != nil {
		if os.Mkdir(dir, 0700) != nil {
			log.Fatal("failed to create directory:", err)
		}
	}
	dir = filepath.Join(dir, "gxuitter")
	_, err = os.Stat(dir)
	if err != nil {
		if os.Mkdir(dir, 0700) != nil {
			log.Fatal("failed to create directory:", err)
		}
	}
	file := filepath.Join(dir, "settings.json")
	config := map[string]string{}

	b, err := ioutil.ReadFile(file)
	if err != nil {
		config["ClientToken"] = "KLxEvRpZAj1LWpnvbSAcjQj4P"
		config["ClientSecret"] = "ZQyhEJy5tPnh2q3mZNyoRSFf53nfJ3dDmd81lR7rODdyZqFZxk"
	} else {
		err = json.Unmarshal(b, &config)
		if err != nil {
			log.Fatal("could not unmarhal settings.json:", err)
		}
	}
	return file, config
}

type option map[string]string

func getTweets(token *oauth.Credentials, url_ string, opt option) ([]Tweet, error) {
	param := make(url.Values)
	for k, v := range opt {
		param.Set(k, v)
	}
	oauthClient.SignParam(token, "GET", url_, param)
	url_ = url_ + "?" + param.Encode()
	res, err := http.Get(url_)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, err
	}
	var tweets []Tweet
	err = json.NewDecoder(res.Body).Decode(&tweets)
	if err != nil {
		return nil, err
	}
	return tweets, nil
}

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
		os.MkdirAll(cacheDir, 0600)
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
	font, err := gl.CreateFont("Ricty-Regular", MustAsset(`data/RictyDiminished-Regular.ttf`), theme.DefaultFont().Size())
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

	button.OnClick(func(ev gxui.MouseEvent) {
		window.Close()
	})

	driver.Events() <- func() {
		adapter.SetData([]string{})
		if false {
			return
		}
		tweets, err := getTweets(token, "https://api.twitter.com/1.1/statuses/home_timeline.json", nil)
		if err != nil {
			log.Fatal(err)
		}
		var items []*Viewer
		adapter.SetData([]string{})
		for _, tweet := range tweets {
			func(tweet Tweet) {
				driver.Events() <- func() {
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
			}(tweet)
		}
	}

	gxui.EventLoop(driver)
}

func main() {
	gl.StartDriver("./data", appMain)
}
