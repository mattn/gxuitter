package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/daviddengcn/go-colortext"
	"github.com/garyburd/go-oauth/oauth"
	"github.com/google/gxui"
	"github.com/google/gxui/drivers/gl"
	//"github.com/google/gxui/math"
	"github.com/google/gxui/themes/dark"
	//"image"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
)

type Driver struct {
	gxui.Driver
}

func (d Driver) CreateFont(name string, size int) gxui.Font {
	log.Println(name + " なんてロードしねぇよ(ﾊﾞｰｶﾊﾞｰｶ")
	return gl.CreateFont("Ricty-Regular", MustAsset(`data/Ricty-Regular.ttf`), size)
}

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
		config["ClientToken"] = ""  // TODO
		config["ClientSecret"] = "" // TODO
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

func appMain(driver gxui.Driver) {
	defer func() {
		err := recover()
		if err != nil {
			log.Println(err)
			os.Exit(1)
			return
		}
	}()

	file, config := getConfig()
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

	theme := dark.CreateTheme(Driver{driver})
	layout := theme.CreateLinearLayout()
	layout.SetHorizontalAlignment(gxui.AlignCenter)

	list := theme.CreateList()
	adapter := gxui.CreateDefaultAdapter()
	adapter.SetData([]string{"foo", "bar"})
	list.SetAdapter(adapter)
	layout.AddChild(list)

	go func() {
		adapter.SetData([]string{})
		tweets, err := getTweets(token, "https://api.twitter.com/1.1/statuses/home_timeline.json", nil)
		if err != nil {
			log.Fatal(err)
		}
		var wg sync.WaitGroup
		var mutex sync.Mutex
		var items []Viewer
		for _, tweet := range tweets {
			wg.Add(1)
			go func() {
				defer wg.Done()
				println(tweet.User.ProfileImageURL)
				res, err := http.Get(tweet.User.ProfileImageURL)
				if err != nil {
					log.Println(err)
					return
				}
				defer res.Body.Close()
				container := theme.CreateLinearLayout()
				container.SetVerticalAlignment(gxui.AlignMiddle)
				/*
					img, _, err := image.Decode(res.Body)
					if err != nil {
						log.Println(err)
						return
					}
					TODO: Doesn't work correctly
					pict := theme.CreateImage()
					pict.SetTexture(driver.CreateTexture(img, 96))
					pict.SetExplicitSize(math.Size{32, 32})
					container.AddChild(pict)
				*/
				/*
					TODO: Doesn't work correctly
					user := theme.CreateLabel()
					user.SetText(tweet.User.ScreenName)
					container.AddChild(user)

					println(tweet.Text)
					text := theme.CreateLabel()
					text.SetText(tweet.Text)
					container.AddChild(text)
				*/
				label := theme.CreateLabel()
				label.SetText(fmt.Sprintf("%s: %s", tweet.User.ScreenName, tweet.Text))
				container.AddChild(label)

				mutex.Lock()
				defer mutex.Unlock()
				items = append(items, Viewer{container})
			}()
			wg.Wait()
		}
		adapter.SetData(items)
	}()

	text := theme.CreateTextBox()
	layout.AddChild(text)

	button := theme.CreateButton()
	button.SetText("Update")
	layout.AddChild(button)

	window := theme.CreateWindow(500, 200, "gxuitter")
	window.AddChild(layout)
	window.OnClose(driver.Terminate)

	button.OnClick(func(ev gxui.MouseEvent) {
		window.Close()
	})

	gxui.EventLoop(driver)
}

func main() {
	gl.StartDriver("", appMain)
}
