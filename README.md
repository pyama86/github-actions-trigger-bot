# Github Actions Triggger Bot

It is GitHub Actoins Trigger Bot For Slack.

## setup
### 概要
Kubernetes上で動くアプリをSlack Appとして登録し、イベントをサブスクライブし、応答します。
主に必要な情報はSlack、GitHub(Enterprise)の認証情報です。
動かしてみたいけど、ハマってよくわからんというかたは、issueを立てていただければサポートします。


### kubernetes
```
$ kubectl apply -f manifests
```

GitHub Enterpriseで利用する場合は、エンドポイントの設定が必要です。

```yaml
env:
  - name: GITHUB_API
    value: "https://git.your.example.com/api/v3/"
  - name: GITHUB_UPLOADS
    value: "https://uploads.your.example.com"
```

Slackの設定はSlack Appを作成し、`Event Subscriptions` メニューから、`Subscribe to bot events` を選択し、`app_mention` イベントをアプリで受け取る必要があります。


## author
- pyama86
