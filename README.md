# Simple komodo telegram alerter

A simple endpoint to accepts komado alerts and sends it in selected telegram chat 

## Config
Custom endpoint in comado settings: `http://alerter:9000`

Compose:
```yaml
services:

# ---- Alerter --------------------
    alerter:
        image: ghcr.io/avillx/alerter:latest
    restart: unless-stopped
    environment:
        ## Telegram bot token *Required*
        - BOT_TOKEN=$BOT_TOKEN

        ## Telegram ID of chat sink *Required*
        - CHAT_ID=$CHAT_ID

    network:
        ## container must be in one network with komado
        - komado
```