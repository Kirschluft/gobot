package gobot

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/disgoorg/disgolink/dgolink"
	"github.com/disgoorg/disgolink/lavalink"
	"github.com/disgoorg/snowflake"
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

	// Create discord session \w token
	dg, err := discordgo.New("Bot " + conf.DiscordToken)
	if err != nil {
		Logger.Fatal("Error creating discord session: ", err)
	}

	// Create bot and add listeners
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

	// Create slash commands
	bot.createCommands(dg)

	// Open socket connection, register nodes and silently fade away
	if err := dg.Open(); err != nil {
		Logger.Fatal("Error opening discord socket: ", err)
	}

	defer dg.Close()

	Logger.Debug("Initializing lavalink node.")
	bot.registerNode(conf)
	bot.Link.BestNode().ConfigureResuming(conf.ResumeKey, conf.ResumeTimeOut)
	Logger.Info("Bot is running.")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
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
	Logger.Debug("Entering play method")

	// Create new manager for guildID if not available
	manager, ok := b.PlayerManagers[guildID]
	Logger.Debug("Manager status: ", manager)
	if !ok {
		id, err := strconv.ParseInt(guildID, 10, 64)
		if err != nil {
			Logger.Warn("Could not convert guildID to int64")
		}

		manager = &PlayerManager{
			Player:        b.Link.Player(snowflake.ParseInt64(id)),
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
	Logger.Debug("Entering leave method")

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

	// Leave channel
	if err := s.ChannelVoiceJoinManual(guildID, "", false, false); err != nil {
		return err
	}

	if err := s.UpdateGameStatus(0, ""); err != nil {
		Logger.Warn("Error updating status: ", err)
	}
	return nil
}

func (b *Bot) skip(s *discordgo.Session, guildID string) error {
	Logger.Debug("Entering skip method")

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

func (b *Bot) registerNode(conf Configuration) {
	_, err := b.Link.AddNode(context.TODO(), lavalink.NodeConfig{
		Name:        conf.LavalinkNode,
		Host:        conf.LavalinkHost,
		Port:        conf.LavalinkPort,
		Password:    conf.LavalinkPW,
		Secure:      true,
		ResumingKey: conf.ResumeKey,
	})

	if err != nil {
		Logger.Fatal("Failed to initialized lavalink node.")
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
	// TODO bulk overwrite?
	// play command /w query parameter
	_, err := s.ApplicationCommandCreate(b.Link.UserID().String(), "", &discordgo.ApplicationCommand{
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
	})
	if err != nil {
		Logger.Fatal("Error occured while setting up play command: ", err)
	}

	// leave command
	_, err = s.ApplicationCommandCreate(b.Link.UserID().String(), "", &discordgo.ApplicationCommand{
		Name:        "leave",
		Description: "Leave the current voice channel.",
	},
	)
	if err != nil {
		Logger.Fatal("Error occured while setting up leave command: ", err)
	}

	// skip command
	_, err = s.ApplicationCommandCreate(b.Link.UserID().String(), "", &discordgo.ApplicationCommand{
		Name:        "skip",
		Description: "Skip the current song.",
	},
	)
	if err != nil {
		Logger.Fatal("Error occured while setting up leave command: ", err)
	}

	// playlist command
	_, err = s.ApplicationCommandCreate(b.Link.UserID().String(), "", &discordgo.ApplicationCommand{
		Name:        "show",
		Description: "Display the current playlist.",
	},
	)
	if err != nil {
		Logger.Fatal("Error occured while setting up show command: ", err)
	}
}
