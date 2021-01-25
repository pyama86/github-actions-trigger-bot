
run:
	go run .

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o github-actions-trigger main.go
image: build
	docker build -t pyama/github-actions-bot .
	docker push pyama/github-actions-bot
