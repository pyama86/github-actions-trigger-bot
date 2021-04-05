FROM alpine:latest
RUN apk --update add ca-certificates tzdata && \
    cp /usr/share/zoneinfo/Asia/Tokyo /etc/localtime && \
    apk del tzdata && \
    rm -rf /var/cache/apk/*
ADD github-actions-trigger /
CMD ["/github-actions-trigger"]
