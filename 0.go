package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

var SendLogo = true          
var ChannelForwarding = true 

func sendMainMenu(client *whatsmeow.Client, v *events.Message, settings BotSettings) {
	uptimeStr := getUptimeString(settings.UptimeStart)

	menu := fmt.Sprintf(`┏━━━〔 👑 𝗔𝗠𝗠𝗔𝗥 𝗛𝟰𝗖𝗞𝟯𝗥 👑 〕━━━┈
┃ 👤 *Owner:* Ammar H4ck3r
┃ ⚙️ *Mode:* %[1]s
┃ ⏱️ *Uptime:* %[2]s
┃ ⚡ *Prefix:* [ %[3]s ]
┃ 📊 *Commands:* 551
┗━━━━━━━━━━━━━━━━━━┈

┍──╼〔 📺 *YOUTUBE MENU* 〕
│ ⬡ %[3]splay
│ ⬡ %[3]ssong
│ ⬡ %[3]svideo
│ ⬡ %[3]syt
│ ⬡ %[3]syts
┕━━━━━━━━━━━━━━━━━━┈

┍──╼〔 📱 *TIKTOK MENU* 〕
│ ⬡ %[3]stt
│ ⬡ %[3]stiktok
│ ⬡ %[3]stts
┕━━━━━━━━━━━━━━━━━━┈

┍──╼〔 🌐 *DOWNLOAD MENU* 〕
│ ⬡ %[3]sfb
│ ⬡ %[3]sfacebook
│ ⬡ %[3]sig
│ ⬡ %[3]sinsta
│ ⬡ %[3]stw
│ ⬡ %[3]sx
│ ⬡ %[3]ssnap
│ ⬡ %[3]sthreads
│ ⬡ %[3]spin
│ ⬡ %[3]sreddit
┕━━━━━━━━━━━━━━━━━━┈

┍──╼〔 🧠 *AI CHAT* 〕
│ ⬡ %[3]sai
│ ⬡ %[3]sask
│ ⬡ %[3]sgpt
│ ⬡ %[3]schatgpt
│ ⬡ %[3]sgemini
│ ⬡ %[3]sclaude
│ ⬡ %[3]sllama
┕━━━━━━━━━━━━━━━━━━┈

┍──╼〔 🛡️ *GROUP MENU* 〕
│ ⬡ %[3]santilink
│ ⬡ %[3]swelcome
│ ⬡ %[3]skick
│ ⬡ %[3]sadd
│ ⬡ %[3]spromote
│ ⬡ %[3]sdemote
│ ⬡ %[3]stagall
│ ⬡ %[3]shidetag
│ ⬡ %[3]sgroup
│ ⬡ %[3]sdel
┕━━━━━━━━━━━━━━━━━━┈

┍──╼〔 ⚙️ *OWNER MENU* 〕
│ ⬡ %[3]ssetprefix
│ ⬡ %[3]smode
│ ⬡ %[3]sstats
│ ⬡ %[3]spair
│ ⬡ %[3]salwaysonline
│ ⬡ %[3]sautoread
│ ⬡ %[3]sautoreact
│ ⬡ %[3]sautostatus
│ ⬡ %[3]sstatusreact
┕━━━━━━━━━━━━━━━━━━┈

┍──╼〔 🛠️ *UTILITY* 〕
│ ⬡ %[3]svv
│ ⬡ %[3]sid
│ ⬡ %[3]svc
┕━━━━━━━━━━━━━━━━━━┈

┍──╼〔 🎨 *EDITING ZONE* 〕
│ ⬡ %[3]ss
│ ⬡ %[3]ssticker
│ ⬡ %[3]stoimg
│ ⬡ %[3]stogif
│ ⬡ %[3]stovideo
│ ⬡ %[3]stourl
│ ⬡ %[3]stoptt
│ ⬡ %[3]sfancy
┕━━━━━━━━━━━━━━━━━━┈

┍──╼〔 ✨ *AI TOOLS* 〕
│ ⬡ %[3]simg
│ ⬡ %[3]sremini
│ ⬡ %[3]sremovebg
│ ⬡ %[3]str
│ ⬡ %[3]sss
│ ⬡ %[3]sgoogle
│ ⬡ %[3]sweather
┕━━━━━━━━━━━━━━━━━━┈

   🔥 *POWERED BY AMMAR H4CK3R* 🔥`,
		strings.ToUpper(settings.Mode), uptimeStr, settings.Prefix)

	var contextInfo *waProto.ContextInfo
	if ChannelForwarding {
		contextInfo = &waProto.ContextInfo{
			ForwardingScore: proto.Uint32(999),
			IsForwarded:     proto.Bool(true),
			ForwardedNewsletterMessageInfo: &waProto.ContextInfo_ForwardedNewsletterMessageInfo{
				NewsletterJID:  proto.String("120363403320186072@newsletter"),
				NewsletterName: proto.String("AMMAR H4CK3R OFFICIAL ✅"),
			},
		}
	}

	var msg *waProto.Message

	if SendLogo {
		imageBytes, err := os.ReadFile("logo.png")
		if err != nil {
			fmt.Println("Error reading logo.png:", err)
		
			msg = &waProto.Message{
				ExtendedTextMessage: &waProto.ExtendedTextMessage{
					Text:        proto.String(menu),
					ContextInfo: contextInfo,
				},
			}
		} else {
		
			resp, err := client.Upload(context.Background(), imageBytes, whatsmeow.MediaImage)
			if err != nil {
				fmt.Println("Error uploading image:", err)
				
				msg = &waProto.Message{
					ExtendedTextMessage: &waProto.ExtendedTextMessage{
						Text:        proto.String(menu),
						ContextInfo: contextInfo,
					},
				}
			} else {
		
				msg = &waProto.Message{
					ImageMessage: &waProto.ImageMessage{
						Caption:       proto.String(menu),
						ContextInfo:   contextInfo,
						Mimetype:      proto.String("image/png"),
						URL:           proto.String(resp.URL),
						DirectPath:    proto.String(resp.DirectPath),
						MediaKey:      resp.MediaKey,
						FileEncSHA256: resp.FileEncSHA256,
						FileSHA256:    resp.FileSHA256,
						FileLength:    proto.Uint64(uint64(resp.FileLength)),
					},
				}
			}
		}
	} else {
		msg = &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text:        proto.String(menu),
				ContextInfo: contextInfo,
			},
		}
	}

	client.SendMessage(context.Background(), v.Info.Chat, msg)
}