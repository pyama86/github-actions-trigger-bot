
run:
	go run .

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o github-actions-trigger main.go
image: build
	docker build -t pyama/github-actions-trigger .
	docker push pyama/github-actions-trigger

test:
	go test github.com/pyama86/github-actions-trigger-bot/...
