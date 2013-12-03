package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	baseUrl    = "https://feedwrangler.net/api/v2"
	authUrl    = baseUrl + "/users/authorize"
	logoutUrl  = baseUrl + "/users/logout"
	listIdsUrl = baseUrl + "/feed_items/list_ids"
	deleteUrl  = baseUrl + "/subscriptions/remove_feed"
)

var (
	email           = ""
	password        = ""
	clientKey       = ""
	deleteAge int64 = 0
	token           = ""
	commit          = false
	since     int64 = 0
)

func init() {
	http.DefaultTransport.(*http.Transport).ResponseHeaderTimeout = time.Second * 15
}

func eCheck(msg string, err error) {
	if err != nil {
		Logout()
		log.Panicf("Error caught %s -- %s", msg, err)
	}
}

type Feed struct {
	Title   string `json:"title"`
	FeedId  int    `json:"feed_id"`
	FeedURL string `json:"feed_url"`
	SiteURL string `json:"site_url"`
}

func (f *Feed) Check() bool {
	resp, err := http.Get(f.FeedURL)
	if err != nil {
		log.Printf("Error resolving %q -- %s", f.FeedURL, err)
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Printf("Http error %d resolving %s", resp.StatusCode, f.FeedURL)
		return false
	}
	return true
}

type Response struct {
	Token  string `json:"access_token"`
	Error  string `json:"error"`
	Result string `json:"result"`
	Feeds  []Feed `json:"feeds"`
	Count  int    `json:"count"`
}

func (r *Response) String() string {
	return fmt.Sprintf("Response{%s -- %s}", r.Result, r.Error)

}

func (r *Response) Success() bool {
	return r.Result == "success"
}

func (r *Response) Check() {
	if !r.Success() {
		log.Panicf("FeedWrangler error in response %v", r)
	}
}

func main() {
	flag.StringVar(&email, "email", "", "Required, FeedWrangler account email address")
	flag.StringVar(&password, "password", "", "Required, FeedWrangler account password")
	flag.StringVar(&clientKey, "client", "", "Required, FeedWrangler client key from https://feedwrangler.net/developers/clients")
	flag.Int64Var(&deleteAge, "deleteAge", 300, "Optional, Delete feeds not updated since X days")
	flag.BoolVar(&commit, "commit", false, "Optional, Commit feed deletes to FeedWrangler")
	flag.Parse()
	email = strings.TrimSpace(email)
	if len(email) == 0 {
		flag.Usage()
		log.Panicf("Email argument is required")
	}
	password = strings.TrimSpace(password)
	if len(password) == 0 {
		flag.Usage()
		log.Panicf("Password argument is required")
	}
	clientKey = strings.TrimSpace(clientKey)
	if len(clientKey) == 0 {
		flag.Usage()
		log.Panicf("Client argument is required")
	}
	since = time.Now().Unix() - (24 * 60 * 60 * deleteAge)
	if !commit {
		log.Printf("**** DRY RUN MODE -- NO CHANGES WILL BE MADE ****")
	}
	del := make([]Feed, 0, 100)
	feeds := Authorize()
	log.Printf("Checking for stale feeds...")
	for _, feed := range feeds {
		if keep := CheckFeed(&feed); keep {
			if found := feed.Check(); !found {
				del = append(del, feed)
			}
		} else {
			del = append(del, feed)
		}
	}
	log.Printf("%d stale feeds found", len(del))
	if commit {
		log.Printf("Deleting %d stale feeds...", len(del))
		for _, feed := range del {
			DeleteFeed(&feed)
		}
	}
	Logout()
	if !commit {
		log.Printf("**** DRY RUN MODE -- NO CHANGES WERE MADE ****")
	}
}

func Authorize() []Feed {
	v := url.Values{}
	v.Set("email", email)
	v.Set("password", password)
	v.Set("client_key", clientKey)
	url := fmt.Sprintf("%s?%s", authUrl, v.Encode())
	log.Printf("Feedwrangler Authorize")
	resp, err := http.Get(url)
	eCheck("Error authorize http get", err)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Panicf("Http error %d in users/authorize", resp.StatusCode)
	}
	respBody, err := ioutil.ReadAll(resp.Body)
	eCheck("Error reading authorize response body", err)
	var authResp Response
	err = json.Unmarshal(respBody, &authResp)
	eCheck("Error unmarshalling authorize json response", err)
	authResp.Check()
	log.Printf("FeedWrangler Authorize Success, %d feeds", len(authResp.Feeds))
	token = authResp.Token
	return authResp.Feeds
}

func CheckFeed(feed *Feed) bool {
	v := url.Values{}
	v.Set("access_token", token)
	v.Set("feed_id", fmt.Sprintf("%d", feed.FeedId))
	v.Set("created_since", fmt.Sprintf("%d", since))
	url := fmt.Sprintf("%s?%s", listIdsUrl, v.Encode())
	resp, err := http.Get(url)
	eCheck("Error http list ids", err)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Panicf("Http error %d in list ids", resp.StatusCode)
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	eCheck("Error reading list ids response body", err)
	var idResp Response
	err = json.Unmarshal(respBody, &idResp)
	eCheck("Error unmarshalling list ids json response", err)
	idResp.Check()
	if idResp.Count == 0 {
		log.Printf("STALE %s", feed.Title)
		return false
	}
	return true
}

func DeleteFeed(feed *Feed) {
	log.Printf("DELETE %s", feed.Title)
	v := url.Values{}
	v.Set("access_token", token)
	v.Set("feed_id", fmt.Sprintf("%d", feed.FeedId))
	url := fmt.Sprintf("%s?%s", deleteUrl, v.Encode())
	resp, err := http.Get(url)
	eCheck("Error delete http", err)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Panicf("Http error %d in delete", resp.StatusCode)
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	eCheck("Error reading delete response body", err)
	var delResp Response
	err = json.Unmarshal(respBody, &delResp)
	eCheck("Error unmarshalling delete json response", err)
	delResp.Check()
}

func Logout() {
	if len(token) < 5 {
		return
	}
	log.Printf("Feedwrangler Logout")
	v := url.Values{}
	v.Set("access_token", token)
	url := fmt.Sprintf("%s?%s", logoutUrl, v.Encode())
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Logout http get error %s", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Panicf("Logout http error %d", resp.StatusCode)
	}
}
