package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
)

type TikWMResponse struct {
	Code int `json:"code"`
	Data struct {
		Title string `json:"title"`
		Play  string `json:"play"`
		Music string `json:"music"`
	} `json:"data"`
}

type YTMediaItem struct {
	MediaURL       string `json:"mediaUrl"`
	Type           string `json:"type"`
	MediaExtension string `json:"mediaExtension"`
}

type YTFirstResponse struct {
	API struct {
		Title      string        `json:"title"`
		MediaItems []YTMediaItem `json:"mediaItems"`
	} `json:"api"`
}

type YTSecondResponse struct {
	API struct {
		Status  string `json:"status"`
		FileURL string `json:"fileUrl"`
	} `json:"api"`
}

type YTAudioItem struct {
	URL string
	Res string
}

type YTVideoItem struct {
	URL string
	Res string
}

func getDownloadLinkDirect(targetUrl, resolution string) (string, string, error) {
	reqRes := strings.ToLower(resolution)
	var targetRes string
	if reqRes == "mp3" || reqRes == "mp4" {
		targetRes = reqRes
	} else {
		if strings.HasSuffix(reqRes, "p") {
			targetRes = reqRes
		} else {
			targetRes = reqRes + "p"
		}
	}

	if strings.Contains(strings.ToLower(targetUrl), "tiktok.com") {
		resp, err := http.Get("https://www.tikwm.com/api/?url=" + url.QueryEscape(targetUrl))
		if err != nil {
			return "", "", err
		}
		defer resp.Body.Close()

		var tikRes TikWMResponse
		if err := json.NewDecoder(resp.Body).Decode(&tikRes); err != nil {
			return "", "", err
		}

		if tikRes.Code != 0 {
			return "", "", errors.New("tiktok api error")
		}

		title := tikRes.Data.Title
		if title == "" {
			title = "TikTok Video"
		}

		if targetRes == "mp3" {
			return tikRes.Data.Music, title, nil
		}
		return tikRes.Data.Play, title, nil

	} else if strings.Contains(strings.ToLower(targetUrl), "youtu") {
		jar, _ := cookiejar.New(nil)
		client := &http.Client{
			Timeout: 30 * time.Second,
			Jar:     jar,
		}

		req, err := http.NewRequest("GET", "https://app.ytdown.to/en23/", nil)
		if err != nil {
			return "", "", err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Mobile Safari/537.36")
		
		resp, err := client.Do(req)
		if err != nil {
			return "", "", err
		}
		resp.Body.Close()

		form := url.Values{}
		form.Set("url", targetUrl)

		req2, err := http.NewRequest("POST", "https://app.ytdown.to/proxy.php", strings.NewReader(form.Encode()))
		if err != nil {
			return "", "", err
		}
		req2.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Mobile Safari/537.36")
		req2.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
		req2.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
		req2.Header.Set("X-Requested-With", "XMLHttpRequest")
		req2.Header.Set("Origin", "https://app.ytdown.to")
		req2.Header.Set("Referer", "https://app.ytdown.to/en23/")

		resp2, err := client.Do(req2)
		if err != nil {
			return "", "", err
		}
		defer resp2.Body.Close()

		var ytFirst YTFirstResponse
		if err := json.NewDecoder(resp2.Body).Decode(&ytFirst); err != nil {
			return "", "", err
		}

		var mediaURL string
		var availableVideos []YTVideoItem
		var availableAudios []YTAudioItem

		for _, item := range ytFirst.API.MediaItems {
			itemURL := strings.ToLower(item.MediaURL)
			itemExt := strings.ToLower(item.MediaExtension)

			if item.Type == "Audio" {
				resStr := "m4a"
				if strings.Contains(itemExt, "mp3") {
					resStr = "mp3"
				}
				availableAudios = append(availableAudios, YTAudioItem{URL: item.MediaURL, Res: resStr})
				if targetRes == "mp3" && strings.Contains(itemExt, "mp3") {
					mediaURL = item.MediaURL
					break
				}
			} else if item.Type == "Video" {
				parts := strings.Split(strings.TrimRight(itemURL, "/"), "/")
				actualRes := ""
				if len(parts) > 0 {
					actualRes = parts[len(parts)-1]
				}
				availableVideos = append(availableVideos, YTVideoItem{URL: item.MediaURL, Res: actualRes})
				if targetRes != "mp3" && strings.Contains(itemURL, targetRes) {
					mediaURL = item.MediaURL
					break
				}
			}
		}

		if mediaURL == "" {
			if targetRes == "mp3" {
				if len(availableAudios) > 0 {
					mediaURL = availableAudios[len(availableAudios)-1].URL
				} else {
					return "", "", errors.New("no audio found")
				}
			} else {
				if len(availableVideos) > 0 {
					mediaURL = availableVideos[0].URL
				} else {
					return "", "", errors.New("no video found")
				}
			}
		}

		form2 := url.Values{}
		form2.Set("url", mediaURL)

		for attempt := 0; attempt < 3; attempt++ {
			req3, err := http.NewRequest("POST", "https://app.ytdown.to/proxy.php", strings.NewReader(form2.Encode()))
			if err != nil {
				return "", "", err
			}
			req3.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Mobile Safari/537.36")
			req3.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
			req3.Header.Set("X-Requested-With", "XMLHttpRequest")
			req3.Header.Set("Origin", "https://app.ytdown.to")
			req3.Header.Set("Referer", "https://app.ytdown.to/en23/")

			resp3, err := client.Do(req3)
			if err != nil {
				return "", "", err
			}

			var ytSecond YTSecondResponse
			if err := json.NewDecoder(resp3.Body).Decode(&ytSecond); err != nil {
				resp3.Body.Close()
				return "", "", err
			}
			resp3.Body.Close()

			if ytSecond.API.Status == "completed" && ytSecond.API.FileURL != "" {
				return ytSecond.API.FileURL, ytFirst.API.Title, nil
			} else if ytSecond.API.Status == "processing" || ytSecond.API.Status == "started" || ytSecond.API.Status == "queued" {
				time.Sleep(5 * time.Second)
				continue
			} else {
				break
			}
		}
		return "", "", errors.New("youtube processing timeout")
	}

	return "", "", errors.New("unsupported url")
}

func downloadViaAPI(client *whatsmeow.Client, v *events.Message, targetUrl, resolution string, isAudio bool) {
	doneAnim := make(chan bool)
	animating := true

	stopAnim := func() {
		if animating {
			close(doneAnim)
			animating = false
		}
	}
	defer stopAnim()

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		emojis := []string{"⏳", "⏬", "📥", "🔄", "⬇️"}
		i := 0
		for {
			select {
			case <-doneAnim:
				return
			case <-ticker.C:
				react(client, v.Info.Chat, v.Info.ID, emojis[i%len(emojis)])
				i++
			}
		}
	}()

	executeFallback := func() {
		stopAnim()
		mode := "video"
		if isAudio {
			mode = "audio"
		}
		downloadAndSend(client, v, targetUrl, mode)
	}

	downloadURL, title, err := getDownloadLinkDirect(targetUrl, resolution)
	if err != nil || downloadURL == "" {
		fmt.Printf("Error: %v\n", err)
		executeFallback()
		return
	}

	httpClient := &http.Client{Timeout: 5 * time.Minute}
	fileResp, err := httpClient.Get(downloadURL)
	if err != nil {
		executeFallback()
		return
	}
	defer fileResp.Body.Close()

	ext := ".mp4"
	if isAudio {
		ext = ".mp3"
	}
	tempFileName := fmt.Sprintf("./data/temp_%d%s", time.Now().UnixNano(), ext)

	outFile, err := os.Create(tempFileName)
	if err != nil {
		executeFallback()
		return
	}

	_, err = io.Copy(outFile, fileResp.Body)
	outFile.Close()
	if err != nil {
		os.Remove(tempFileName)
		executeFallback()
		return
	}

	defer os.Remove(tempFileName)

	fileInfo, err := os.Stat(tempFileName)
	if err != nil {
		executeFallback()
		return
	}

	fileSize := fileInfo.Size()
	var filesToSend []string

	stopAnim()

	if fileSize > int64(MaxWhatsAppSize) && !isAudio {
		react(client, v.Info.Chat, v.Info.ID, "✂️")
		parts, err := splitVideoSmart(tempFileName, SafeMarginMB)
		if err != nil || len(parts) == 0 {
			filesToSend = append(filesToSend, tempFileName)
		} else {
			filesToSend = parts
		}
	} else {
		filesToSend = append(filesToSend, tempFileName)
	}

	react(client, v.Info.Chat, v.Info.ID, "📤")

	for i, filePath := range filesToSend {
		uploadAndSendFile(client, v, filePath, title, isAudio, i+1, len(filesToSend))
		if filePath != tempFileName {
			os.Remove(filePath)
		}
	}

	react(client, v.Info.Chat, v.Info.ID, "✅")
}
