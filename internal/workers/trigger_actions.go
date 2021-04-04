package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/caarlos0/env/v6"
	"github.com/google/go-github/v33/github"
	"github.com/jrallison/go-workers"
	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/thoas/go-funk"
	"golang.org/x/oauth2"
)

var commonRegexp = `\w\-\_\.`
var triggerActionsBaseExp = regexp.MustCompile(`(?P<org>[^\/]+)\/(?P<repo>[` +
	commonRegexp + `]+)[^\w]+(?P<task>[` + commonRegexp + `]+)`)
var triggerActionsParamsExp = regexp.MustCompile(`([[:alnum:]]+:[[:alnum:]]+)`)

var requireParams = []string{"repo", "org", "task"}

type TriggerActionsParams struct {
	Event *slackevents.AppMentionEvent
}

type config struct {
	UnlockTaskName     string `env:"ACTIONS_UNLOCK_TASKNAME" envDefault:"unlock"`
	LockKeyParamsKey   string `env:"ACTIONS_LOCK_KEY" envDefault:"branch"`
	LockValueParamsKey string `env:"ACTIONS_LOCK_VALUE" envDefault:"user"`
	LockTTLParamsKey   string `env:"ACTIONS_LOCK_TTL" envDefault:"ttl"`
}

func parseTriggerMessage(text string) map[string]string {
	match := triggerActionsBaseExp.FindAllStringSubmatch(text, -1)
	result := make(map[string]string)
	if len(match) == 0 {
		return nil
	}

	for i, name := range triggerActionsBaseExp.SubexpNames() {
		if i != 0 && name != "" && funk.ContainsString(requireParams, name) {
			result[name] = match[0][i]
		}
	}

	match = triggerActionsParamsExp.FindAllStringSubmatch(text, -1)
	if len(match) == 0 {
		return result
	}
	for _, v := range match {
		if strings.Index(v[0], ":") > 0 {
			kv := strings.Split(v[0], ":")
			result[kv[0]] = kv[1]
		}
	}

	return result
}

func canLock(ctx context.Context, key, value, ttl string) (bool, error) {

	t, err := time.ParseDuration(ttl)
	if err != nil {
		return false, err
	}
	return redisClient.SetNX(ctx, key, value, t).Result()
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
	cfg := config{}
	if err := env.Parse(&cfg); err != nil {
		panicWithLog(err)
	}

	ctx := context.Background()
	if result[cfg.LockKeyParamsKey] != "" && result[cfg.LockValueParamsKey] != "" && result[cfg.LockTTLParamsKey] != "" {
		key := strings.Join([]string{
			result["org"],
			result["repo"],
			result[cfg.LockKeyParamsKey],
		}, "-")

		if result["task"] == cfg.UnlockTaskName {
			val, err := redisClient.Get(ctx, key).Result()
			if err != nil {
				panicWithLog(err)
			}
			if val == result[cfg.LockValueParamsKey] {
				_, err := redisClient.Del(ctx, key).Result()
				if err != nil {
					panicWithLog(err)
				}
				if _, _, err := api.PostMessage(
					param.Event.Channel,
					slack.MsgOptionText(fmt.Sprintf("%s/%s release lock from %s", result["org"], result["repo"], val), false)); err != nil {
					panicWithLog(err)
				}
				return
			}
		}

		getLock, err := canLock(ctx, key, result[cfg.LockValueParamsKey], result[cfg.LockTTLParamsKey])
		if err != nil {
			panicWithLog(err)
		}
		logrus.Infof("get lock %s from %s", result[cfg.LockValueParamsKey], result[cfg.LockTTLParamsKey])

		if !getLock {
			val, err := redisClient.Get(ctx, key).Result()
			if err != nil {
				panicWithLog(err)
			}

			warn := " ************ WARNING ************"
			if _, _, err := api.PostMessage(
				param.Event.Channel,
				slack.MsgOptionText(fmt.Sprintf("%s\n%s/%s is locking from %s\n%s", warn, result["org"], result["repo"], val, warn), false)); err != nil {
				panicWithLog(err)
			}
			return

		}
	}

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
