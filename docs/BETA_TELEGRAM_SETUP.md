# DailyOps Telegram Setup

1. Create a dedicated non-production bot with Telegram BotFather.
2. For a direct chat, open the bot conversation and send `/start`. For a group
   or channel, add the bot and grant only the permission needed to post.
3. Determine the destination chat ID using an approved Telegram method. Do not
   paste tokens or private chat data into an issue, screenshot, or support chat.
4. In DailyOps setup, enter the chat ID and create the `TELEGRAM_BOT`
   credential. The token is stored encrypted in the external data directory.
5. Select **Test Telegram**. This calls `getMe` and `getChat`; it does not call
   `sendMessage`. Require `Valid` before completing setup.
6. Run one report with sanitized source data and confirm exactly one message.

If the token might be exposed, revoke it with BotFather and follow
[Credential Rotation](CREDENTIAL_ROTATION.md). For `telegram_unauthorized`,
replace the token. For `telegram_chat_inaccessible`, send `/start`, check bot
membership and permissions, and verify the chat ID.

