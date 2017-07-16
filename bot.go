package main

import (
	"os"
	tbot "github.com/go-telegram-bot-api/telegram-bot-api"
	"net/http"
	scraper2 "github.com/cardigann/go-cloudflare-scraper"
	"golang.org/x/net/html"
	"strings"
	"io/ioutil"
)

var (
	TOKEN  = ""
	GO_ENV = ""
	PORT   = ""
)

type InstagramResponse struct {
	Username string
	Realname string
	image    string
}

func main() {

	Info.Println("Starting Bot...")

	PORT = os.Getenv("PORT")
	if PORT == "" {
		Error.Fatalln("$PORT not set")
	}

	GO_ENV = os.Getenv("GO_ENV")
	if GO_ENV == "" {
		Warn.Println("$GO_ENV not set")
		GO_ENV = "development"
	}

	Info.Println("$GO_ENV=" + GO_ENV)

	TOKEN = os.Getenv("TOKEN")
	if TOKEN == "" {
		Error.Fatalln("$TOKEN not set")
	}

	bot, err := tbot.NewBotAPI(TOKEN)
	if err != nil {
		Error.Fatalln("Error in starting bot", err.Error())
	}
	//if GO_ENV == "development" {
	bot.Debug = true
	//}

	Info.Printf("Authorized on account %s\n", bot.Self.UserName)

	updates := fetchUpdates(bot)

	for update := range updates {
		if update.Message == nil {
			msg := tbot.NewMessage(update.Message.Chat.ID, "Sorry, I am not sure what you mean, Type /help to get help")
			bot.Send(msg)
			continue
		}
		handleUpdates(bot, update)
	}
}

func fetchUpdates(bot *tbot.BotAPI) tbot.UpdatesChannel {
	if GO_ENV == "development" {
		//Use polling, because testing on local machine

		//Remove webhook
		bot.RemoveWebhook()

		Info.Println("Using Polling Method to fetch updates")
		u := tbot.NewUpdate(0)
		u.Timeout = 60
		updates, err := bot.GetUpdatesChan(u)
		if err != nil {
			Warn.Println("Problem in fetching updates", err.Error())
		}

		return updates

	} else {
		//	USe Webhooks, because deploying on heroku
		Info.Println("Setting webhooks to fetch updates")
		_, err := bot.SetWebhook(tbot.NewWebhook("http://dry-hamlet-60060.herokuapp.com/" + bot.Token))
		if err != nil {
			Error.Fatalln("Problem in setting webhook", err.Error())
		}

		updates := bot.ListenForWebhook("/" + bot.Token)

		Info.Println("Starting HTTPS Server")
		go http.ListenAndServe(":"+PORT, nil)

		return updates
	}
}

func handleUpdates(bot *tbot.BotAPI, u tbot.Update) {

	if u.Message.IsCommand() {
		switch u.Message.Text {
		case "/start", "/help":
			msg := tbot.NewMessage(u.Message.Chat.ID, "Give me an Instagram User, And I'll give you their Profile Picture")
			msg.ReplyToMessageID = u.Message.MessageID
			bot.Send(msg)

		default:
			msg := tbot.NewMessage(u.Message.Chat.ID, "Invalid Command")
			msg.ReplyToMessageID = u.Message.MessageID
			bot.Send(msg)
		}
		return
	}

	if u.Message.Text != "" {

		i, err := fetchInstagramPhoto(u.Message.Text)
		if err != nil {
			Warn.Println("Error in fetching Profile Picture", err.Error())

			msg := tbot.NewMessage(u.Message.Chat.ID, "Error in fetching User's Profile Picture")
			msg.ReplyToMessageID = u.Message.MessageID
			bot.Send(msg)
			return
		}

		if i.Username == "" && i.Realname == "" && i.image == "" {
			//	No such user
			msg := tbot.NewMessage(u.Message.Chat.ID, "Invalid User ID, Enter Valid User ID")
			msg.ReplyToMessageID = u.Message.MessageID
			bot.Send(msg)
			return
		}

		Info.Printf("Serving %s (@%s) Profile Picture\n", i.Realname, i.Username)

		imgBytes, err := downloadImage(i.image)

		if err != nil {
			Warn.Println("Error in downloading Image", err.Error())
			msg := tbot.NewMessage(u.Message.Chat.ID, "Error in downloading Image, Please retry")
			msg.ReplyToMessageID = u.Message.MessageID
			bot.Send(msg)
			return
		}

		msg := tbot.NewPhotoUpload(u.Message.Chat.ID, imgBytes)
		msg.ReplyToMessageID = u.Message.MessageID

		bot.Send(msg)

	}
}

func fetchInstagramPhoto(u string) (*InstagramResponse, error) {

	scraper, err := scraper2.NewTransport(http.DefaultTransport)
	if err != nil {
		return nil, err
	}

	c := http.Client{Transport: scraper}
	res, err := c.Get("https://instagram.com/" + u)

	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	h, err := html.Parse(res.Body)
	if err != nil {
		Warn.Println("Problem in parsing instagram page", err.Error())

		return nil, err
	}

	var f func(node *html.Node)

	i := &InstagramResponse{}

	f = func(n *html.Node) {

		if n.Type == html.ElementNode && n.Data == "meta" {

			find(n.Attr, i)
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(h)

	return i, nil
}

func find(attr []html.Attribute, insta *InstagramResponse) {

	for _, a := range attr {
		if a.Val == "og:image" {
			for _, v := range attr {
				if v.Key == "content" {

					img := strings.Replace(v.Val, "s150x150", "s1080x1080", 1)

					insta.image = img

				}
			}
		}

		if a.Val == "og:title" {
			for _, v := range attr {
				if v.Key == "content" {

					a := strings.Split(v.Val, " (@")

					insta.Realname = a[0]

					a = strings.Split(a[1], ")")

					insta.Username = a[0]

				}
			}
		}
	}
}

func downloadImage(u string) ([]byte, error) {
	resp, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}
