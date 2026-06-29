# jiral-tg

Personal Telegram bot that uploads Telegram attachments to [jiral](https://github.com/FrostyDeFreeze/jiral) and forwards Instagram/TikTok links to remote fetch

## Setup

1. Copy `config.example.json` to `config.json`
2. Set `telegramBotToken`, `allowedUserIds`, `jiralBaseUrl`, and `jiralApiToken`
3. Run:

```bash
go run .
```

The bot uses long polling and processes one upload/fetch task at a time. The default Telegram file download limit is 20 MB because this uses the regular Telegram Bot API
