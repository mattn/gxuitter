package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/daviddengcn/go-colortext"
	"github.com/garyburd/go-oauth/oauth"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const (
	POST_TWEET_ENDPOINT    = "https://api.twitter.com/1.1/statuses/update.json"
	HOME_TIMELINE_ENDPOINT = "https://api.twitter.com/1.1/statuses/home_timeline.json"
	USER_TIMELINE_ENDPOINT = "https://api.twitter.com/1.1/statuses/user_timeline.json"
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
		return nil, errors.New(res.Status)
	}
	var tweets []Tweet
	err = json.NewDecoder(res.Body).Decode(&tweets)
	if err != nil {
		return nil, err
	}
	return tweets, nil
}

func postTweet(token *oauth.Credentials, url_ string, opt option) error {
	param := make(url.Values)
	for k, v := range opt {
		param.Set(k, v)
	}
	oauthClient.SignParam(token, "POST", url_, param)
	res, err := http.PostForm(url_, url.Values(param))
	if err != nil {
		log.Println("failed to post tweet:", err)
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Println("failed to get timeline:", err)
		return err
	}
	return nil
}
