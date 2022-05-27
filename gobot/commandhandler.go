package gobot

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/disgoorg/disgolink/lavalink"
	"github.com/sirupsen/logrus"
)

var CommandsHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate, b *Bot){
	"play":  playCommand,
	"leave": leaveCommand,
	"skip":  skipCommand,
	"show":  showCommand,
	"set":   setCommand,
	"seek":  seekCommand,
}

func playCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b *Bot) {
	// Get input string from play command
	data := i.ApplicationCommandData().Options[0]
	if data == nil {
		Logger.Warn("Expected user query but options are empty. Make sure the commands are set up properly.")
		response := SingleInteractionResponse("An error occurred. The bot command appears to be set up incorrectly. Please try again later.",
			discordgo.InteractionResponseChannelMessageWithSource)
		if err := s.InteractionRespond(i.Interaction, response); err != nil {
			Logger.Warn("Failed to create interaction response: ", err)
		}
		return
	}
	query := fmt.Sprintf("%v", data.Value)

	playLogger := Logger.WithFields(logrus.Fields{
		"cmd":     "play",
		"userID":  i.Member.User.ID,
		"guildID": i.GuildID,
		"query":   query,
	})
	playLogger.Info("Play command selected.")

	// Defer message since it may take some time to retrieve yt queries
	deferredResponse := SingleInteractionResponse("Response will soon follow.", discordgo.InteractionResponseDeferredChannelMessageWithSource)
	if err := s.InteractionRespond(i.Interaction, deferredResponse); err != nil {
		playLogger.Warn("Failed to create deferred response: ", err)
	}

	// If query is url, add ytsearch for lavalink
	if !urlPattern.MatchString(query) {
		query = "ytsearch:" + query
	}

	//TODO filter for length > 100

	var response *discordgo.WebhookParams
	// Handle different return values from lavalink and play track(s) ...
	_ = b.Link.BestRestClient().LoadItemHandler(context.TODO(), query, lavalink.NewResultHandler(
		func(track lavalink.AudioTrack) {
			// Directly queue track if it is a single track
			playLogger.Debug("Single audio track is returned by lavalink.")
			if err := b.Play(s, i, track); err != nil {
				playLogger.Warn("Error occurred while trying to play single track: ", err)
				response = SingleFollowUpResponse("An error occurred trying to play the track " + track.Info().Title + ". Please try again.")
			} else {
				// Initial response
				response = SingleButtonFollowUpResponse("Adding the song to queue: "+query, "Link to your song :)", *track.Info().URI, "🤷")
			}
			if _, err := s.FollowupMessageCreate(i.Interaction, true, response); err != nil {
				playLogger.Warn("Something went wrong when interacting with play command: ", err)
			}
		},
		func(playlist lavalink.AudioPlaylist) {
			// Directly queue playlist
			playLogger.Debug("Playlist is returned by lavalink.")
			if err := b.Play(s, i, playlist.Tracks()...); err != nil {
				playLogger.Warn("Error occurred while trying to play single track: ", err)
				response = SingleFollowUpResponse("An error occurred trying to play the playlist " + playlist.Name() + ". Please try again.")
			} else {
				// Initial response
				response = SingleButtonFollowUpResponse("Adding the song to queue: "+query, "Link to your playlist :)", query, "🤷")
			}
			if _, err := s.FollowupMessageCreate(i.Interaction, true, response); err != nil {
				playLogger.Warn("Something went wrong when interacting with play command: ", err)
			}
		},
		func(tracks []lavalink.AudioTrack) {
			// Give user yt search options to choose from ...
			playLogger.Debug("Multiple tracks are returned by lavalink.")
			var options []discordgo.SelectMenuOption
			for i := 0; i < 5; i++ {
				options = append(options, discordgo.SelectMenuOption{
					Label: tracks[i].Info().Title,
					Value: tracks[i].Info().Identifier,
					Emoji: discordgo.ComponentEmoji{
						Name: NumberEmojiMap[i+1],
					},
					Default: false,
					// Description: fmt.Sprintf("yt search result number %d", i),
				})
			}

			// Follow up on the deferred message
			response := SingleSelectMenuFollowUpResponse("Please choose a song from the menu.", "selectTrack", "Choose your desired youtube video 👇", options)
			if _, err := s.FollowupMessageCreate(i.Interaction, true, response); err != nil {
				playLogger.Warn("Failed to create interaction menu for yt search: ", err)
			} else {
				var currentTrackMap = make(map[string]lavalink.AudioTrack)
				for i := 0; i < 5; i++ {
					currentTrackMap[tracks[i].Info().Identifier] = tracks[i]
				}
				b.TrackMap[i.Member.User.ID] = currentTrackMap
			}
		},
		func() {
			playLogger.Debug("Lavalink did not return any search results.")
			response = SingleButtonFollowUpResponse("No matches found for your query.", "Try again or something", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "🤷")
			if _, err := s.FollowupMessageCreate(i.Interaction, true, response); err != nil {
				playLogger.Warn("Failed to create follow up message for empty query matches: ", err)
			}
		},
		func(ex lavalink.FriendlyException) {
			playLogger.Warn("Lavalink query exception: ", ex)
			response = SingleButtonFollowUpResponse("Error while loading your queried track.", "Try again or something", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "🤷")
			if _, err := s.FollowupMessageCreate(i.Interaction, true, response); err != nil {
				playLogger.Warn("Failed to create follow up message for query ", err)
			}
		},
	))
}

func leaveCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b *Bot) {
	leaveLogger := Logger.WithFields(logrus.Fields{
		"cmd":     "leave",
		"userID":  i.Member.User.ID,
		"guildID": i.GuildID,
	})
	leaveLogger.Info("Leave command selected.")

	var response *discordgo.InteractionResponse
	// Check if bot is connected to a voice channel
	if state, _ := s.State.VoiceState(i.GuildID, s.State.User.ID); state != nil {
		if err := b.leave(s, i.GuildID); err != nil {
			leaveLogger.Warn("Bot was unable to leave voice channel: ", err)
		}

		response = SingleInteractionResponse("行ってきます、ご主人様", discordgo.InteractionResponseChannelMessageWithSource)

	} else {
		response = SingleInteractionResponse("I'm not connected to any voice channel. Why are you trying to make me leave? :/",
			discordgo.InteractionResponseChannelMessageWithSource)
	}

	if err := s.InteractionRespond(i.Interaction, response); err != nil {
		leaveLogger.Warn("Failed to create interaction response: ", err)
	}
}

func skipCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b *Bot) {
	// Get input string from skip command
	data := i.ApplicationCommandData().Options[0]
	if data == nil {
		Logger.Warn("Expected user query but options are empty. Make sure the commands are set up properly.")
		response := SingleInteractionResponse("An error occurred. The bot command appears to be set up incorrectly. Please try again later.",
			discordgo.InteractionResponseChannelMessageWithSource)
		if err := s.InteractionRespond(i.Interaction, response); err != nil {
			Logger.Warn("Failed to create interaction response: ", err)
		}
		return
	}
	query := fmt.Sprintf("%v", data.Name)

	skipLogger := Logger.WithFields(logrus.Fields{
		"cmd":     "skip",
		"userID":  i.Member.User.ID,
		"guildID": i.GuildID,
		"query":   query,
	})
	skipLogger.Info("Skip command selected.")

	var response *discordgo.InteractionResponse
	if isPlaying, err := b.IsPlaying(i.GuildID); err != nil {
		skipLogger.Warn("An error occurred checking if a song is playing: ", err)
		response = SingleInteractionResponse("I'm not connected. Why would you do that? 😢", discordgo.InteractionResponseChannelMessageWithSource)
	} else if isPlaying {
		switch query {
		case "all":
			if err := b.purgeQueue(i.GuildID); err != nil {
				skipLogger.Warn("Bot was unable to purge queue: ", err)
			}
		case "single":
			break
		default:
			if err := s.InteractionRespond(i.Interaction, SingleInteractionResponse("Unsupported seek option. How did you get here?",
				discordgo.InteractionResponseChannelMessageWithSource)); err != nil {
				skipLogger.Warn("Failed to create interaction response: ", err)
			}
			return
		}
		if err := b.skip(s, i.GuildID); err != nil {
			skipLogger.Warn("Bot was unable to skip the song: ", err)
			response = SingleInteractionResponse("I failed to skip the song. 本当に御免なさい、ご主人様 😭", discordgo.InteractionResponseChannelMessageWithSource)
		} else {
			response = SingleInteractionResponse("Skipping song(s). 🤫", discordgo.InteractionResponseChannelMessageWithSource)
		}
	} else {
		response = SingleInteractionResponse("There are no songs to skip. Why would you do that? 😢", discordgo.InteractionResponseChannelMessageWithSource)
	}

	if err := s.InteractionRespond(i.Interaction, response); err != nil {
		skipLogger.Warn("Failed to create interaction response: ", err)
	}
}

func showCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b *Bot) {
	showLogger := Logger.WithFields(logrus.Fields{
		"cmd":     "show",
		"userID":  i.Member.User.ID,
		"guildID": i.GuildID,
	})
	showLogger.Info("Show command selected.")

	var response *discordgo.WebhookParams
	// Defer message since it may take some time to retrieve the whole query
	deferredResponse := SingleInteractionResponse("Response will soon follow.", discordgo.InteractionResponseDeferredChannelMessageWithSource)
	if err := s.InteractionRespond(i.Interaction, deferredResponse); err != nil {
		showLogger.Warn("Failed to create deferred response: ", err)
	}

	if tracks, err := b.getTracks(i.GuildID); err != nil {
		showLogger.Warn("Could not retrieve playlist: ", err)
		response = SingleFollowUpResponse("An error occurred trying to display playlist. Please try again and make sure the bot is connected.")
	} else if tracks != nil {
		var messageEmbedField []*discordgo.MessageEmbedField
		// TODO make embeds nicer
		// TODO prepend playing track to playlist as other embed
		var displayNumber int
		if len(tracks) > 5 {
			displayNumber = 5
		} else {
			displayNumber = len(tracks)
		}

		for i := 0; i < displayNumber; i++ {
			messageEmbedField = append(messageEmbedField, &discordgo.MessageEmbedField{
				Name:   NumberEmojiMap[i+1],
				Value:  tracks[i].Info().Title,
				Inline: false,
			})
		}

		response = SingleEmbedFollowUpResponse("Songs in the playlist are listed in the following.", "Header of the playlist:", messageEmbedField)
	} else {
		response = SingleFollowUpResponse("Playlist is empty.")
	}

	if _, err := s.FollowupMessageCreate(i.Interaction, true, response); err != nil {
		showLogger.Warn("Failed to create interaction response: ", err)
	}
}

func setCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b *Bot) {
	setLogger := Logger.WithFields(logrus.Fields{
		"cmd":     "set",
		"userID":  i.Member.User.ID,
		"guildID": i.GuildID,
	})
	setLogger.Info("Set command selected.")

	var response *discordgo.InteractionResponse
	mode := i.ApplicationCommandData().Options[0].Name
	if err := b.setMode(i.GuildID, mode); err != nil {
		setLogger.Warn("Unable to set play mode: ", err)
		response = SingleInteractionResponse("Unable to set play mode. Please try again and use one of the available modes (off, single, all).",
			discordgo.InteractionResponseChannelMessageWithSource)
	} else {
		response = SingleInteractionResponse(fmt.Sprintf("Set mode to: %v", mode),
			discordgo.InteractionResponseChannelMessageWithSource)
	}

	if err := s.InteractionRespond(i.Interaction, response); err != nil {
		setLogger.Warn("Failed to create interaction response: ", err)
	}
}

func seekCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b *Bot) {
	// Get parameter name and position from seek command
	data := i.ApplicationCommandData().Options[0]
	if data == nil {
		Logger.Warn("Expected user query but options are empty. Make sure the commands are set up properly.")
		response := SingleInteractionResponse("An error occurred. The bot command appears to be set up incorrectly. Please try again later.",
			discordgo.InteractionResponseChannelMessageWithSource)
		if err := s.InteractionRespond(i.Interaction, response); err != nil {
			Logger.Warn("Failed to create interaction response: ", err)
		}
		return
	}
	query := fmt.Sprintf("%v", data.Name)
	position := data.Options[0].IntValue()

	seekLogger := Logger.WithFields(logrus.Fields{
		"cmd":      "seek",
		"userID":   i.Member.User.ID,
		"guildID":  i.GuildID,
		"query":    query,
		"position": position,
	})
	seekLogger.Info("Seek command selected.")

	var response *discordgo.InteractionResponse
	if isPlaying, err := b.IsPlaying(i.GuildID); err != nil {
		seekLogger.Warn("An error occurred checking if a song is playing: ", err)
		response = SingleInteractionResponse("I'm not connected. Why would you do that? 😢", discordgo.InteractionResponseChannelMessageWithSource)
	} else if isPlaying {
		response = seekHelper(query, b, i.GuildID, position)
	} else {
		response = SingleInteractionResponse("There are no songs available. Why would you do that? 😢", discordgo.InteractionResponseChannelMessageWithSource)
	}

	if err := s.InteractionRespond(i.Interaction, response); err != nil {
		seekLogger.Warn("Failed to create interaction response to seek command: ", err)
	}
}

func seekHelper(query string, b *Bot, guildID string, position int64) *discordgo.InteractionResponse {
	// Check if player is playing a track and retrieve it
	isPlaying, err := b.IsPlaying(guildID)
	if err != nil {
		Logger.Warn("Error trying to check if player is playing a track: ", err)
		return SingleInteractionResponse("Unable to check if a track is playing.", discordgo.InteractionResponseChannelMessageWithSource)
	} else if !isPlaying {
		Logger.Warn("Seek command called when no playing track available.")
		return SingleInteractionResponse("No track is playing.", discordgo.InteractionResponseChannelMessageWithSource)
	}

	playingTrack, err := b.playingTrack(guildID)
	if err != nil || playingTrack == nil {
		Logger.Warn("Error trying to retrieve playing track ", playingTrack, " : ", err)
		return SingleInteractionResponse("No track is playing.", discordgo.InteractionResponseChannelMessageWithSource)
	}

	switch query {
	case "absolute":
		return seekAbsolute(b, guildID, position, playingTrack)
	case "relative":
		return seekRelative(b, guildID, position, playingTrack)
	default:
		return SingleInteractionResponse("Unsupported seek option. How did you get here?",
			discordgo.InteractionResponseChannelMessageWithSource)
	}
}

func seekAbsolute(b *Bot, guildID string, position int64, playingTrack lavalink.AudioTrack) *discordgo.InteractionResponse {
	songDuration := playingTrack.Info().Length

	// Skip to the end if position is greater than duration of song
	// Skip to the start if the position is negative
	if position > songDuration.Seconds() {
		position = songDuration.Seconds()
	} else if position < 0 {
		position = 0
	}

	if err := b.seek(guildID, lavalink.Duration(position*1000)); err != nil {
		Logger.Warn("Bot was unable to seek position: ", err)
		return SingleInteractionResponse(fmt.Sprintf("I failed to seek absolute position %d in the song. 本当に御免なさい、ご主人様 😭", position),
			discordgo.InteractionResponseChannelMessageWithSource)
	}

	return SingleInteractionResponse(fmt.Sprintf("Seeking absolute position %d in song. 🤫", position),
		discordgo.InteractionResponseChannelMessageWithSource)
}

func seekRelative(b *Bot, guildID string, position int64, playingTrack lavalink.AudioTrack) *discordgo.InteractionResponse {
	songPosition, err := b.currentPosition(guildID)

	if err != nil {
		Logger.Warn("Bot was unable to retrieve the current position of the player: ", err)
		return SingleInteractionResponse("I failed to seek relative position in the song. 本当に御免なさい、ご主人様 😭",
			discordgo.InteractionResponseChannelMessageWithSource)
	} else if songPosition == -1 {
		Logger.Warn("Bot was unable to retrieve the current position of the player. Bot appears to not be connected.")
		return SingleInteractionResponse("I failed to seek relative position in the song. 本当に御免なさい、ご主人様 😭",
			discordgo.InteractionResponseChannelMessageWithSource)
	}

	seekPosition := songPosition.Seconds() + position

	if seekPosition < 0 {
		seekPosition = 0
	} else if seekPosition > playingTrack.Info().Length.Seconds() {
		seekPosition = playingTrack.Info().Length.Seconds()
	}

	if err = b.seek(guildID, lavalink.Duration(seekPosition*1000)); err != nil {
		Logger.Warn("Bot was unable to seek position: ", err)
		return SingleInteractionResponse(fmt.Sprintf("I failed to seek relative position %d in the song. 本当に御免なさい、ご主人様 😭", seekPosition),
			discordgo.InteractionResponseChannelMessageWithSource)
	}

	return SingleInteractionResponse(fmt.Sprintf("Seeking relative position %d in song. 🤫", seekPosition),
		discordgo.InteractionResponseChannelMessageWithSource)
}
