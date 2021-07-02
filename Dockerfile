FROM alpine:3
RUN apk --update add --no-cache ca-certificates tzdata && \
    cp /usr/share/zoneinfo/Asia/Tokyo /etc/localtime && \
    rm -rf /var/cache/apk/*
RUN adduser bot
USER bot
COPY github-actions-trigger /
CMD ["/github-actions-trigger"]
