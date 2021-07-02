FROM alpine:latest
RUN apk --update add --no-cache ca-certificates tzdata && \
    cp /usr/share/zoneinfo/Asia/Tokyo /etc/localtime && \
    rm -rf /var/cache/apk/*
COPY github-actions-trigger /
CMD ["/github-actions-trigger"]
