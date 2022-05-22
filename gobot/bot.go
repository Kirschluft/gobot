package gobot

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/disgoorg/disgolink/dgolink"
	"github.com/disgoorg/disgolink/lavalink"
	"github.com/disgoorg/snowflake/v2"
)

var (
	urlPattern     = regexp.MustCompile("^https?://[-a-zA-Z0-9+&@#/%?=~_|!:,.;]*[-a-zA-Z0-9+&@#/%=~_|]?")
	NumberEmojiMap = map[int]string{
		1:  "1️⃣",
		2:  "2️⃣",
		3:  "3️⃣",
		4:  "4️⃣",
		5:  "5️⃣",
		6:  "6️⃣",
		7:  "7️⃣",
		8:  "8️⃣",
		9:  "9️⃣",
		10: "🔟",
	}
)

type Bot struct {
	Link           *dgolink.Link                             // Corresponding Link
	PlayerManagers map[string]*PlayerManager                 // available playermanager, maps guildid to manager
	TrackMap       map[string]map[string]lavalink.AudioTrack // maps query author and selected track id to track object
}

func StartBot(conf Configuration) {
	Logger.Info("Setting up discord bot session and lavalink node.")

	dg, err := discordgo.New("Bot " + conf.DiscordToken)
	if err != nil {
		Logger.Fatal("Error creating discord session: ", err)
	}

	bot := &Bot{
		Link:           dgolink.New(dg, lavalink.WithLogger(Logger)),
		PlayerManagers: map[string]*PlayerManager{},
		TrackMap:       map[string]map[string]lavalink.AudioTrack{},
	}

	Logger.Debug("Adding event handlers.")
	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		// Redirect to correct event handler
		switch i.Type {
		case discordgo.InteractionApplicationCommand:
			if h, ok := CommandsHandlers[i.ApplicationCommandData().Name]; ok {
				h(s, i, bot)
			}
		case discordgo.InteractionMessageComponent:

			if h, ok := ComponentsHandlers[i.MessageComponentData().CustomID]; ok {
				h(s, i, bot)
			}
		}
	})

	Logger.Debug("Creating and adding slash commands.")
	bot.createCommands(dg)

	// Open socket connection, register nodes and silently fade away
	if err := dg.Open(); err != nil {
		Logger.Fatal("Error opening discord socket: ", err)
	}

	defer dg.Close()

	Logger.Debug("Initializing lavalink node.")
	bot.registerNode(conf)
	bot.Link.BestNode().ConfigureResuming(conf.ResumeKey, conf.ResumeTimeOut)
	// TODO rejoin if resuming session ...

	Logger.Info("Bot is running.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
	Logger.Info("Shutting down bot due to syscalls or interupts: ", sc)
}

func (b *Bot) Play(s *discordgo.Session, i *discordgo.InteractionCreate, tracks ...lavalink.AudioTrack) error {
	// find voicestate of query user (and connect)
	voiceChannel, err := b.findChannelQueryUser(s, i, i.Member.User.ID)
	if err != nil {
		Logger.Warn("Could not find voice state of user: ", err)
	} else {
		Logger.Debug("User found in: ", voiceChannel)
	}

	if state, _ := s.State.VoiceState(i.GuildID, s.State.User.ID); state == nil && voiceChannel != nil {
		if err := s.ChannelVoiceJoinManual(i.GuildID, voiceChannel.ChannelID, false, false); err != nil {
			Logger.Warn("Could not join user voice channel: ", err)
			return errors.New("could not join voice state of user")
		}
	} else if voiceChannel == nil && state == nil {
		return errors.New("both user and bot are not in a voice channel")
	}

	return b.play(s, i.GuildID, tracks...)
}

func (b *Bot) play(s *discordgo.Session, guildID string, tracks ...lavalink.AudioTrack) error {
	// Create new manager for guildID if not available
	manager, ok := b.PlayerManagers[guildID]
	Logger.Debug("Manager status: ", manager)
	if !ok {
		schneeFlogge, err := snowflake.Parse(guildID)
		if err != nil {
			Logger.Warn("Could not convert guildID to int64")
		}

		manager = &PlayerManager{
			Player:        b.Link.Player(schneeFlogge),
			RepeatingMode: RepeatingModeOff,
			PlayerSession: s,
		}
		b.PlayerManagers[guildID] = manager
		manager.Player.AddListener(manager)
	}

	Logger.Debug("Player status: ", manager.Player)
	Logger.Debug("Player track: ", manager.Player.PlayingTrack())

	// Append to queue if queue is not empty; do not run play
	Logger.Debug("Adding tracks: ", tracks)
	Logger.Debug("Current queue: ", manager.Queue)
	Logger.Debug("Current playing song: ", manager.Player.PlayingTrack())
	if manager.PeekQueue() != nil || manager.isPlaying() {
		Logger.Debug("Returning after adding song to queue.")
		Logger.Debug("Manager playing status: ", manager.isPlaying())
		manager.AddQueue(tracks...)
		return nil
	}
	manager.AddQueue(tracks...)

	if track := manager.PopQueue(); track != nil {
		Logger.Debug("Next track: ", track)
		if err := manager.Player.Play(track); err != nil {
			return err
		}
	}

	return nil
}

func (b *Bot) leave(s *discordgo.Session, guildID string) error {
	// Leave channel
	if err := s.ChannelVoiceJoinManual(guildID, "", false, false); err != nil {
		return err
	}

	// Get rid of player and manager of player for this server
	manager, ok := b.PlayerManagers[guildID]
	if !ok {
		Logger.Warn("No player manager for guild available.")
		return errors.New("no player manager available. Connect the bot first")
	}

	// Appears to set the playing track to nil
	if err := manager.Player.Stop(); err != nil {
		return err
	}
	if err := manager.Player.Destroy(); err != nil {
		return err
	}
	delete(b.PlayerManagers, guildID)

	if err := s.UpdateGameStatus(0, ""); err != nil {
		Logger.Warn("Error updating status: ", err)
	}
	return nil
}

func (b *Bot) skip(s *discordgo.Session, guildID string) error {
	manager, ok := b.PlayerManagers[guildID]
	if !ok {
		Logger.Warn("No player manager for guild available.")
		return errors.New("no player manager available. Connect the bot first")
	}

	switch manager.RepeatingMode {
	case RepeatingModeOff:
		if nextTrack := manager.PopQueue(); nextTrack != nil {
			if err := manager.Player.Play(nextTrack); err != nil {
				Logger.Warn("Error playing next track: ", err)
				return err
			}
		} else {
			if err := manager.Player.Stop(); err != nil {
				Logger.Warn("Error stopping player: ", err)
				return err
			}
			if err := s.UpdateGameStatus(0, ""); err != nil {
				Logger.Warn("Error updating status: ", err)
			}
		}
	case RepeatingModeSong:
		if err := manager.Player.Play(manager.Player.PlayingTrack().Clone()); err != nil {
			Logger.Warn("Error playing next track: ", err)
			return err
		}

	case RepeatingModeQueue:
		manager.AddQueue(manager.Player.PlayingTrack().Clone())
		if nextTrack := manager.PopQueue(); nextTrack != nil {
			if err := manager.Player.Play(nextTrack); err != nil {
				Logger.Warn("Error playing next track: ", err)
				return err
			}
		}
	}

	return nil
}

func (b *Bot) IsQueueEmpty(guildID string) (bool, error) {
	manager, ok := b.PlayerManagers[guildID]
	if !ok {
		return true, errors.New("no player manager available. Connect the bot first")
	}

	if track := manager.PeekQueue(); track != nil {
		return false, nil
	}

	return true, nil
}

func (b *Bot) IsPlaying(guildID string) (bool, error) {
	manager, ok := b.PlayerManagers[guildID]
	if !ok {
		return false, errors.New("no player manager available. Connect the bot first")
	}

	return manager.isPlaying(), nil
}

func (b *Bot) getTracks(guildID string) ([]lavalink.AudioTrack, error) {
	manager, ok := b.PlayerManagers[guildID]
	if !ok {
		return nil, errors.New("no player manager available. Connect the bot first")
	}

	return manager.getAllTracks(), nil
}

func (b *Bot) setMode(guildID string, mode string) error {
	manager, ok := b.PlayerManagers[guildID]
	if !ok {
		return errors.New("no player manager available. Connect the bot first")
	}

	switch mode {
	case "off":
		manager.setMode(RepeatingModeOff)
	case "single":
		manager.setMode(RepeatingModeSong)
	case "all":
		manager.setMode(RepeatingModeQueue)
	default:
		return errors.New("entered unsupported mode")
	}

	return nil
}

func (b *Bot) seek(guildID string, position lavalink.Duration) error {
	manager, ok := b.PlayerManagers[guildID]
	if !ok {
		return errors.New("no player manager available. Connect the bot first")
	}

	Logger.Debug("Seeking position: ", position)
	if err := manager.Player.Seek(position); err != nil {
		return err
	}

	return nil
}

func (b *Bot) playingTrack(guildID string) (lavalink.AudioTrack, error) {
	manager, ok := b.PlayerManagers[guildID]
	if !ok {
		return nil, errors.New("no player manager available. Connect the bot first")
	}

	return manager.Player.PlayingTrack(), nil
}

func (b *Bot) currentPosition(guildID string) (lavalink.Duration, error) {
	manager, ok := b.PlayerManagers[guildID]
	if !ok {
		return lavalink.Duration(-1), errors.New("no player manager available. Connect the bot first")
	}

	return manager.Player.Position(), nil
}

func (b *Bot) registerNode(conf Configuration) {
	node, err := b.Link.AddNode(context.TODO(), lavalink.NodeConfig{
		Name:        conf.LavalinkNode,
		Host:        conf.LavalinkHost,
		Port:        conf.LavalinkPort,
		Password:    conf.LavalinkPW,
		Secure:      true,
		ResumingKey: conf.ResumeKey,
	})

	if err != nil {
		Logger.Fatal("Failed to initialized lavalink node: ", err)
	}
	if node == nil {
		Logger.Fatal("Cannot establish connection to lavalink node: ", node)
	}
}

func (b *Bot) findChannelQueryUser(s *discordgo.Session, i *discordgo.InteractionCreate, userID string) (*discordgo.VoiceState, error) {
	guild, err := s.State.Guild(i.GuildID)
	if err != nil {
		return nil, err
	}

	for _, vs := range guild.VoiceStates {
		if vs.UserID == userID {
			return vs, nil
		}
	}

	return nil, errors.New("could not find user's voice state")
}

func (b *Bot) createCommands(s *discordgo.Session) {
	// Register commands for all guilds
	// play command
	playCmd := discordgo.ApplicationCommand{
		Name:        "play",
		Description: "Play a query song.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "query",
				Description: "Song query that should be played.",
				Required:    true,
			},
		},
	}

	// leave command
	leaveCmd := discordgo.ApplicationCommand{
		Name:        "leave",
		Description: "Leave the current voice channel.",
	}

	// skip command
	skipCmd := discordgo.ApplicationCommand{
		Name:        "skip",
		Description: "Skip one or all songs.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "single",
				Description: "Skip the currently playing song.",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "all",
				Description: "Skip all songs in the playlist.",
				Required:    false,
			},
		},
	}

	// playlist command
	playlistCmd := discordgo.ApplicationCommand{
		Name:        "show",
		Description: "Display the current playlist.",
	}

	// play mode command
	setCmd := discordgo.ApplicationCommand{
		Name:        "set",
		Description: "Set the play mode.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "off",
				Description: "No play mode.",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "single",
				Description: "Single repeat mode.",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "all",
				Description: "All repeat mode.",
			},
		},
	}

	// seek command
	seekCmd := discordgo.ApplicationCommand{
		Name:        "seek",
		Description: "Jump to a position in a song (in seconds).",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "relative",
				Description: "Jump to relative position from current position (in seconds).",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionInteger,
						Name:        "position",
						Description: "Relative position (integer).",
						Required:    true,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "absolute",
				Description: "Jump to absolute position (in seconds).",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionInteger,
						Name:        "position",
						Description: "Absolute position (integer).",
						Required:    true,
					},
				},
			},
		},
	}

	allCmds := []*discordgo.ApplicationCommand{&playCmd, &leaveCmd, &skipCmd, &playlistCmd, &setCmd, &seekCmd}
	if _, err := s.ApplicationCommandBulkOverwrite(b.Link.UserID().String(), "", allCmds); err != nil {
		Logger.Panic("Failed to overwrite commands: ", err)
		// TODO may need to create commands if not created on server
	}
}
