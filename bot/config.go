package bot

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the Grok bot
type Config struct {
	Discord DiscordConfig `mapstructure:"discord"`
	Grok    GrokConfig    `mapstructure:"grok"`
	Bot     BotConfig     `mapstructure:"bot"`
	Server  ServerConfig  `mapstructure:"server"`
}

// DiscordConfig holds Discord-specific configuration
type DiscordConfig struct {
	Token string `mapstructure:"token"`
}

// GrokConfig holds Grok API-specific configuration
type GrokConfig struct {
	APIKey      string        `mapstructure:"api_key"`
	BaseURL     string        `mapstructure:"base_url"`
	Model       string        `mapstructure:"model"`
	Temperature float64       `mapstructure:"temperature"`
	MaxTokens   int           `mapstructure:"max_tokens"`
	Timeout     time.Duration `mapstructure:"timeout"`
	Stream      bool          `mapstructure:"stream"`
}

// BotConfig holds bot behavior configuration
type BotConfig struct {
	MaxHistory           int    `mapstructure:"max_history"`
	Verbose              bool   `mapstructure:"verbose"`
	EnableEmojis         bool   `mapstructure:"enable_emojis"`
	EnableHistory        bool   `mapstructure:"enable_history"`
	MaxMessageSize       int    `mapstructure:"max_message_size"`
	DefaultSystemMessage string `mapstructure:"default_system_message"`
}

// ServerConfig holds web server configuration
type ServerConfig struct {
	Port    string `mapstructure:"port"`
	Enabled bool   `mapstructure:"enabled"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Discord: DiscordConfig{
			Token: "",
		},
		Grok: GrokConfig{
			APIKey:      "",
			BaseURL:     "https://api.x.ai/v1",
			Model:       "grok-4-fast",
			Temperature: 0.5,
			MaxTokens:   1000,
			Timeout:     120 * time.Second,
			Stream:      false,
		},
		Bot: BotConfig{
			MaxHistory:           100,
			Verbose:              false,
			EnableEmojis:         true,
			EnableHistory:        true,
			MaxMessageSize:       2000,
			DefaultSystemMessage: getDefaultSystemMessage(),
		},
		Server: ServerConfig{
			Port:    "8080",
			Enabled: true,
		},
	}
}

// LoadConfig loads configuration from file and environment variables
func LoadConfig(configPath string) (*Config, error) {
	config := DefaultConfig()

	// Set up Viper
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("/etc/grok-bot")
	viper.AddConfigPath("$HOME/.grok-bot")

	// If a specific config path is provided, use it
	if configPath != "" {
		viper.SetConfigFile(configPath)
	}

	// Set environment variable prefix
	viper.SetEnvPrefix("GROK_BOT")
	viper.AutomaticEnv()

	// Bind environment variables to config keys
	viper.BindEnv("discord.token", "DISCORD_TOKEN")
	viper.BindEnv("grok.api_key", "GROK_API_KEY")
	viper.BindEnv("grok.base_url", "GROK_BASE_URL")
	viper.BindEnv("grok.model", "GROK_MODEL")
	viper.BindEnv("grok.temperature", "GROK_TEMPERATURE")
	viper.BindEnv("grok.max_tokens", "GROK_MAX_TOKENS")
	viper.BindEnv("grok.timeout", "GROK_TIMEOUT")
	viper.BindEnv("grok.stream", "GROK_STREAM")
	viper.BindEnv("bot.max_history", "GROK_HISTORY_SIZE")
	viper.BindEnv("bot.verbose", "GROK_VERBOSE")
	viper.BindEnv("bot.enable_emojis", "GROK_ENABLE_EMOJIS")
	viper.BindEnv("bot.enable_history", "GROK_ENABLE_HISTORY")
	viper.BindEnv("bot.max_message_size", "GROK_MAX_MESSAGE_SIZE")
	viper.BindEnv("bot.default_system_message", "GROK_DEFAULT_SYSTEM_MESSAGE")
	viper.BindEnv("server.port", "GROK_BOT_SERVER_PORT")
	viper.BindEnv("server.enabled", "GROK_BOT_SERVER_ENABLED")

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// Config file not found, use defaults and environment variables
	}

	// Unmarshal config
	if err := viper.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Validate required fields
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Discord.Token == "" {
		return fmt.Errorf("discord token is required")
	}
	if c.Grok.APIKey == "" {
		return fmt.Errorf("grok api key is required")
	}
	if c.Grok.BaseURL == "" {
		return fmt.Errorf("grok base url is required")
	}
	if c.Grok.Model == "" {
		return fmt.Errorf("grok model is required")
	}
	if c.Grok.Temperature < 0 || c.Grok.Temperature > 2 {
		return fmt.Errorf("grok temperature must be between 0 and 2")
	}
	if c.Grok.MaxTokens <= 0 {
		return fmt.Errorf("grok max tokens must be greater than 0")
	}
	if c.Bot.MaxHistory <= 0 {
		return fmt.Errorf("bot max history must be greater than 0")
	}
	if c.Bot.MaxMessageSize <= 0 {
		return fmt.Errorf("bot max message size must be greater than 0")
	}
	return nil
}

// GetConfigPath returns the path to the config file being used
func GetConfigPath() string {
	return viper.ConfigFileUsed()
}

// getDefaultSystemMessage returns the default system message
func getDefaultSystemMessage() string {
	return `You are Grok, a helpful Discord bot assistant. Be casual, friendly, and conversational. Keep responses concise but helpful and sometimes snarky. You're chatting in a Discord server, so feel free to be informal and you can swear if that seems appropriate

IMPORTANT: In the conversation history, user messages are formatted as "[Username]: message content" to help you understand who said what. You can reference users by name when appropriate. Assistant messages are your previous responses.

When appropriate, you may use these server emojis (only if they exist in the current server). Use sparingly and contextually:

<:FeelsEvilMan:734304991682232331> – mischievous / evil grin – for playful plotting.
<:FeelsLaunchWeek:734305000347402310> – excitement / hype – for launches and news.
<:FeelsOkayMan:734305008870359061> – content / accepting – it's fine, not great.
<:PepeRaugh:734305166429519883> – exaggerated laugh – goofy hilarious moments.
<:monkaHmm:734305212587835392> – skeptical / thinking / unsure.
<:monkaS:734305295014297660> – anxious / nervous – intense or scary.
<:PeepoHappy:734305400664359002> – pure joy – wholesome happiness.
<:PepePoint:734305443945381938> – pointing – draw attention to something.
<:PepeGive:734305636149493863> – offering / kindness.
<:peeposalute:734305772015583252> – salute / respect.
<:CringeChamp:734305931545804883> – cringe reaction – embarrassing moments.
<:KEKW:734305966190887012> – laughing hard – hysterical laughter.
<:Whoa:734306032758554625> – surprised / impressed.
<:ReallyPal:734306104636342293> – sarcastic disbelief – "really?"
<:PepeHardhat:734306379656986647> – construction / working – building or doing work.
<:SmoothBrain:734307118458601493> – dumb moment – playful jab at not thinking.
<:peepoGlare:734308800072384625> – judging / glaring – disapproval.
<:tbh:734309542573244497> – honesty / "to be honest".
<:PepeSimp:734312446633967636> – simping – infatuated or admiring.
<:waitwhat:736124216033804329> – confused / disbelief – "wait… what??"
<:SMILERS:747581716566376488> – forced smile – awkward positivity.
<:WhatSheSees:747991898962002010> – POV meme – humorous situations.
<:PepeSmile:753913643212734474> – genuine happiness – warm response.
<:MilkMe:761785362539741184> – weird / suggestive humor – meme context only.
<:peepoS:805018260461846548> – sad / disappointed.
<:KEKPOPCORN:805023229769023518> – watching drama – observing chaos or gossip.
<:LFG:805023270960889906> – hype – "Let's F***ing Go".
<:GAMINGHARDCORE:805036362675781663> – intense gaming / hype.
<:PogO:810377168587587594> – shocked / hype – "POG".
<:WICKED:811480967603945503> – hype / wicked cool.
<:SmogO:817230985715384351> – shocked / weird reaction.
<:KEKL:821725755020935168> – laughing / kek variant.
<:FeelsOldMan:827360929254473768> – nostalgia / feeling old.
<:yeet:840771085817479168> – throwing / excitement.
<:pepeLFG:840827942497681418> – LFG (Pepe version).
<:KEK:845891104914014272> – laughing / meme reaction.
<:yeehaw:845900394634805248> – cowboy / yeehaw energy.
<:bald:848093898172661780> – bald meme – joking insult.
<:clueless:955263155720839168> – naive / unaware.
<:Handshakege:1089337603423215729> – handshake – agreement / teamwork.
<:BirdStare:1090152526462062623> – staring / awkward silence.
<:BirdQuestion:1097031900805222421> – confused / curious.
<:susW:1115349088519602356> – "sus" / suspicious behavior.
<:JOEVER:1119325324912623758> – "it's over" – dramatic or joking.
<:YOUCANT:1137498477044187156> – denial / refusal.
<:Pointless:1140321867174002749> – stating something is pointless.
<:Smugjak:1157119284678496346> – smug expression – gloating / self-satisfaction.
<:soymouth:1157366984451833997> – soyjak open-mouth – overexcitement.
<:lookingR:1159213772104286258> / <:lookingL:1159214410317967391> – looking right/left – hinting.
<:stare:1161771733829820426> – neutral stare – awkward or serious.
<:okk:1162013143845838961> – "ok" – neutral acknowledgment.
<:lulWut:1172251873131114526> – confused humor – "lol what?"
<:CAUGHT:1183915104669020331> – caught doing something.
<:sir:1206806163279052870> – polite / mock formality.
<:CHAD:1222721809393385585> – alpha / confident energy.
<:INSANITY:1223028653827297351> – shock / extreme reaction.
<:oh:1223683806998036620> – simple "oh" – realization or silence.`
}
