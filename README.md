# Simple Komodo Telegram Alerter

A simple server that accepts Komodo alerts and forwards them to a selected Telegram chat.

## Config

Add a custom webhook in the Komodo alert settings: http://alerter:9000

Compose:
```yaml
services:

  # ---- Alerter --------------------
    alerter:
        image: ghcr.io/avillx/alerter:latest
        restart: unless-stopped
        environment:
        # Telegram bot token (required)
        - BOT_TOKEN=${BOT_TOKEN}

        # Telegram chat ID to send alerts to (required)
        - CHAT_ID=${CHAT_ID}

    networks:
        # Must be in the same network as Komodo
        - komodo
```
