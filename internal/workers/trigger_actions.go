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
	"github.com/thoas/go-funk"
	"golang.org/x/oauth2"
)

var commonRegexp = `\w\-\_\.`
var triggerActionsExp = regexp.MustCompile(`(?P<org>[^\/]+)\/(?P<repo>[` +
	commonRegexp + `]+)[^\w]+(?P<task>[` + commonRegexp + `]+)[^\w]*` +
	`(?P<params>([^:\s]+:[^:\s]+) |([^:\s]+:[^:\s]+))*$`)
var requireParams = []string{"repo", "org", "task"}

type TriggerActionsParams struct {
	Event *slackevents.AppMentionEvent
}

func parseTriggerMessage(text string) map[string]string {
	match := triggerActionsExp.FindAllStringSubmatch(text, -1)
	result := make(map[string]string)
	if len(match) == 0 {
		return nil
	}
	for i, name := range triggerActionsExp.SubexpNames() {
		if i != 0 && name != "" && funk.ContainsString(requireParams, name) {
			result[name] = match[0][i]
		}
	}

	for _, v := range match[0][1:] {
		if strings.Index(v, ":") > 0 {
			kv := strings.Split(v, ":")
			result[kv[0]] = kv[1]
		}
	}
	return result
}

func TriggerActions(message *workers.Msg) {

	param := new(TriggerActionsParams)
	if err := parseMessage(param, message); err != nil {
		panicWithLog(err)
	}

	api := slack.New(os.Getenv("SLACK_BOT_TOKEN"))
	text := strings.Split(param.Event.Text, " ")
	result := parseTriggerMessage(strings.Join(text[1:], " "))

	if result == nil {
		if _, _, err := api.PostMessage(
			param.Event.Channel,
			slack.MsgOptionText("format error, please send me [@botname <org/repo> <task> <key>:<value> ...]", false)); err != nil {
			panicWithLog(err)
		}
	}

	for _, r := range requireParams {
		if result[r] == "" {
			if _, _, err := api.PostMessage(
				param.Event.Channel,
				slack.MsgOptionText("format error, please send me [@botname <org/repo> <task> <key>:<value> ...]", false)); err != nil {
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

	if result["branch"] != "" {
		result["ref"] = result["branch"]

	}

	bytes, err := json.Marshal(result)
	if err != nil {
		panicWithLog(err)
	}

	payload := json.RawMessage(bytes)
	input := github.DispatchRequestOptions{EventType: result["task"], ClientPayload: &payload}

	logrus.Infof("github actions payload %s", string(payload))
	_, _, err = client.Repositories.Dispatch(ctx, result["org"], result["repo"], input)
	if err != nil {
		panicWithLog(err)
	}

	if _, _, err := api.PostMessage(param.Event.Channel, slack.MsgOptionText(fmt.Sprintf("%s/%s %s is starting", result["org"], result["repo"], result["task"]), false)); err != nil {
		panicWithLog(err)
	}
}
