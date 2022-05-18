package gobot

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"sync"
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
	PlayerManagers map[string]*PlayerManager                 // available playermanager
	TextChannel    discordgo.Channel                         //
	Guild          discordgo.Guild                           // guild that is served
	TrackMap       map[string]map[string]lavalink.AudioTrack // maps query author and selected track id to track object
}

type PlayerManager struct {
	lavalink.PlayerEventAdapter
	Player        lavalink.Player
	Queue         []lavalink.AudioTrack
	QueueMu       sync.Mutex
	RepeatingMode RepeatingMode
	PlayerSession discordgo.Session
}

type RepeatingMode int

const (
	RepeatingModeOff = iota
	RepeatingModeSong
	RepeatingModeQueue
)

func StartBot(conf Configuration) {
	Logger.Info("Setting up discord bot session and lavalink node.")

	// Create discord session \w token
	dg, err := discordgo.New("Bot " + conf.DiscordToken)
	if err != nil {
		Logger.Fatal("Error creating discord session: ", err)
	}

	// Check if guild and voice channel match
	guild, err := dg.Guild(conf.Guild)
	if err != nil {
		Logger.Fatal("Could not find guild with ID: ", conf.Guild)
	}

	channel, err := dg.Channel(conf.TextChannel)
	if err != nil {
		Logger.Fatal("Could not find channel with ID: ", conf.TextChannel)
	}

	if channel.GuildID != guild.ID {
		Logger.Fatal("Guild of text channel is not the same as the guild specified: ", channel.GuildID, " != ", guild.ID)
	}

	// Create bot and add listeners
	bot := &Bot{
		Link:           dgolink.New(dg),
		PlayerManagers: map[string]*PlayerManager{},
		TextChannel:    *channel,
		Guild:          *guild,
		TrackMap:       make(map[string]map[string]lavalink.AudioTrack),
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

func (m *PlayerManager) AddQueue(tracks ...lavalink.AudioTrack) {
	m.QueueMu.Lock()
	defer m.QueueMu.Unlock()
	m.Queue = append(m.Queue, tracks...)
}

func (m *PlayerManager) PopQueue() lavalink.AudioTrack {
	m.QueueMu.Lock()
	defer m.QueueMu.Unlock()
	if len(m.Queue) == 0 {
		return nil
	}
	var track lavalink.AudioTrack
	track, m.Queue = m.Queue[0], m.Queue[1:]
	return track
}

func (m *PlayerManager) PeekQueue() lavalink.AudioTrack {
	m.QueueMu.Lock()
	defer m.QueueMu.Unlock()
	if len(m.Queue) == 0 {
		return nil
	}
	return m.Queue[0]
}

func (m *PlayerManager) getAllTracks() []lavalink.AudioTrack {
	if len(m.Queue) == 0 {
		return nil
	}
	return m.Queue
}

func (m *PlayerManager) OnWebSocketClosed(player lavalink.Player, code int, reason string, byRemote bool) {
	Logger.Fatal("Fuck you")
}

func (m *PlayerManager) OnTrackStart(player lavalink.Player, track lavalink.AudioTrack) {
	m.PlayerSession.UpdateGameStatus(0, track.Info().Title)
}

func (m *PlayerManager) OnTrackEnd(player lavalink.Player, track lavalink.AudioTrack, endReason lavalink.AudioTrackEndReason) {
	if !endReason.MayStartNext() {
		return
	}
	switch m.RepeatingMode {
	case RepeatingModeOff:
		if nextTrack := m.PopQueue(); nextTrack != nil {
			if err := player.Play(nextTrack); err != nil {
				Logger.Warn("Error playing next track: ", err)
			}
		}
	case RepeatingModeSong:
		if err := player.Play(track.Clone()); err != nil {
			Logger.Warn("Error playing next track: ", err)
		}

	case RepeatingModeQueue:
		m.AddQueue(track)
		if nextTrack := m.PopQueue(); nextTrack != nil {
			if err := player.Play(nextTrack); err != nil {
				Logger.Warn("Error playing next track: ", err)
			}
		}
	}

	player.Stop()
	m.PlayerSession.UpdateGameStatus(0, "")
}

func (b *Bot) Play(s *discordgo.Session, i *discordgo.InteractionCreate, tracks ...lavalink.AudioTrack) error {
	// find voicestate of query user
	voiceChannel, err := b.findChannelQueryUser(s, i, i.Member.User.ID)
	if err != nil {
		Logger.Warn("Could not find user: ", err)
	}

	Logger.Debug("User found in: ", voiceChannel)

	return b.play(s, i.GuildID, voiceChannel.ChannelID, tracks...)
}

func (b *Bot) play(s *discordgo.Session, guildID string, voiceChannelID string, tracks ...lavalink.AudioTrack) error {
	if err := s.ChannelVoiceJoinManual(guildID, voiceChannelID, false, false); err != nil {
		return err
	}

	id, err := strconv.ParseInt(guildID, 10, 64)
	if err != nil {
		Logger.Warn("Could not convert guildID to int64")
	}

	manager, ok := b.PlayerManagers[guildID]
	if !ok {
		manager = &PlayerManager{
			Player:        b.Link.Player(snowflake.ParseInt64(id)),
			RepeatingMode: RepeatingModeOff,
			PlayerSession: *s,
		}
		b.PlayerManagers[guildID] = manager
		manager.Player.AddListener(manager)
	}

	// Append to queue if queue is not empty; do not run play
	Logger.Debug("Adding tracks: ", tracks)
	if manager.PeekQueue() != nil || manager.Player.PlayingTrack() != nil {
		manager.AddQueue(tracks...)
		return nil
	}
	manager.AddQueue(tracks...)

	if track := manager.PopQueue(); track != nil {
		Logger.Debug("Next track: ", track)
		if err := manager.Player.Play(track); err != nil {
			return err
		}
		s.UpdateGameStatus(0, track.Info().Title)
	}

	return nil
}

func (b *Bot) leave(s *discordgo.Session, guildID string) error {
	if err := s.ChannelVoiceJoinManual(guildID, "", false, false); err != nil {
		return err
	}

	return nil
}

func (b *Bot) skip(s *discordgo.Session, guildID string) error {
	manager, ok := b.PlayerManagers[guildID]
	if !ok {
		Logger.Warn("No player manager for guild available.")
		return errors.New("no player manager available. Connect the bot first")
	}

	//TODO test player.stop
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

func (b *Bot) createCommands(dg *discordgo.Session) {
	// play command /w query parameter
	_, err := dg.ApplicationCommandCreate(b.Link.UserID().String(), b.Guild.ID, &discordgo.ApplicationCommand{
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
	_, err = dg.ApplicationCommandCreate(b.Link.UserID().String(), b.Guild.ID, &discordgo.ApplicationCommand{
		Name:        "leave",
		Description: "Leave the current voice channel.",
	},
	)
	if err != nil {
		Logger.Fatal("Error occured while setting up leave command: ", err)
	}

	// skip command
	_, err = dg.ApplicationCommandCreate(b.Link.UserID().String(), b.Guild.ID, &discordgo.ApplicationCommand{
		Name:        "skip",
		Description: "Skip the current song.",
	},
	)
	if err != nil {
		Logger.Fatal("Error occured while setting up leave command: ", err)
	}
}
