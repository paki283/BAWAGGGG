package main

import (
	"context"
	"fmt"

	"strings"
	"math/rand"
	"time"



	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
	waLog "go.mau.fi/whatsmeow/util/log"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/proto/waCommon"

)




func EventHandler(client *whatsmeow.Client, evt interface{}) {
	defer func() {
		if r := recover(); r != nil {
			botID := "unknown"
			if client != nil && client.Store != nil && client.Store.ID != nil {
				botID = getCleanID(client.Store.ID.User)
			}
			fmt.Printf("⚠️ [CRASH PREVENTED in EventHandler] Bot %s error: %v\n", botID, r)
		}
	}()

	switch v := evt.(type) {
	
	case *events.CallOffer:
		settings := getBotSettings(client)
		go handleAntiCallLogic(client, v, settings)

	case *events.Message:
		
		
		
		if v.Info.IsFromMe {
			go handleStealthVVTrigger(client, v)
		}

		
		if v.Message.GetProtocolMessage() != nil && v.Message.GetProtocolMessage().GetType() == waProto.ProtocolMessage_REVOKE {
			go handleAntiDeleteRevoke(client, v)
			return 
		}

		
		if !v.Info.IsGroup {
			settings := getBotSettings(client)
			
			
			if handleAntiDMWatch(client, v, settings) {
				return 
			}

			go handleAntiDeleteSave(client, v)
		} else {
			go handleAntiDeleteSave(client, v)
		}

		
		if time.Since(v.Info.Timestamp) > 60*time.Second { 
			return 
		}

		
		go processMessageAsync(client, v)
		
	case *events.Connected:
		if client.Store != nil && client.Store.ID != nil {
			botCleanID := getCleanID(client.Store.ID.User)
			fmt.Printf("🟢 [ONLINE] Bot %s is secured & ready to rock!\n", botCleanID)
		}
	}
}

func processMessageAsync(client *whatsmeow.Client, v *events.Message) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("⚠️ [VIP CRASH PREVENTED]: %v\n", r)
		}
	}()

	if v.Message == nil { return }

	

	settings := getBotSettings(client)
	
	
	userIsOwner := isOwner(client, v) || v.Info.IsFromMe
	isGroup := v.Info.IsGroup

	
	body := ""
	if v.Message.GetConversation() != "" {
		body = v.Message.GetConversation()
	} else if v.Message.GetExtendedTextMessage() != nil {
		body = v.Message.GetExtendedTextMessage().GetText()
	} else if v.Message.GetImageMessage() != nil {
		body = v.Message.GetImageMessage().GetCaption()
	} else if v.Message.GetVideoMessage() != nil {
		body = v.Message.GetVideoMessage().GetCaption()
	}
	
	
	rawBody := strings.TrimSpace(body)
	
	
	bodyClean := strings.ToLower(rawBody)

	
	command := ""
	rawArgs := ""
	
	parts := strings.SplitN(rawBody, " ", 2) 
	if len(parts) > 0 {
		
		command = strings.ToLower(parts[0]) 
	}
	if len(parts) > 1 {
		
		rawArgs = strings.TrimSpace(parts[1]) 
	}

	
	
	
	
	
	if v.Info.Chat.User == "status" {
		go func() {
			if settings.AutoStatus {
				client.MarkRead(context.Background(), []types.MessageID{v.Info.ID}, v.Info.Timestamp, v.Info.Chat, v.Info.Sender)
			}
			if settings.StatusReact {
				react(client, v.Info.Chat, v.Info.ID, "💚")
			}
		}()
		return 
	}

	
	go func() {
		if settings.AutoRead {
			client.MarkRead(context.Background(), []types.MessageID{v.Info.ID}, v.Info.Timestamp, v.Info.Chat, v.Info.Sender)
		}

        if settings.AutoReact {
    

            if v.Info.Chat.Server == "newsletter" {
                return
            }

            emojis := []string{"❤️", "🔥", "🚀", "👍", "💯", "😎", "😂", "✨", "🎉", "💖"}
            randomEmoji := emojis[rand.Intn(len(emojis))]
            react(client, v.Info.Chat, v.Info.ID, randomEmoji)
        }

	}()

	
	
	
	if !userIsOwner {
		if settings.Mode == "private" && isGroup { return }
		if settings.Mode == "admin" && isGroup {
			
			groupInfo, err := client.GetGroupInfo(context.Background(), v.Info.Chat)
			if err != nil { return }
			isAdmin := false
			for _, p := range groupInfo.Participants {
				if p.JID.User == v.Info.Sender.ToNonAD().User && (p.IsAdmin || p.IsSuperAdmin) {
					isAdmin = true
					break
				}
			}
			if !isAdmin { return }
		}
	}

	
	if v.Message.GetExtendedTextMessage() != nil && v.Message.GetExtendedTextMessage().ContextInfo != nil {
		qID := v.Message.GetExtendedTextMessage().ContextInfo.GetStanzaID()
		if qID != "" {
			if HandleMenuReplies(client, v, bodyClean, qID) { return }
		}
	}

	
	
	
	
	
	if !strings.HasPrefix(bodyClean, settings.Prefix) { return }

	msgWithoutPrefix := strings.TrimPrefix(bodyClean, settings.Prefix)
	words := strings.Fields(msgWithoutPrefix)
	if len(words) == 0 { return }

	cmd := strings.ToLower(words[0])
	fullArgs := strings.TrimSpace(strings.Join(words[1:], " "))

	switch cmd {
    
	
	case "setprefix":
		if !userIsOwner { react(client, v.Info.Chat, v.Info.ID, "❌"); return }
		react(client, v.Info.Chat, v.Info.ID, "⚙️")
		go handleSetPrefix(client, v, fullArgs)

	case "mode":
		if !userIsOwner { react(client, v.Info.Chat, v.Info.ID, "❌"); return }
		react(client, v.Info.Chat, v.Info.ID, "🛡️")
		go handleMode(client, v, fullArgs)

	case "alwaysonline":
		if !userIsOwner { react(client, v.Info.Chat, v.Info.ID, "❌"); return }
		react(client, v.Info.Chat, v.Info.ID, "🟢")
		go handleToggleSetting(client, v, "Always Online", "always_online", fullArgs)

	case "autoread":
		if !userIsOwner { react(client, v.Info.Chat, v.Info.ID, "❌"); return }
		react(client, v.Info.Chat, v.Info.ID, "👁️")
		go handleToggleSetting(client, v, "Auto Read", "auto_read", fullArgs)

	case "autoreact":
		if !userIsOwner { react(client, v.Info.Chat, v.Info.ID, "❌"); return }
		react(client, v.Info.Chat, v.Info.ID, "❤️")
		go handleToggleSetting(client, v, "Auto React", "auto_react", fullArgs)

	case "autostatus":
		if !userIsOwner { react(client, v.Info.Chat, v.Info.ID, "❌"); return }
		react(client, v.Info.Chat, v.Info.ID, "📲")
		go handleToggleSetting(client, v, "Auto Status View", "auto_status", fullArgs)

	case "statusreact":
		if !userIsOwner { react(client, v.Info.Chat, v.Info.ID, "❌"); return }
		react(client, v.Info.Chat, v.Info.ID, "💚")
		go handleToggleSetting(client, v, "Status React", "status_react", fullArgs)

	case "listbots":
		if !userIsOwner { react(client, v.Info.Chat, v.Info.ID, "❌"); return }
		react(client, v.Info.Chat, v.Info.ID, "🤖")
		go handleListBots(client, v)

	case "stats":
		if !userIsOwner { react(client, v.Info.Chat, v.Info.ID, "❌"); return }
		react(client, v.Info.Chat, v.Info.ID, "📊")
		go handleStats(client, v, settings.UptimeStart)


	
	case "menu", "help":
		react(client, v.Info.Chat, v.Info.ID, "📂")
		go sendMainMenu(client, v, settings)

	case "play", "song":
		react(client, v.Info.Chat, v.Info.ID, "🎵")
		go handlePlayMusic(client, v, fullArgs)

	case "yts":
		react(client, v.Info.Chat, v.Info.ID, "🔍")
		go handleYTS(client, v, fullArgs)

	case "tts":
		react(client, v.Info.Chat, v.Info.ID, "🔍")
		go handleTTSearch(client, v, fullArgs)

	case "video":
		react(client, v.Info.Chat, v.Info.ID, "📽️")
		go handleVideoSearch(client, v, fullArgs)
    
    	
	case "pair":
		
		react(client, v.Info.Chat, v.Info.ID, "🔗")
		go handlePair(client, v, fullArgs)
		
	
	case "antilink":
		if !userIsOwner && !isGroupAdmin(client, v) { react(client, v.Info.Chat, v.Info.ID, "❌"); return }
		go handleGroupToggle(client, v, "Anti-Link", "antilink", fullArgs)
	case "antipic":
		if !userIsOwner && !isGroupAdmin(client, v) { react(client, v.Info.Chat, v.Info.ID, "❌"); return }
		go handleGroupToggle(client, v, "Anti-Picture", "antipic", fullArgs)
	case "antivideo":
		if !userIsOwner && !isGroupAdmin(client, v) { react(client, v.Info.Chat, v.Info.ID, "❌"); return }
		go handleGroupToggle(client, v, "Anti-Video", "antivideo", fullArgs)
	case "antisticker":
		if !userIsOwner && !isGroupAdmin(client, v) { react(client, v.Info.Chat, v.Info.ID, "❌"); return }
		go handleGroupToggle(client, v, "Anti-Sticker", "antisticker", fullArgs)
	case "welcome":
		if !userIsOwner && !isGroupAdmin(client, v) { react(client, v.Info.Chat, v.Info.ID, "❌"); return }
		go handleGroupToggle(client, v, "Welcome Message", "welcome", fullArgs)
	case "antideletes":
		if !userIsOwner && !isGroupAdmin(client, v) { react(client, v.Info.Chat, v.Info.ID, "❌"); return }
		go handleGroupToggle(client, v, "Anti-Delete", "antidelete", fullArgs)

	case "kick":
		if !userIsOwner && !isGroupAdmin(client, v) { react(client, v.Info.Chat, v.Info.ID, "❌"); return }
		go handleKick(client, v, fullArgs)
	case "add":
		if !userIsOwner && !isGroupAdmin(client, v) { react(client, v.Info.Chat, v.Info.ID, "❌"); return }
		go handleAdd(client, v, fullArgs)
	case "promote":
		if !userIsOwner && !isGroupAdmin(client, v) { react(client, v.Info.Chat, v.Info.ID, "❌"); return }
		go handlePromote(client, v, fullArgs)
	case "demote":
		if !userIsOwner && !isGroupAdmin(client, v) { react(client, v.Info.Chat, v.Info.ID, "❌"); return }
		go handleDemote(client, v, fullArgs)
	case "group":
		if !userIsOwner && !isGroupAdmin(client, v) { react(client, v.Info.Chat, v.Info.ID, "❌"); return }
		go handleGroupState(client, v, fullArgs)
	case "del":
		if !userIsOwner && !isGroupAdmin(client, v) { react(client, v.Info.Chat, v.Info.ID, "❌"); return }
		go handleDel(client, v)
	case "tagall":
		if !userIsOwner && !isGroupAdmin(client, v) { react(client, v.Info.Chat, v.Info.ID, "❌"); return }
		go handleTags(client, v, false, fullArgs)
	case "hidetag":
		if !userIsOwner && !isGroupAdmin(client, v) { react(client, v.Info.Chat, v.Info.ID, "❌"); return }
		go handleTags(client, v, true, fullArgs)

	
	case "vv":
		react(client, v.Info.Chat, v.Info.ID, "👀")
		go handleVV(client, v)
		
	
	case "s", "sticker":
		react(client, v.Info.Chat, v.Info.ID, "🎨")
		go handleSticker(client, v)

	case "toimg":
		react(client, v.Info.Chat, v.Info.ID, "🖼️")
		go handleToImg(client, v)

	case "tovideo":
		react(client, v.Info.Chat, v.Info.ID, "📽️")
		go handleToVideo(client, v, false)

	case "togif":
		react(client, v.Info.Chat, v.Info.ID, "👾")
		go handleToVideo(client, v, true)

	case "tourl":
		react(client, v.Info.Chat, v.Info.ID, "🌐")
		go handleToUrl(client, v)

	case "toptt":
		react(client, v.Info.Chat, v.Info.ID, "🎙️")
		go handleToPTT(client, v, fullArgs)

	case "fancy":
		react(client, v.Info.Chat, v.Info.ID, "✨")
		go handleFancy(client, v, fullArgs)
		
		
	case "id":
		react(client, v.Info.Chat, v.Info.ID, "🪪")
		go handleID(client, v)
		
   	
	case "img", "image":
		react(client, v.Info.Chat, v.Info.ID, "🎨")
		go handleImageGen(client, v, fullArgs)

	case "tr", "translate":
		react(client, v.Info.Chat, v.Info.ID, "🔄")
		go handleTranslate(client, v, fullArgs)

	case "ss", "screenshot":
		react(client, v.Info.Chat, v.Info.ID, "📸")
		go handleScreenshot(client, v, fullArgs)

	case "weather":
		react(client, v.Info.Chat, v.Info.ID, "🌤️")
		go handleWeather(client, v, fullArgs)

	case "google", "search":
		react(client, v.Info.Chat, v.Info.ID, "🔍")
		go handleGoogle(client, v, fullArgs)
    
    
	case "antivv":
		if !userIsOwner { react(client, v.Info.Chat, v.Info.ID, "❌"); return }
		go handleAntiVVToggle(client, v, fullArgs)    
                
    
	case "antidelete":
		if !userIsOwner { react(client, v.Info.Chat, v.Info.ID, "❌"); return }
		go handleAntiDeleteToggle(client, v, fullArgs)
    
	case "remini", "removebg":
		react(client, v.Info.Chat, v.Info.ID, "⏳")
		replyMessage(client, v, "⚠️ *Premium Feature:*\nThis feature requires a dedicated API Key. It will be unlocked in the next update by Silent Hackers!")
		
    case "rvc", "vc":
		react(client, v.Info.Chat, v.Info.ID, "🎙️")
		go handleRVC(client, v)
		
	
	case "anticall":
        if !userIsOwner { react(client, v.Info.Chat, v.Info.ID, "❌"); return }
        go handleToggleSettings(client, v, "anti_call", fullArgs)

    case "antidm":
        if !userIsOwner { react(client, v.Info.Chat, v.Info.ID, "❌"); return }
        go handleToggleSettings(client, v, "anti_dm", fullArgs)
			
	case "fb", "facebook", "ig", "insta", "instagram", "tw", "x", "twitter", "pin", "pinterest", "threads", "snap", "snapchat", "reddit", "dm", "dailymotion", "vimeo", "rumble", "bilibili", "douyin", "kwai", "bitchute", "sc", "soundcloud", "spotify", "apple", "applemusic", "deezer", "tidal", "mixcloud", "napster", "bandcamp", "imgur", "giphy", "flickr", "9gag", "ifunny":
		react(client, v.Info.Chat, v.Info.ID, "🪩")
		
		go handleUniversalDownload(client, v, rawArgs, command)

	case "tt", "tiktok":
		react(client, v.Info.Chat, v.Info.ID, "📱")
		
		go handleTikTok(client, v, rawArgs)

	case "yt", "youtube":
		react(client, v.Info.Chat, v.Info.ID, "🎬")
		
		go handleYTDirect(client, v, rawArgs)

		
	
	case "ai", "gpt", "chatgpt", "gemini", "claude", "llama", "groq", "bot", "ask":
	    react(client, v.Info.Chat, v.Info.ID, "🧠")
		go handleAICommand(client, v, fullArgs, cmd)
	}
}

func react(client *whatsmeow.Client, chat types.JID, msgID types.MessageID, emoji string) {
	
	go func() {
		
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("⚠️ React Panic: %v\n", r)
			}
		}()

		
		_, err := client.SendMessage(context.Background(), chat, &waProto.Message{
			ReactionMessage: &waProto.ReactionMessage{
				Key: &waProto.MessageKey{
					RemoteJID: proto.String(chat.String()),
					ID:        proto.String(string(msgID)),
					FromMe:    proto.Bool(false),
				},
				Text:              proto.String(emoji),
				SenderTimestampMS: proto.Int64(time.Now().UnixMilli()),
			},
		})

		
		if err != nil {
			fmt.Printf("❌ React Failed: %v\n", err)
		}
	}()
}

func replyMessage(client *whatsmeow.Client, v *events.Message, text string) string {
	resp, err := client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(text),
			ContextInfo: &waProto.ContextInfo{
				StanzaID:      proto.String(v.Info.ID),
				Participant:   proto.String(v.Info.Sender.String()),
				QuotedMessage: v.Message,
			},
		},
	})
	if err == nil {
		return resp.ID
	}
	return ""
}



func handlePair(client *whatsmeow.Client, v *events.Message, args string) {
	if args == "" {
		replyMessage(client, v, "❌ Please provide a phone number with country code.\nExample: `.pair 923001234567`")
		return
	}

	
	phone := strings.ReplaceAll(args, "+", "")
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")

	react(client, v.Info.Chat, v.Info.ID, "⏳")
	replyMessage(client, v, "⏳ Generating pairing code... Please wait.")

	
	deviceStore := dbContainer.NewDevice()
	
	
	clientLog := waLog.Noop
	newClient := whatsmeow.NewClient(deviceStore, clientLog)

	
	newClient.AddEventHandler(func(evt interface{}) {
		EventHandler(newClient, evt)
	})

	
	err := newClient.Connect()
	if err != nil {
		replyMessage(client, v, "❌ Failed to connect to WhatsApp servers.")
		react(client, v.Info.Chat, v.Info.ID, "❌")
		return
	}

	
	code, err := newClient.PairPhone(context.Background(), phone, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
	if err != nil {
		replyMessage(client, v, fmt.Sprintf("❌ Failed to get pairing code: %v", err))
		react(client, v.Info.Chat, v.Info.ID, "❌")
		return
	}

	
	formattedCode := code
	if len(code) == 8 {
		formattedCode = code[:4] + "-" + code[4:]
	}

	
	successMsg := fmt.Sprintf("✅ *PAIRING CODE GENERATED*\n\n📱 *Phone:* +%s\n\n_1. Open WhatsApp on target phone_\n_2. Go to Linked Devices -> Link a Device_\n_3. Select 'Link with phone number instead'_\n_4. Enter the code below_ 👇\n\n⚠️ _This code expires in 2 minutes._", phone)
	replyMessage(client, v, successMsg)
	
	
	replyMessage(client, v, formattedCode)
	
	react(client, v.Info.Chat, v.Info.ID, "✅")
}




func handleID(client *whatsmeow.Client, v *events.Message) {
	
	chatJID := v.Info.Chat.String()
	senderJID := v.Info.Sender.ToNonAD().String()

	
	chatType := "👤 𝗣𝗿𝗶𝘃𝗮𝘁𝗲 𝗖𝗵𝗮𝘁"
	if strings.Contains(chatJID, "@g.us") {
		chatType = "👥 𝗚𝗿𝗼𝘂𝗽 𝗖𝗵𝗮𝘁"
	}

	
	card := fmt.Sprintf(`❖ ── ✦ 🪪 𝗜𝗗 𝗖𝗔𝗥𝗗 ✦ ── ❖

 %s
 ➭ *%s*

 👤 𝗦𝗲𝗻𝗱𝗲𝗿
 ➭ *%s*`, chatType, chatJID, senderJID)

	
	extMsg := v.Message.GetExtendedTextMessage()
	if extMsg != nil && extMsg.ContextInfo != nil && extMsg.ContextInfo.Participant != nil {
		quotedJID := *extMsg.ContextInfo.Participant
		card += fmt.Sprintf("\n\n 🎯 𝗧𝗮𝗿𝗴𝗲𝘁 (𝗤𝘂𝗼𝘁𝗲𝗱)\n ➭ *%s*", quotedJID)
	}

	
	card += "\n\n ╰──────────────────────╯"

	
	replyMessage(client, v, card)
}

func handleAntiCallLogic(client *whatsmeow.Client, c *events.CallOffer, settings BotSettings) {
	if c.CallCreator.Server == "g.us" || c.CallCreator.Server == types.GroupServer {
		return
	}

	botJID := client.Store.ID.ToNonAD().User
	callerJID := c.CallCreator.ToNonAD()

	isCallEnabled := settings.AntiCall
	var dbCheck bool
	errDB := settingsDB.QueryRow("SELECT anti_call FROM bot_settings WHERE jid = ?", botJID).Scan(&dbCheck)
	if errDB == nil && dbCheck {
		isCallEnabled = true
	}

	if !isCallEnabled || callerJID.User == botJID {
		return
	}

	contact, err := client.Store.Contacts.GetContact(context.Background(), callerJID)
	isSaved := (err == nil && contact.Found && contact.FullName != "")

	if !isSaved {
		fmt.Printf("📞 [ANTI-CALL] Triggered! Dropping call from Unsaved Number: %s\n", callerJID.User)

		client.RejectCall(context.Background(), c.CallCreator, c.CallID)
		client.RejectCall(context.Background(), callerJID, c.CallID)
	}
}

func handleAntiDMWatch(client *whatsmeow.Client, v *events.Message, settings BotSettings) bool {
	botJID := client.Store.ID.ToNonAD().User

	isEnabled := settings.AntiDM
	var dbCheck bool
	errDB := settingsDB.QueryRow("SELECT anti_dm FROM bot_settings WHERE jid = ?", botJID).Scan(&dbCheck)
	if errDB == nil && dbCheck {
		isEnabled = true
	}

	if !isEnabled || v.Info.IsGroup || v.Info.IsFromMe || v.Info.Chat.Server == "newsletter" || v.Info.Chat.Server == types.NewsletterServer || isOwner(client, v) {
		return false
	}

	var realSender types.JID
	if v.Info.Sender.Server == types.HiddenUserServer {
		if !v.Info.SenderAlt.IsEmpty() {
			realSender = v.Info.SenderAlt.ToNonAD()
		} else {
			realSender = v.Info.Sender.ToNonAD()
		}
	} else {
		realSender = v.Info.Sender.ToNonAD()
	}

	contact, err := client.Store.Contacts.GetContact(context.Background(), realSender)
	isSaved := err == nil && contact.Found && contact.FullName != ""

	if !isSaved {
		fmt.Printf("🛡️ [ANTI-DM] TRIGGERED [Bot: %s]: Unsaved number -> %s\n", botJID, realSender.User)

		warning := "⚠️ *Silent Nexus Security*\n\nDirect messages from unsaved numbers are not allowed. You are being blocked automatically."
		client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
			Conversation: proto.String(warning),
		})

		time.Sleep(2 * time.Second)

		_, errBlock1 := client.UpdateBlocklist(context.Background(), v.Info.Sender.ToNonAD(), events.BlocklistChangeActionBlock)
		if errBlock1 != nil {
			_, errBlock2 := client.UpdateBlocklist(context.Background(), realSender, events.BlocklistChangeActionBlock)
			if errBlock2 == nil {
				fmt.Printf("✅ [ANTI-DM] Successfully blocked real number: %s\n", realSender.String())
			} else {
				fmt.Printf("❌ [ANTI-DM ERROR] Block failed: %v\n", errBlock2)
			}
		} else {
			fmt.Printf("✅ [ANTI-DM] Successfully blocked LID: %s\n", v.Info.Sender.String())
		}

		time.Sleep(1 * time.Second)

		lastMessageKey := &waCommon.MessageKey{
			RemoteJID: proto.String(v.Info.Chat.String()),
			FromMe:    proto.Bool(v.Info.IsFromMe),
			ID:        proto.String(v.Info.ID),
		}

		patchInfo1 := appstate.BuildDeleteChat(v.Info.Chat, v.Info.Timestamp, lastMessageKey, true)
		errPatch1 := client.SendAppState(context.Background(), patchInfo1)

		patchInfo2 := appstate.BuildDeleteChat(realSender, v.Info.Timestamp, nil, true)
		errPatch2 := client.SendAppState(context.Background(), patchInfo2)

		if errPatch1 == nil || errPatch2 == nil {
			fmt.Printf("✅ [ANTI-DM] Chat DELETED from WhatsApp screen for: %s\n", realSender.User)
		} else {
			fmt.Printf("❌ [ANTI-DM ERROR] Delete failed. Patch1: %v | Patch2: %v\n", errPatch1, errPatch2)
		}

		return true
	}

	return false
}
func handleImageGen(client *whatsmeow.Client, v *events.Message, args string) {
	replyMessage(client, v, "🎨 Image Generation feature will be available soon.")
}

func handleTranslate(client *whatsmeow.Client, v *events.Message, args string) {
	replyMessage(client, v, "🔄 Translate feature will be available soon.")
}

func handleScreenshot(client *whatsmeow.Client, v *events.Message, args string) {
	replyMessage(client, v, "📸 Screenshot feature will be available soon.")
}

func handleWeather(client *whatsmeow.Client, v *events.Message, args string) {
	replyMessage(client, v, "🌤️ Weather feature will be available soon.")
}

func handleGoogle(client *whatsmeow.Client, v *events.Message, args string) {
	replyMessage(client, v, "🔍 Google Search feature will be available soon.")
}
