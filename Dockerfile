FROM alpine:latest
RUN apk --update add ca-certificates && rm -rf /var/cache/apk/*

ADD github-actions-trigger /
CMD ["/github-actions-trigger"]
