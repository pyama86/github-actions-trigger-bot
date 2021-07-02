FROM alpine:3
RUN apk --update add --no-cache ca-certificates tzdata && \
    cp /usr/share/zoneinfo/Asia/Tokyo /etc/localtime && \
    rm -rf /var/cache/apk/*
RUN adduser -S bot \
    && echo "bot ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers \
        && echo 'bot:bot' | chpasswd
USER bot
COPY github-actions-trigger /
CMD ["/github-actions-trigger"]
