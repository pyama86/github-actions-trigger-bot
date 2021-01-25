package workers

import (
	"encoding/json"
	"fmt"
	"runtime"

	goworkers "github.com/jrallison/go-workers"
	"github.com/sirupsen/logrus"
)

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
