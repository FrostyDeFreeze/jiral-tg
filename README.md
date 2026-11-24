# jiral-tg

Personal Telegram bot for [jiral](https://github.com/FrostyDeFreeze/jiral). It
uploads any file sent to it, as a photo, video or document, and makes jiral
fetch Instagram, TikTok, Twitch, YouTube and X links. Each file and link gets
its own reply with the link and copy and delete buttons.

## Deploy

Needs Go 1.25+ and a running jiral.

```bash
git clone https://github.com/FrostyDeFreeze/jiral-tg
cd jiral-tg
cp config.example.json config.json
go build -o jiral-tg .
cp jiral-tg.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now jiral-tg
```

Logs: `journalctl -u jiral-tg -f`.

## Config

`config.json` in the working directory. All fields are required, unknown fields
are an error.

| Field              | Meaning                           |
| ------------------ | --------------------------------- |
| `telegramBotToken` | token from @BotFather             |
| `allowedUserIds`   | Telegram user IDs the bot answers |
| `jiralBaseUrl`     | public jiral URL                  |
| `jiralApiToken`    | jiral `apiToken`                  |

The bot uses the regular Telegram Bot API, so files over 20 MB can't be
uploaded through it. Messages are handled one at a time.
