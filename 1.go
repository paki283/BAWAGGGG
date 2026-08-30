package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
//	"io"
	"net/http"
//	"os"
	"regexp"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
)

type AIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AISession struct {
	SenderID string
	Messages []AIMessage
	BotLID   string
}

var aiCache = make(map[string]AISession)

func handleAICommand(client *whatsmeow.Client, v *events.Message, query string, cmd string) {
	if query == "" {
		replyMessage(client, v, "❌ *Error:* Please ask a question.\nExample: `.ai Explain BS CS concepts.`")
		return
	}

	react(client, v.Info.Chat, v.Info.ID, "🧠")

	persona := `You are a silent ai made by Nothing Is Impossible. 
RULES:
1. CASUAL CHAT: If the user says hi/hello or talks casually, be friendly and short.
2. HAPPY MODE: If the user angry you send always smile emoji and happy response.
3. HINDI BLOCK: STRICTLY NO HINDI (Devanagari script). Reply only in Roman Urdu or clean Urdu text.
4. SHORT ANSWER: Always Short Answer Send.
5. MEMORY: Always keep context in mind from previous turns in the conversation.`

	session := AISession{
		SenderID: v.Info.Sender.User,
		BotLID:   getCleanID(client.Store.ID.User),
		Messages: []AIMessage{
			{Role: "system", Content: persona},
			{Role: "user", Content: query},
		},
	}

	go processAndSendAI(client, v, session)
}

func processAndSendAI(client *whatsmeow.Client, v *events.Message, session AISession) {
	react(client, v.Info.Chat, v.Info.ID, "⏳")

	var compiledPrompt strings.Builder
	for _, msg := range session.Messages {
		if msg.Role == "system" {
			compiledPrompt.WriteString(msg.Content + "\n\n")
		} else if msg.Role == "user" {
			compiledPrompt.WriteString("User: " + msg.Content + "\n")
		} else if msg.Role == "assistant" {
			compiledPrompt.WriteString("AI: " + msg.Content + "\n")
		}
	}
	compiledPrompt.WriteString("AI:")

	requestBody := map[string]string{
		"key":    "silent-ai",
		"prompt": compiledPrompt.String(),
	}

	jsonData, _ := json.Marshal(requestBody)
	req, _ := http.NewRequest("POST", "https://silent-ai-pro-phi.vercel.app/api/ask", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: 90 * time.Second}
	resp, err := httpClient.Do(req)

	if err != nil {
		fmt.Printf("❌ [AI ERROR]: %v\n", err)
		replyMessage(client, v, "❌ Network issue.")
		react(client, v.Info.Chat, v.Info.ID, "❌")
		return
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	var rawResponse strings.Builder

	for {
		line, err := reader.ReadString('\n')
		if err != nil { break }
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			jsonStr := strings.TrimPrefix(line, "data: ")
			var dataChunk struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}
			if err := json.Unmarshal([]byte(jsonStr), &dataChunk); err == nil && dataChunk.Type == "text" {
				rawResponse.WriteString(dataChunk.Text)
			}
		}
	}

	aiReplyText := strings.TrimSpace(rawResponse.String())

	aiReplyText = strings.ReplaceAll(aiReplyText, "**", "*")
	reHeaders := regexp.MustCompile(`(?m)^#{1,6}\s+(.*)$`)
	aiReplyText = reHeaders.ReplaceAllString(aiReplyText, "*$1*")

	if aiReplyText != "" {
		msgID := replyMessage(client, v, aiReplyText)
		session.Messages = append(session.Messages, AIMessage{Role: "assistant", Content: aiReplyText})

		if msgID != "" {
			aiCache[msgID] = session
			go func(id string) {
				time.Sleep(1 * time.Hour)
				delete(aiCache, id)
			}(msgID)
		}
		react(client, v.Info.Chat, v.Info.ID, "✅")
	} else {
		replyMessage(client, v, "❌ Got empty response.")
		react(client, v.Info.Chat, v.Info.ID, "❌")
	}
}

func HandleAIChatReply(client *whatsmeow.Client, v *events.Message, bodyClean string, qID string) bool {
	if session, ok := aiCache[qID]; ok {
		if strings.Contains(v.Info.Sender.User, session.SenderID) {

			session.Messages = append(session.Messages, AIMessage{Role: "user", Content: bodyClean})
		
			if len(session.Messages) > 15 {
				session.Messages = append([]AIMessage{session.Messages[0]}, session.Messages[len(session.Messages)-14:]...)
			}
			
			go processAndSendAI(client, v, session)
			return true
		}
	}
	return false
}

func getCleanID(jidStr string) string {
	if jidStr == "" { return "unknown" }
	parts := strings.Split(jidStr, "@")
	if len(parts) == 0 { return "unknown" }
	return strings.TrimSpace(parts[0])
}
