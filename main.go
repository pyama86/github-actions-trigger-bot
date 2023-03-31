package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/joho/godotenv"
	goworkers "github.com/jrallison/go-workers"
	"github.com/pyama86/github-actions-trigger-bot/internal/workers"
	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		logrus.Warn("Error loading .env file")
	}
	reporeg := regexp.MustCompile(`\w+\/\w+`)

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/slack/events", func(w http.ResponseWriter, r *http.Request) {
		verifier, err := slack.NewSecretsVerifier(r.Header, os.Getenv("SLACK_SIGNING_SECRET"))
		if err != nil {
			logrus.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		bodyReader := io.TeeReader(r.Body, &verifier)
		body, err := ioutil.ReadAll(bodyReader)
		if err != nil {
			logrus.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := verifier.Ensure(); err != nil {
			logrus.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
		if err != nil {
			logrus.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		switch eventsAPIEvent.Type {
		case slackevents.URLVerification:
			var res *slackevents.ChallengeResponse
			if err := json.Unmarshal(body, &res); err != nil {
				logrus.Error(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			if _, err := w.Write([]byte(res.Challenge)); err != nil {
				logrus.Error(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		case slackevents.CallbackEvent:
			innerEvent := eventsAPIEvent.InnerEvent
			switch event := innerEvent.Data.(type) {
			case *slackevents.AppMentionEvent:
				logrus.Info(event.Text)
				// for slack remind
				event.Text = strings.Replace(event.Text, "Reminder: ", "", -1)
				event.Text = strings.TrimSuffix(event.Text, ".")

				event.Text = strings.Replace(event.Text, "\u00a0", " ", -1)
				message := strings.Split(event.Text, " ")
				command := message[1]
				api := slack.New(os.Getenv("SLACK_BOT_TOKEN"))

				switch {
				case command == "ping":
					if _, _, err := api.PostMessage(event.Channel, slack.MsgOptionText("pong or sing a song?", false)); err != nil {
						logrus.Error(err)
						w.WriteHeader(http.StatusInternalServerError)
					}

					w.WriteHeader(http.StatusOK)
				case command == "help":
					if _, _, err := api.PostMessage(event.Channel, slack.MsgOptionText("<org/repo> <task> <key:value>...", false)); err != nil {
						logrus.Error(err)
						w.WriteHeader(http.StatusInternalServerError)
					}

					w.WriteHeader(http.StatusOK)
				case reporeg.MatchString(command):
					logrus.Info(command)
					_, err := goworkers.EnqueueWithOptions("trigger_actions", "Add", workers.TriggerActionsParams{
						Event: event,
					}, goworkers.EnqueueOptions{Retry: true})
					if err != nil {
						logrus.Error(err)
						w.WriteHeader(http.StatusInternalServerError)
					}

				}
			}
		}
	})
	redisURL := "localhost:6379"
	redisDB := "0"
	if os.Getenv("REDIS_URL") != "" {
		redisURL = os.Getenv("REDIS_URL")
	}
	if os.Getenv("REDIS_DB") != "" {
		redisDB = os.Getenv("REDIS_DB")
	}

	goworkers.Configure(map[string]string{
		"server":        redisURL,
		"database":      redisDB,
		"pool":          "30",
		"process":       "1",
		"poll_interval": "1",
	})

	goworkers.Process("pong", workers.Pong, 10)
	goworkers.Process("trigger_actions", workers.TriggerActions, 10)
	go func() {
		if err := http.ListenAndServe(":8080", nil); err != nil {
			logrus.Fatal(err)
		}
	}()

	logrus.Info("[INFO] Server listening")
	goworkers.Run()

}
