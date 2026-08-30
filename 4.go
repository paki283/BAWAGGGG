package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"bufio"
	"net/url"
	"bytes"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)




type MediaSession struct {
	Results  []SearchResult
	SenderID string
}

type SearchResult struct {
	Title string
	Url   string
}

type YTDownloadState struct {
	Url      string
	SenderID string
}

var ytSearchCache = make(map[string]MediaSession)
var ttSearchCache = make(map[string]MediaSession)
var ytQualityCache = make(map[string]YTDownloadState)




type APIResponse struct {
	Success     bool   `json:"success"`
	Title       string `json:"title"`
	Resolution  string `json:"resolution"`
	DownloadURL string `json:"download_url"`
}


const MaxWhatsAppSize int64 = 1932735283 
const SafeMarginMB = 1800.0







func downloadAndSend(client *whatsmeow.Client, v *events.Message, targetUrl, mode string, optionalFormat ...string) {
	
	
	
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
	

	isAudio := mode == "audio"

	
	fmt.Printf("\n📥 [INTERNAL SCRAPER] Sending raw link: %s\n", targetUrl) 
	
	title, downloadURL, err := extractVidsSaveURL(targetUrl, mode)
	
	if err != nil || downloadURL == "" {
		stopAnim()
		
		fmt.Printf("\n========================================\n")
		fmt.Printf("❌ [EXTRACTION ERROR IN downloadAndSend]\n")
		fmt.Printf("👉 Input URL: %s\n", targetUrl)
		fmt.Printf("👉 Error: %v\n", err)
		fmt.Printf("👉 Result URL: '%s'\n", downloadURL)
		fmt.Printf("========================================\n\n")

		replyMessage(client, v, "❌ *Download Failed:* System could not extract this link.")
		react(client, v.Info.Chat, v.Info.ID, "❌")
		return
	}

	httpClient := &http.Client{Timeout: 5 * time.Minute}

	
	fmt.Printf("🌐 [STREAMING] Downloading from: %s\n", downloadURL)
	fileResp, err := httpClient.Get(downloadURL)
	if err != nil { 
		stopAnim()
		fmt.Printf("❌ [STREAM ERROR]: %v\n", err)
		replyMessage(client, v, "❌ *Error:* Failed to stream media from server.")
		react(client, v.Info.Chat, v.Info.ID, "❌")
		return 
	}
	defer fileResp.Body.Close()

	ext := ".mp4"
	if isAudio { ext = ".m4a" }
	tempFileName := fmt.Sprintf("./data/temp_%d%s", time.Now().UnixNano(), ext)
	
	outFile, err := os.Create(tempFileName)
	if err != nil { stopAnim(); react(client, v.Info.Chat, v.Info.ID, "❌"); return }
	
	_, err = io.Copy(outFile, fileResp.Body)
	outFile.Close()

	if err != nil { 
		os.Remove(tempFileName)
		stopAnim()
		fmt.Printf("❌ [SAVE ERROR]: %v\n", err)
		react(client, v.Info.Chat, v.Info.ID, "❌")
		return 
	}

	defer os.Remove(tempFileName)

	
	fileInfo, err := os.Stat(tempFileName)
	if err != nil { stopAnim(); react(client, v.Info.Chat, v.Info.ID, "❌"); return }
	
	fileSize := fileInfo.Size()
	fmt.Printf("✅ [DOWNLOADED] File Size: %.2f MB\n", float64(fileSize)/(1024*1024))

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
	fmt.Printf("🎉 [COMPLETED] Successfully sent to user.\n")
}





func uploadAndSendFile(client *whatsmeow.Client, v *events.Message, filePath string, title string, isAudio bool, partNum int, totalParts int) {
	fileData, err := os.ReadFile(filePath)
	if err != nil { 
		fmt.Printf("❌ ReadFile failed: %v\n", err)
		return 
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	var mType whatsmeow.MediaType
	var mime string
	if isAudio { 
		mType = whatsmeow.MediaAudio; mime = "audio/mpeg" 
	} else { 
		if len(fileData) > 90*1024*1024 {
			mType = whatsmeow.MediaDocument; mime = "video/mp4"
		} else {
			mType = whatsmeow.MediaVideo; mime = "video/mp4"
		}
	}

	up, err := client.Upload(ctx, fileData, mType)
	if err != nil { 
		fmt.Printf("❌ Upload failed for part %d: %v\n", partNum, err)
		return 
	}

	var msg waProto.Message
	finalTitle := title
	if totalParts > 1 {
		finalTitle = fmt.Sprintf("%s (Part %d/%d)", title, partNum, totalParts)
	}

	if isAudio {
		msg.AudioMessage = &waProto.AudioMessage{
			URL:           proto.String(up.URL), 
			DirectPath:    proto.String(up.DirectPath), 
			MediaKey:      up.MediaKey,
			Mimetype:      proto.String(mime), 
			FileLength:    proto.Uint64(uint64(len(fileData))), 
			PTT:           proto.Bool(false),
			FileSHA256:    up.FileSHA256,       
			FileEncSHA256: up.FileEncSHA256,    
			ContextInfo: &waProto.ContextInfo{
				StanzaID:      proto.String(v.Info.ID),
				Participant:   proto.String(v.Info.Sender.String()),
				QuotedMessage: v.Message,
			},
		}
	} else if mType == whatsmeow.MediaDocument {
		msg.DocumentMessage = &waProto.DocumentMessage{
			URL:           proto.String(up.URL), 
			DirectPath:    proto.String(up.DirectPath), 
			MediaKey:      up.MediaKey,
			Mimetype:      proto.String(mime), 
			Title:         proto.String(finalTitle), 
			FileName:      proto.String(finalTitle + ".mp4"),
			FileLength:    proto.Uint64(uint64(len(fileData))), 
			Caption:       proto.String("✅ " + finalTitle),
			FileSHA256:    up.FileSHA256,       
			FileEncSHA256: up.FileEncSHA256,    
			ContextInfo: &waProto.ContextInfo{
				StanzaID:      proto.String(v.Info.ID),
				Participant:   proto.String(v.Info.Sender.String()),
				QuotedMessage: v.Message,
			},
		}
	} else {
		msg.VideoMessage = &waProto.VideoMessage{
			URL:           proto.String(up.URL), 
			DirectPath:    proto.String(up.DirectPath), 
			MediaKey:      up.MediaKey,
			Mimetype:      proto.String(mime), 
			Caption:       proto.String("✅ " + finalTitle), 
			FileLength:    proto.Uint64(uint64(len(fileData))),
			FileSHA256:    up.FileSHA256,       
			FileEncSHA256: up.FileEncSHA256,    
			ContextInfo: &waProto.ContextInfo{
				StanzaID:      proto.String(v.Info.ID),
				Participant:   proto.String(v.Info.Sender.String()),
				QuotedMessage: v.Message,
			},
		}
	}

	_, err = client.SendMessage(ctx, v.Info.Chat, &msg)
	if err != nil {
		fmt.Printf("❌ SendMessage Error: %v\n", err)
	}
}




func splitVideoSmart(inputPath string, targetMB float64) ([]string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", inputPath)
	out, err := cmd.Output()
	if err != nil { return nil, err }
	
	durationSec, _ := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	
	info, _ := os.Stat(inputPath)
	totalSizeMB := float64(info.Size()) / (1024 * 1024)
	
	chunkDuration := (targetMB / totalSizeMB) * durationSec
	chunkDuration = chunkDuration * 0.95 

	fmt.Printf("✂️ Splitting video. Total: %.2f MB, Target: %.2f MB, Chunk Time: %.0f sec\n", totalSizeMB, targetMB, chunkDuration)

	outputPattern := strings.Replace(inputPath, ".mp4", "_part%03d.mp4", 1)
	
	splitCmd := exec.Command("ffmpeg", 
		"-i", inputPath, 
		"-c", "copy",          
		"-map", "0", 
		"-f", "segment", 
		"-segment_time", fmt.Sprintf("%.0f", chunkDuration), 
		"-reset_timestamps", "1", 
		outputPattern,
	)

	if err := splitCmd.Run(); err != nil {
		return nil, err
	}

	baseName := strings.TrimSuffix(outputPattern, "%03d.mp4")
	files, _ := filepath.Glob(baseName + "*")
	return files, nil
}







func handleYTS(client *whatsmeow.Client, v *events.Message, query string) {
	if query == "" { return }
	react(client, v.Info.Chat, v.Info.ID, "🔍")

	
	cmd := exec.Command("yt-dlp", "ytsearch5:"+query, "--flat-playlist", "--print", "%(title)s|||%(id)s")
	out, err := cmd.CombinedOutput()
	
	if err != nil { 
		
		errMsg := strings.TrimSpace(string(out))
		if len(errMsg) > 500 { errMsg = errMsg[:500] + "..." } 
		
		fmt.Printf("❌ [YTS ERROR]: %v\nOutput: %s\n", err, errMsg)
		replyMessage(client, v, fmt.Sprintf("❌ *YouTube Search Error:*\n```\n%s\n```", errMsg))
		react(client, v.Info.Chat, v.Info.ID, "❌")
		return 
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var results []SearchResult
	
	menuText := "❖ ── ✦ 𝗬𝗢𝗨𝗧𝗨𝗕𝗘 𝗦𝗘𝗔𝗥𝗖𝗛 ✦ ── ❖\n\n"
	icons := []string{"❶", "❷", "❸", "❹", "❺"}
	count := 0
	for _, line := range lines {
		parts := strings.Split(line, "|||")
		if len(parts) < 2 || count >= 5 { continue }
		
		title := strings.TrimSpace(parts[0])
		vidID := strings.TrimSpace(parts[1])
		results = append(results, SearchResult{Title: title, Url: "https://www.youtube.com/watch?v=" + vidID})
		
		menuText += fmt.Sprintf(" %s %s\n\n", icons[count], title)
		count++
	}

	if count == 0 { 
		replyMessage(client, v, "❌ *Error:* No videos found for this search.")
		react(client, v.Info.Chat, v.Info.ID, "❌")
		return 
	}
	menuText += "↬ _Reply with a number (1-5)_"

	msgID := replyMessage(client, v, menuText)
	ytSearchCache[msgID] = MediaSession{Results: results, SenderID: v.Info.Sender.User}
}




func handleVideoSearch(client *whatsmeow.Client, v *events.Message, query string) {
	if query == "" { return }
	react(client, v.Info.Chat, v.Info.ID, "🔍")

	
	cmd := exec.Command("yt-dlp", "ytsearch1:"+query, "--flat-playlist", "--print", "id")
	out, err := cmd.CombinedOutput()
	
	if err != nil {
		errMsg := strings.TrimSpace(string(out))
		if len(errMsg) > 500 { errMsg = errMsg[:500] + "..." }
		
		fmt.Printf("❌ [VIDEO SEARCH ERROR]: %v\nOutput: %s\n", err, errMsg)
		replyMessage(client, v, fmt.Sprintf("❌ *Search Error:*\n```\n%s\n```", errMsg))
		react(client, v.Info.Chat, v.Info.ID, "❌")
		return
	}

	
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		replyMessage(client, v, "❌ *Error:* No video found for this search.")
		react(client, v.Info.Chat, v.Info.ID, "❌")
		return
	}

	vidID := strings.TrimSpace(lines[0])
	ytUrl := "https://www.youtube.com/watch?v=" + vidID
	
	
	go downloadViaAPI(client, v, ytUrl, "360p", false)
}




func handlePlayMusic(client *whatsmeow.Client, v *events.Message, query string) {
	if query == "" { return }
	react(client, v.Info.Chat, v.Info.ID, "🔍")

	
	cmd := exec.Command("yt-dlp", "ytsearch1:"+query, "--flat-playlist", "--print", "id")
	out, err := cmd.CombinedOutput()
	
	if err != nil {
		errMsg := strings.TrimSpace(string(out))
		if len(errMsg) > 500 { errMsg = errMsg[:500] + "..." }
		
		fmt.Printf("❌ [PLAY SEARCH ERROR]: %v\nOutput: %s\n", err, errMsg)
		replyMessage(client, v, fmt.Sprintf("❌ *Search Error:*\n```\n%s\n```", errMsg))
		react(client, v.Info.Chat, v.Info.ID, "❌")
		return
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		replyMessage(client, v, "❌ *Error:* No audio found for this search.")
		react(client, v.Info.Chat, v.Info.ID, "❌")
		return
	}

	vidID := strings.TrimSpace(lines[0])
	ytUrl := "https://www.youtube.com/watch?v=" + vidID

	
	go downloadViaAPI(client, v, ytUrl, "mp3", true)
}




func handleYTDirect(client *whatsmeow.Client, v *events.Message, ytUrl string) {
	if ytUrl == "" { return }
	
	handleYTQualityMenu(client, v, ytUrl)
}




func handleYTQualityMenu(client *whatsmeow.Client, v *events.Message, ytUrl string) {
	menu := `❖ ── ✦ 𝗤𝗨𝗔𝗟𝗜𝗧𝗬 ✦ ── ❖

 ❶  144p  (Low)
 ❷  240p  (Low+)
 ❸  360p  (Normal)
 ❹  480p  (SD)
 ❺  720p  (HD)
 ❻  1080p (FHD)
 ❼  MP3   (Audio)

↬ _Reply with a number (1-7)_`

	msgID := replyMessage(client, v, menu)
	ytQualityCache[msgID] = YTDownloadState{Url: ytUrl, SenderID: v.Info.Sender.User}
}




func HandleMenuReplies(client *whatsmeow.Client, v *events.Message, bodyClean string, qID string) bool {
    if HandleAIChatReply(client, v, bodyClean, qID) {
		return true
	}
	
	
	if session, ok := ytSearchCache[qID]; ok {
		if strings.Contains(v.Info.Sender.User, session.SenderID) {
			delete(ytSearchCache, qID)
			if idx, err := strconv.Atoi(bodyClean); err == nil && idx > 0 && idx <= len(session.Results) {
				handleYTQualityMenu(client, v, session.Results[idx-1].Url)
			}
			return true
		}
	}

	
	if state, ok := ytQualityCache[qID]; ok {
		if strings.Contains(v.Info.Sender.User, state.SenderID) {
			delete(ytQualityCache, qID)
			
			
			resMap := map[string]string{
				"1": "144p",
				"2": "240p",
				"3": "360p",
				"4": "480p",
				"5": "720p",
				"6": "1080p",
				"7": "mp3",
			}
			
			resConfig, exists := resMap[bodyClean]
			if !exists { resConfig = "360p" } 
			
			go downloadViaAPI(client, v, state.Url, resConfig, resConfig == "mp3")
			return true
		}
	}

	
	if session, ok := ttSearchCache[qID]; ok {
		if strings.Contains(v.Info.Sender.User, session.SenderID) {
			delete(ttSearchCache, qID)
			if idx, err := strconv.Atoi(bodyClean); err == nil && idx > 0 && idx <= len(session.Results) {
				go downloadViaAPI(client, v, session.Results[idx-1].Url, "mp4", false)
			}
			return true
		}
	}
	return false
}





func handleTTSearch(client *whatsmeow.Client, v *events.Message, query string) {
	if query == "" { return }
	react(client, v.Info.Chat, v.Info.ID, "🔍")

	
	cmd := exec.Command("python3", "tiktok_search.py", query)
	
	
	out, err := cmd.CombinedOutput()
	if err != nil { 
		fmt.Printf("❌ [GO] Execution Error: %v\n", err)
		react(client, v.Info.Chat, v.Info.ID, "❌") 
		return 
	}

	
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	lastLine := lines[len(lines)-1]

	var results []SearchResult
	
	
	if jsonErr := json.Unmarshal([]byte(lastLine), &results); jsonErr != nil || len(results) == 0 {
		
		if err2 := json.Unmarshal(out, &results); err2 != nil || len(results) == 0 {
			fmt.Printf("❌ [GO] JSON Parse Error: %v\nRaw Output: %s\n", jsonErr, string(out))
			react(client, v.Info.Chat, v.Info.ID, "❌")
			return
		}
	}

	
	menuText := "❖ ── ✦ 𝗧𝗜𝗞𝗧𝗢𝗞 𝗦𝗘𝗔𝗥𝗖𝗛 ✦ ── ❖\n\n"
	icons := []string{"❶", "❷", "❸", "❹", "❺", "❻", "❼", "❽", "❾", "❿"}
	
	limit := len(results)
	if limit > 5 { limit = 5 } 

	for i := 0; i < limit; i++ {
		menuText += fmt.Sprintf(" %s %s\n\n", icons[i], results[i].Title)
	}
	menuText += "↬ _Reply with a number_"

	
	msgID := replyMessage(client, v, menuText)

	
	if msgID != "" {
		ttSearchCache[msgID] = MediaSession{Results: results[:limit], SenderID: v.Info.Sender.User}
	}
}


func handleTikTok(client *whatsmeow.Client, v *events.Message, args string) {
	if args == "" { return }
	args = strings.TrimSpace(args)
	mode, isAudio, urlStr := "mp4", false, args
	
	parts := strings.Fields(args)
	if len(parts) > 1 && (strings.ToLower(parts[0]) == "a" || strings.ToLower(parts[0]) == "audio") {
		mode, isAudio, urlStr = "mp3", true, parts[1]
	}
	go downloadViaAPI(client, v, urlStr, mode, isAudio)
}








func handleUniversalDownload(client *whatsmeow.Client, v *events.Message, url string, cmd string) {
	if url == "" {
		replyMessage(client, v, "❌ *Error:* Please provide a valid link.")
		return
	}

	
	var emoji, mode string
	mode = "video" 

	
	switch cmd {
	case "fb", "facebook":
		emoji = "💙"
	case "ig", "insta", "instagram":
		emoji = "📸"
	case "tw", "x", "twitter":
		emoji = "🐦"
	case "pin", "pinterest":
		emoji = "📌"
	case "snap", "snapchat":
		emoji = "👻"
	case "reddit":
		emoji = "👽"
	case "dm", "dailymotion":
		emoji = "📺"
	case "sc", "soundcloud", "spotify", "apple", "applemusic", "deezer", "tidal", "mixcloud", "napster", "bandcamp":
		emoji = "🎵"
		mode = "audio"
	default:
		emoji = "🚀"
	}

	
	react(client, v.Info.Chat, v.Info.ID, emoji)

	
	go downloadAndSend(client, v, url, mode)
}


func extractVidsSaveURL(videoURL string, mode string) (string, string, error) {
	resolution := "mp4"
	if mode == "audio" {
		resolution = "mp3"
	}

	
	parseData := url.Values{}
	parseData.Set("auth", "20250901majwlqo")
	parseData.Set("domain", "api-ak.vidssave.com")
	parseData.Set("origin", "source")
	parseData.Set("link", videoURL)

	resp, err := http.PostForm("https://api.vidssave.com/api/contentsite_api/media/parse", parseData)
	if err != nil {
		return "", "", fmt.Errorf("parse API failed: %v", err)
	}
	defer resp.Body.Close()

	var parseResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&parseResp); err != nil {
		return "", "", fmt.Errorf("failed to decode parse JSON: %v", err)
	}

	data, ok := parseResp["data"].(map[string]interface{})
	if !ok {
		return "", "", fmt.Errorf("invalid parse response format")
	}
	title, _ := data["title"].(string)
	resources, _ := data["resources"].([]interface{})

	
	var resourceContent string
	for _, res := range resources {
		rMap, ok := res.(map[string]interface{})
		if !ok { continue }
		
		quality, _ := rMap["quality"].(string)
		format, _ := rMap["format"].(string)
		format = strings.ToLower(format)
		
		if resolution == "mp3" && format == "mp3" {
			resourceContent, _ = rMap["resource_content"].(string)
			break
		} else if strings.Contains(quality, resolution) || format == resolution {
			resourceContent, _ = rMap["resource_content"].(string)
			break
		}
	}

	
	if resourceContent == "" && len(resources) > 0 {
		rMap := resources[0].(map[string]interface{})
		resourceContent, _ = rMap["resource_content"].(string)
	}

	if resourceContent == "" {
		return "", "", fmt.Errorf("no suitable resource found")
	}

	
	dlData := url.Values{}
	dlData.Set("auth", "20250901majwlqo")
	dlData.Set("domain", "api-ak.vidssave.com")
	dlData.Set("request", resourceContent)
	dlData.Set("no_encrypt", "1")

	dResp, err := http.PostForm("https://api.vidssave.com/api/contentsite_api/media/download", dlData)
	if err != nil {
		return "", "", fmt.Errorf("download task API failed: %v", err)
	}
	defer dResp.Body.Close()

	var dlResp map[string]interface{}
	json.NewDecoder(dResp.Body).Decode(&dlResp)
	taskData, ok := dlResp["data"].(map[string]interface{})
	if !ok {
		return "", "", fmt.Errorf("task_id not found")
	}
	taskID, _ := taskData["task_id"].(string)

	
	queryURL := fmt.Sprintf("https://api.vidssave.com/sse/contentsite_api/media/download_query?auth=20250901majwlqo&domain=api-ak.vidssave.com&task_id=%s&download_domain=vidssave.com&origin=content_site", url.QueryEscape(taskID))
	
	sseResp, err := http.Get(queryURL)
	if err != nil {
		return "", "", fmt.Errorf("SSE query failed: %v", err)
	}
	defer sseResp.Body.Close()
	
	scanner := bufio.NewScanner(sseResp.Body)
	var downloadLink string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			var eventData map[string]interface{}
			json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &eventData)
			if eventData["status"] == "success" {
				downloadLink, _ = eventData["download_link"].(string)
				break
			}
		}
	}

	if downloadLink == "" {
		return "", "", fmt.Errorf("failed to get download link from SSE")
	}

	
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse 
		},
	}
	redirReq, _ := http.NewRequest("GET", downloadLink, nil)
	redirResp, err := client.Do(redirReq)
	if err != nil {
		return "", "", fmt.Errorf("redirect request failed: %v", err)
	}
	defer redirResp.Body.Close()

	finalURL := redirResp.Header.Get("Location")
	if finalURL == "" {
		return "", "", fmt.Errorf("location header missing")
	}

	return title, finalURL, nil
}

func handleRVC(client *whatsmeow.Client, v *events.Message) {
	
	contextInfo := v.Message.GetExtendedTextMessage().GetContextInfo()
	if contextInfo == nil || contextInfo.GetQuotedMessage() == nil {
		replyMessage(client, v, "❌ *Error:* Please reply to a voice or audio message with the command (e.g., .rvc)")
		return
	}

	quotedMsg := contextInfo.GetQuotedMessage()
	audioMsg := quotedMsg.GetAudioMessage()

	
	if audioMsg == nil {
		replyMessage(client, v, "❌ *Error:* This command only works for audio or voice notes.")
		return
	}

	
	targetJID := v.Info.Chat
	fullText := v.Message.GetExtendedTextMessage().GetText()
	args := strings.Split(strings.TrimSpace(fullText), " ")

	if len(args) > 1 {
		rawTarget := args[1]
		if strings.HasSuffix(rawTarget, "@g.us") {
			parsed, err := types.ParseJID(rawTarget)
			if err == nil {
				targetJID = parsed
			}
		} else {
			cleanNum := ""
			for _, r := range rawTarget {
				if r >= '0' && r <= '9' {
					cleanNum += string(r)
				}
			}
			if cleanNum != "" {
				targetJID = types.NewJID(cleanNum, types.DefaultUserServer)
			}
		}
	}

	
	go func(target types.JID, msg *events.Message) {
		
		react(client, msg.Info.Chat, msg.Info.ID, "⏳")

		
		audioData, err := client.Download(context.Background(), audioMsg)
		if err != nil {
			replyMessage(client, msg, fmt.Sprintf("❌ *Download Error:*\n```\n%v\n```", err))
			return
		}

		
		timestamp := time.Now().UnixNano()
		inOgg := fmt.Sprintf("in_%d.ogg", timestamp)
		inMp3 := fmt.Sprintf("in_%d.mp3", timestamp)
		downloadedMp3 := fmt.Sprintf("dl_%d.mp3", timestamp)
		finalOgg := fmt.Sprintf("final_%d.ogg", timestamp)

		
		defer func() {
			os.Remove(inOgg)
			os.Remove(inMp3)
			os.Remove(downloadedMp3)
			os.Remove(finalOgg)
		}()

		
		os.WriteFile(inOgg, audioData, 0644)

		
		exec.Command("ffmpeg", "-i", inOgg, "-y", inMp3).Run()

		
		cmd := exec.Command("python3", "rvc_engine.py", inMp3)
		var out bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr
		err = cmd.Run()

		if err != nil {
			replyMessage(client, msg, fmt.Sprintf("❌ *API Processing Error:*\n```\n%v\n%s\n```", err, stderr.String()))
			return
		}

		
		output := out.String()
		var finalUrl string
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if strings.Contains(line, "RESULT_URL:") {
				finalUrl = strings.TrimSpace(strings.Replace(line, "RESULT_URL:", "", 1))
				break
			}
		}

		if finalUrl == "" {
			replyMessage(client, msg, "❌ *Error:* Wait For Update.")
			return
		}

		
		resp, err := http.Get(finalUrl)
		if err != nil || resp.StatusCode != 200 {
			replyMessage(client, msg, "❌ *Fetch Error:* Failed to retrieve converted audio.")
			if resp != nil {
				resp.Body.Close()
			}
			return
		}

		dlFile, err := os.Create(downloadedMp3)
		if err != nil {
			replyMessage(client, msg, "❌ *System Error:* Could not create local file.")
			resp.Body.Close()
			return
		}
		io.Copy(dlFile, resp.Body)
		dlFile.Close()
		resp.Body.Close()

		
		exec.Command("ffmpeg", "-i", downloadedMp3, "-c:a", "libopus", "-b:a", "64k", "-vbr", "on", "-compression_level", "10", "-frame_duration", "60", "-y", finalOgg).Run()

		
		finalData, err := os.ReadFile(finalOgg)
		if err != nil {
			replyMessage(client, msg, "❌ *Error:* Failed to read the processed audio file.")
			return
		}

		
		uploaded, err := client.Upload(context.Background(), finalData, whatsmeow.MediaAudio)
		if err != nil {
			replyMessage(client, msg, "❌ *Upload Error:* Failed to upload to WhatsApp.")
			return
		}

		
		ptt := true
		client.SendMessage(context.Background(), target, &waProto.Message{
			AudioMessage: &waProto.AudioMessage{
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      proto.String("audio/ogg; codecs=opus"),
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(finalData))),
				Seconds:       audioMsg.Seconds,
				PTT:           &ptt,
			},
		})

		
		react(client, msg.Info.Chat, msg.Info.ID, "✅")

	}(targetJID, v) 
}
