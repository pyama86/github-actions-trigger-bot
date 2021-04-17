package workers

import (
	"os"

	"github.com/jrallison/go-workers"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

type PongParam struct {
	Event *slackevents.AppMentionEvent
}

func Pong(message *workers.Msg) {
	param := new(PongParam)
	if err := parseMessage(param, message); err != nil {
		panicWithLog(err)
	}

	api := slack.New(os.Getenv("SLACK_BOT_TOKEN"))
	if _, _, err := api.PostMessage(param.Event.Channel, slack.MsgOptionText("pong or sing a song?", false)); err != nil {
		panicWithLog(err)
	}
}
