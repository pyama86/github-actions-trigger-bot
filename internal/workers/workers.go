package workers

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strconv"

	"github.com/go-redis/redis/v8"
	goworkers "github.com/jrallison/go-workers"
	"github.com/sirupsen/logrus"
)

var redisClient *redis.Client

func init() {
	redisURL := "localhost:6379"
	redisDB := 0
	if os.Getenv("REDIS_URL") != "" {
		redisURL = os.Getenv("REDIS_URL")
	}
	if os.Getenv("REDIS_DB") != "" {
		rb, err := strconv.Atoi(os.Getenv("REDIS_DB"))
		if err != nil {
			panic(err)
		}
		redisDB = rb
	}
	redisClient = redis.NewClient(
		&redis.Options{
			Addr:     redisURL,
			Password: os.Getenv("REDIS_PASSWORD"),
			DB:       redisDB,
		})
}
func parseMessage(s interface{}, message *goworkers.Msg) error {
	b, err := message.Args().Encode()
	if err != nil {
		return runtimeError(err)
	}

	if err := json.Unmarshal(b, &s); err != nil {
		return runtimeError(err)
	}
	return nil
}

func runtimeError(err error) error {
	if err != nil {
		_, src, line, _ := runtime.Caller(1)
		e := fmt.Errorf("file: %s, line: %d, message: %s", src, line, err.Error())
		return e
	}
	return err
}

func panicWithLog(e error) {
	logrus.Error(e)
	panic(e)
}
