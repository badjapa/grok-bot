# Configuration

The Grok bot now supports flexible configuration using Viper, which allows you to configure the bot through:

1. **Configuration files** (YAML, JSON, TOML, etc.)
2. **Environment variables**
3. **Command line flags**

## Configuration File

Create a `config.yaml` file in your project directory or copy `config.example.yaml`:

```yaml
discord:
  token: "your_discord_bot_token_here"

grok:
  api_key: "your_grok_api_key_here"
  base_url: "https://api.x.ai/v1"
  model: "grok-4-fast"
  temperature: 0.5
  max_tokens: 1000
  timeout: "120s"
  stream: false

bot:
  max_history: 100
  verbose: false
  enable_emojis: true
  enable_history: true
  max_message_size: 2000
```

## Environment Variables

You can also use environment variables with the `GROK_BOT_` prefix:

```bash
export GROK_BOT_DISCORD_TOKEN="your_discord_bot_token"
export GROK_BOT_GROK_API_KEY="your_grok_api_key"
export GROK_BOT_GROK_MODEL="grok-4-fast"
export GROK_BOT_GROK_TEMPERATURE="0.7"
export GROK_BOT_BOT_MAX_HISTORY="50"
export GROK_BOT_BOT_VERBOSE="true"
```

## Command Line Usage

```bash
# Use default configuration (environment variables)
./grok-bot

# Use specific configuration file
./grok-bot -config /path/to/config.yaml

# Use configuration file in current directory
./grok-bot -config config.yaml
```

## Configuration Priority

Configuration values are loaded in the following order (later sources override earlier ones):

1. Default values
2. Configuration file
3. Environment variables
4. Command line flags

## Configuration Options

### Discord Configuration
- `discord.token` - Discord bot token (required)

### Grok API Configuration
- `grok.api_key` - Grok API key (required)
- `grok.base_url` - API base URL (default: "https://api.x.ai/v1")
- `grok.model` - Model to use (default: "grok-4-fast")
- `grok.temperature` - Response creativity (0.0-2.0, default: 0.5)
- `grok.max_tokens` - Maximum response length (default: 1000)
- `grok.timeout` - Request timeout (default: "120s")
- `grok.stream` - Enable streaming (default: false)

### Bot Behavior Configuration
- `bot.max_history` - Chat history size per channel (default: 100)
- `bot.verbose` - Enable verbose logging (default: false)
- `bot.enable_emojis` - Enable emoji support (default: true)
- `bot.enable_history` - Enable history population (default: true)
- `bot.max_message_size` - Max message size before file (default: 2000)
- `bot.default_system_message` - Custom system message for bot personality (default: Discord-specific instructions with emojis)

## Customizing the Bot's Personality

You can customize the bot's personality and behavior by modifying the `bot.default_system_message` configuration. This message is sent to the AI model with every request and defines how the bot should behave.

### Example Custom System Messages

**Professional Assistant:**
```yaml
bot:
  default_system_message: |
    You are a professional Discord bot assistant. Be helpful, polite, and informative. 
    Keep responses concise and professional. Avoid using informal language or emojis.
```

**Casual Gaming Bot:**
```yaml
bot:
  default_system_message: |
    You are a gaming-focused Discord bot. Be enthusiastic about games, use gaming terminology, 
    and be casual and fun. You can use gaming emojis and references when appropriate.
```

**Technical Support Bot:**
```yaml
bot:
  default_system_message: |
    You are a technical support bot. Focus on providing accurate, helpful technical information. 
    Be patient and thorough in your explanations. Ask clarifying questions when needed.
```

The default system message includes Discord-specific instructions and emoji references, but you can replace it entirely with your own custom message to change the bot's personality and behavior.

## Backward Compatibility

The bot maintains backward compatibility with the old environment variable approach:
- `DISCORD_TOKEN` - Discord bot token
- `GROK_API_KEY` - Grok API key
- `GROK_HISTORY_SIZE` - Chat history size
