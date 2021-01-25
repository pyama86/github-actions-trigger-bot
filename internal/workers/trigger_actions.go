package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/google/go-github/v33/github"
	"github.com/jrallison/go-workers"
	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"golang.org/x/oauth2"
)

var triggerActionsExp = regexp.MustCompile(`(?P<org>[^\/]+)\/(?P<repo>[\w\-\_]+)[^\w]+(?P<task>[\w\-\_]+)[^\w]+branch:(?P<branch>[\w\-\_]+)`)

type TriggerActionsParams struct {
	Event *slackevents.AppMentionEvent
}

func TriggerActions(message *workers.Msg) {
	requireParams := []string{"repo", "org", "task", "branch"}

	param := new(TriggerActionsParams)
	if err := parseMessage(param, message); err != nil {
		panicWithLog(err)
	}

	api := slack.New(os.Getenv("SLACK_BOT_TOKEN"))

	text := strings.Split(param.Event.Text, " ")
	match := triggerActionsExp.FindStringSubmatch(strings.Join(text[1:], " "))
	result := make(map[string]string)
	for i, name := range triggerActionsExp.SubexpNames() {
		if i != 0 && name != "" {
			result[name] = match[i]
		}
	}

	for _, r := range requireParams {
		if result[r] == "" {
			if _, _, err := api.PostMessage(
				param.Event.Channel,
				slack.MsgOptionText("format error, please send me [@botname <org/repo> <task> branch:<banrah_name>]", false)); err != nil {
				panicWithLog(err)
			}
			return

		}
	}
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	if os.Getenv("GITHUB_API") != "" && os.Getenv("GITHUB_UPLOADS") != "" {
		c, err := github.NewEnterpriseClient(os.Getenv("GITHUB_API"), os.Getenv("GITHUB_UPLOADS"), tc)
		if err != nil {
			panicWithLog(err)
		}
		client = c
	}

	pst := struct {
		Ref string `json:"ref"`
	}{
		Ref: result["branch"],
	}

	bytes, _ := json.Marshal(pst)
	payload := json.RawMessage(bytes)
	input := github.DispatchRequestOptions{EventType: result["task"], ClientPayload: &payload}

	logrus.Infof("github actions payload %s", string(payload))
	_, _, err := client.Repositories.Dispatch(ctx, result["org"], result["repo"], input)
	if err != nil {
		panicWithLog(err)
	}

	if _, _, err := api.PostMessage(param.Event.Channel, slack.MsgOptionText(fmt.Sprintf("%s/%s %s is starting", result["org"], result["repo"], result["task"]), false)); err != nil {
		panicWithLog(err)
	}
}
