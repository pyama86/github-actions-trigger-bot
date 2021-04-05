package workers

import (
	"context"
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

func Test_canLock(t *testing.T) {
	type args struct {
		key   string
		value string
		ttl   string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				key:   "test",
				value: "value",
				ttl:   "3s",
			},
			want:    true,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		ctx := context.Background()
		t.Run(tt.name, func(t *testing.T) {
			got, err := canLock(ctx, tt.args.key, tt.args.value, tt.args.ttl)
			if (err != nil) != tt.wantErr {
				t.Errorf("canLock() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("canLock() = %v, want %v", got, tt.want)
			}

			got, err = canLock(ctx, tt.args.key, tt.args.value, tt.args.ttl)
			if (err != nil) != tt.wantErr {
				t.Errorf("canLock() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !got {
				t.Errorf("canLock() = %v, want %v", got, true)
			}

			got, err = canLock(ctx, tt.args.key, "other user", tt.args.ttl)
			if (err != nil) != tt.wantErr {
				t.Errorf("canLock() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got {
				t.Errorf("canLock() = %v, want %v", got, false)
			}
		})
	}
}
