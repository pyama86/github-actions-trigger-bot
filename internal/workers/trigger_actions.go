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
	"github.com/go-redis/redis/v8"
	"github.com/google/go-github/v36/github"
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
var triggerActionsParamsExp = regexp.MustCompile(`([\S]+:[\S]+)`)

var requireParams = []string{"repo", "org", "task"}

type TriggerActionsParams struct {
	Event *slackevents.AppMentionEvent
}

type config struct {
	UnlockTaskName     string `env:"ACTIONS_UNLOCK_TASKNAME" envDefault:"unlock"`
	LockKeyParamsKey   string `env:"ACTIONS_LOCK_KEY" envDefault:"stage"`
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

func canLock(ctx context.Context, key, value, ttl string) (bool, string, string, error) {

	t, err := time.ParseDuration(ttl)
	if err != nil {
		return false, "", "", err
	}
	lock, err := redisClient.SetNX(ctx, key, value, t).Result()
	if err != nil {
		return false, "", "", err
	}

	setTTL, err := redisClient.TTL(ctx, key).Result()
	if err != nil {
		return false, "", "", err
	}

	expireAt := time.Now().Add(setTTL).Format("2006/01/02 15:04:05")

	if !lock {
		v, err := redisClient.Get(ctx, key).Result()
		if err != nil {
			return false, "", "", err
		}

		return v == value, v, expireAt, nil
	}
	return true, value, expireAt, nil
}

func unlock(ctx context.Context, key string, result map[string]string, cfg *config, param *TriggerActionsParams, api *slack.Client) error {
	val, err := redisClient.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			if _, _, err := api.PostMessage(
				param.Event.Channel,
				slack.MsgOptionText(fmt.Sprintf("%s/%s hasn't any lock %s", result["org"], result["repo"], result[cfg.LockKeyParamsKey]), false)); err != nil {
				return err
			}
			return nil
		}
		return err
	}

	if val == result[cfg.LockValueParamsKey] {
		_, err := redisClient.Del(ctx, key).Result()
		if err != nil {
			return err
		}
		if _, _, err := api.PostMessage(
			param.Event.Channel,
			slack.MsgOptionText(fmt.Sprintf("%s/%s release lock from %s", result["org"], result["repo"], val), false)); err != nil {
			return err
		}
	} else {
		if _, _, err := api.PostMessage(
			param.Event.Channel,
			slack.MsgOptionText(fmt.Sprintf("%s/%s don't release lock, because lock owner is  %s", result["org"], result["repo"], val), false)); err != nil {
			return err
		}
	}
	return nil

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
	cfg := &config{}
	if err := env.Parse(cfg); err != nil {
		panicWithLog(err)
	}

	ctx := context.Background()

	if result[cfg.LockKeyParamsKey] != "" && result[cfg.LockValueParamsKey] != "" {
		key := strings.Join([]string{
			result["org"],
			result["repo"],
			result[cfg.LockKeyParamsKey],
		}, "-")

		if result["task"] == cfg.UnlockTaskName {
			if err := unlock(ctx, key, result, cfg, param, api); err != nil {
				panicWithLog(err)
			}
			return
		}

		if result[cfg.LockTTLParamsKey] != "" {
			getLock, lockValue, expireAt, err := canLock(ctx, key, result[cfg.LockValueParamsKey], result[cfg.LockTTLParamsKey])
			if err != nil {
				panicWithLog(err)
			}
			logrus.Infof("get lock %s from %s", result[cfg.LockValueParamsKey], result[cfg.LockTTLParamsKey])

			if !getLock {
				warn := "*========== WARNING ==========*"
				if _, _, err := api.PostMessage(
					param.Event.Channel,
					slack.MsgOptionText(fmt.Sprintf("%s\n%s/%s is locking from %s until %s\n%s", warn, result["org"], result["repo"], lockValue, expireAt, warn), false)); err != nil {
					panicWithLog(err)
				}
				return
			}
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

	startTime := time.Now()
	logrus.Infof("github actions payload %s", string(payload))
	_, _, err = client.Repositories.Dispatch(ctx, result["org"], result["repo"], input)
	if err != nil {
		panicWithLog(err)
	}

	var resultMessage = ""
	try := 30
	perPage := 100
	totalCount := 1
L:
	for range make([]int, try) {
		page := 1
		for (page-1)*perPage < totalCount {
			wfr, _, err := client.Actions.ListRepositoryWorkflowRuns(ctx, result["org"], result["repo"], &github.ListWorkflowRunsOptions{
				Event:       "repository_dispatch",
				Branch:      result["branch"],
				ListOptions: github.ListOptions{Page: page, PerPage: perPage},
			})
			if err != nil {
				logrus.Error(err)
				break L
			}
			totalCount = *wfr.TotalCount
			page++

			if wfr != nil && len(wfr.WorkflowRuns) > 0 {
				for _, w := range wfr.WorkflowRuns {
					logrus.Infof("task: %s, start_at: %s, created_at: %s", *w.Name, startTime.Local(), w.CreatedAt.Local())
					if startTime.Local().Before(w.CreatedAt.Local()) || startTime.Local().Equal(w.CreatedAt.Local()) {
						resultMessage = fmt.Sprintf("%s/%s %s is starting %s",
							result["org"], result["repo"], *w.Name, *w.HTMLURL)
						break L
					}
				}
			}
		}
		time.Sleep(1 * time.Second)
	}

	if resultMessage == "" {
		resultMessage = fmt.Sprintf("%s/%s %s is starting", result["org"], result["repo"], result["task"])
	}

	if _, _, err := api.PostMessage(param.Event.Channel, slack.MsgOptionText(resultMessage, false)); err != nil {
		panicWithLog(err)
	}
}
