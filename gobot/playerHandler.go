package gobot

import (
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/disgoorg/disgolink/lavalink"
)

type PlayerManager struct {
	lavalink.PlayerEventAdapter
	Player        lavalink.Player
	Queue         []lavalink.AudioTrack
	QueueMu       sync.Mutex
	RepeatingMode RepeatingMode
	PlayerSession *discordgo.Session
}

type RepeatingMode int

const (
	RepeatingModeOff = iota
	RepeatingModeSong
	RepeatingModeQueue
)

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

func (m *PlayerManager) DeleteQueue() {
	m.QueueMu.Lock()
	defer m.QueueMu.Unlock()
	m.Queue = []lavalink.AudioTrack{}
}

func (m *PlayerManager) getAllTracks() []lavalink.AudioTrack {
	m.QueueMu.Lock()
	defer m.QueueMu.Unlock()
	if len(m.Queue) == 0 {
		return nil
	}
	return m.Queue
}

func (m *PlayerManager) OnWebSocketClosed(player lavalink.Player, code int, reason string, byRemote bool) {
	Logger.Info("Websocket to lavalink closed")
}

func (m *PlayerManager) OnTrackStart(player lavalink.Player, track lavalink.AudioTrack) {
	if err := m.PlayerSession.UpdateGameStatus(0, track.Info().Title); err != nil {
		Logger.Warn("Error updating status: ", err)
	}
}

func (m *PlayerManager) OnTrackEnd(player lavalink.Player, track lavalink.AudioTrack, endReason lavalink.AudioTrackEndReason) {
	Logger.Info("Track ended: ", track.Info().Title)

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

	//TODO May need to set playing track to nil

	if err := m.PlayerSession.UpdateGameStatus(0, ""); err != nil {
		Logger.Warn("Error updating status: ", err)
	}
}
