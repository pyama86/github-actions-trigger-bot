package workers

import (
	"reflect"
	"testing"

	"github.com/k0kubun/pp"
)

func Test_parseTriggerMessage(t *testing.T) {
	tests := []struct {
		name string
		text string
		want map[string]string
	}{
		{
			name: "ok",
			text: "org/repo task branch:default",
			want: map[string]string{
				"org":    "org",
				"repo":   "repo",
				"task":   "task",
				"branch": "default",
			},
		},
		{
			name: "unmatch",
			text: "unmatch",
			want: nil,
		},
		{
			name: "ok",
			text: "org/repo task foo:bar hoge:fuga",
			want: map[string]string{
				"org":  "org",
				"repo": "repo",
				"task": "task",
				"foo":  "bar",
				"hoge": "fuga",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseTriggerMessage(tt.text); !reflect.DeepEqual(got, tt.want) {
				pp.Println(got)
				t.Errorf("parseTriggerMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}
