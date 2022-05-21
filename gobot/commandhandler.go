package gobot

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/disgoorg/disgolink/lavalink"
)

var CommandsHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate, b *Bot){
	"play":  playCommand,
	"leave": leaveCommand,
	"skip":  skipCommand, // TODO skip all
	"show":  showCommand,
	"set":   setCommand,
	"seek":  seekCommand,
	"help":  helpCommand,
}

func playCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b *Bot) {
	Logger.Debug("Play command executed by: ", i.Member.User.ID)

	// Defer message since it may take some time to retrieve yt queries
	deferredResponse := SingleInteractionResponse("Response will soon follow.", discordgo.InteractionResponseDeferredChannelMessageWithSource)
	if err := s.InteractionRespond(i.Interaction, deferredResponse); err != nil {
		Logger.Warn("Failed to create deferred response: ", err)
	}

	// Get input string from play command
	query := fmt.Sprintf("%v", i.ApplicationCommandData().Options[0].Value)
	Logger.Debug("Song queried: ", query)

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
			if err := b.Play(s, i, track); err != nil {
				Logger.Warn("Error occured while trying to play single track: ", err)
				response = SingleFollowUpResponse("An error occured trying to play the track " + track.Info().Title + ". Please try again.")
			} else {
				// Initial response
				response = SingleButtonFollowUpResponse("Adding the song to queue: "+query, "Link to your song :)", *track.Info().URI, "🤷")
			}
			if _, err := s.FollowupMessageCreate(i.Interaction, true, response); err != nil {
				Logger.Warn("Something went wrong when interacting with play command: ", err)
			}
		},
		func(playlist lavalink.AudioPlaylist) {
			// Directly queue playlist
			if err := b.Play(s, i, playlist.Tracks()...); err != nil {
				Logger.Warn("Error occured while trying to play single track: ", err)
				response = SingleFollowUpResponse("An error occured trying to play the playlist " + playlist.Name() + ". Please try again.")
			} else {
				// Initial response
				response = SingleButtonFollowUpResponse("Adding the song to queue: "+query, "Link to your playlist :)", query, "🤷")
			}
			if _, err := s.FollowupMessageCreate(i.Interaction, true, response); err != nil {
				Logger.Warn("Something went wrong when interacting with play command: ", err)
			}
		},
		func(tracks []lavalink.AudioTrack) {
			// Give user yt search options to choose from ...
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
				Logger.Warn("Failed to create interaction menu for yt search: ", err)
			} else {
				var currentTrackMap = make(map[string]lavalink.AudioTrack)
				for i := 0; i < 5; i++ {
					currentTrackMap[tracks[i].Info().Identifier] = tracks[i]
				}
				b.TrackMap[i.Member.User.ID] = currentTrackMap
			}
		},
		func() {
			response = SingleButtonFollowUpResponse("No matches found for your query.", "Try again or something", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "🤷")
			if _, err := s.FollowupMessageCreate(i.Interaction, true, response); err != nil {
				Logger.Warn("Failed to create follow up message for empty query matches: ", err)
			}
		},
		func(ex lavalink.FriendlyException) {
			response = SingleButtonFollowUpResponse("Error while loading your queried track.", "Try again or something", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "🤷")
			if _, err := s.FollowupMessageCreate(i.Interaction, true, response); err != nil {
				Logger.Warn("Failed to create follow up message for query ", err)
			}
			Logger.Warn("Query exception: ", ex)
		},
	))
}

func leaveCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b *Bot) {
	Logger.Debug("Leave command executed by: ", i.Member.User.ID)

	var response *discordgo.InteractionResponse
	// Check if bot is connected to a voice channel
	if state, _ := s.State.VoiceState(i.GuildID, s.State.User.ID); state != nil {
		if err := b.leave(s, i.GuildID); err != nil {
			Logger.Warn("Bot was unable to leave voice channel: ", err)
		}

		response = SingleInteractionResponse("行ってきます、ご主人様", discordgo.InteractionResponseChannelMessageWithSource)

	} else {
		response = SingleInteractionResponse("I'm not connected to any voice channel. Why are you trying to make me leave? :/",
			discordgo.InteractionResponseChannelMessageWithSource)
	}

	if err := s.InteractionRespond(i.Interaction, response); err != nil {
		Logger.Warn("Failed to create interaction response to leave command: ", err)
	}
}

func skipCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b *Bot) {
	Logger.Debug("Skip command executed by: ", i.Member.User.ID)

	var response *discordgo.InteractionResponse
	if isPlaying, err := b.IsPlaying(i.GuildID); err != nil {
		Logger.Warn("An error occured checking if a song is playing: ", err)
		response = SingleInteractionResponse("I'm not connected. Why would you do that? 😢", discordgo.InteractionResponseChannelMessageWithSource)
	} else if isPlaying {
		if err := b.skip(s, i.GuildID); err != nil {
			Logger.Warn("Bot was unable to skip the song: ", err)
			response = SingleInteractionResponse("I failed to skip the song. 本当に御免なさい、ご主人様 😭", discordgo.InteractionResponseChannelMessageWithSource)
		} else {
			response = SingleInteractionResponse("Skipping song. 🤫", discordgo.InteractionResponseChannelMessageWithSource)
		}
	} else {
		response = SingleInteractionResponse("There are no songs to skip. Why would you do that? 😢", discordgo.InteractionResponseChannelMessageWithSource)
	}

	if err := s.InteractionRespond(i.Interaction, response); err != nil {
		Logger.Warn("Failed to create interaction response to skip command: ", err)
	}
}

func showCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b *Bot) {
	Logger.Debug("Show command executed by: ", i.Member.User.ID)

	var response *discordgo.WebhookParams
	// Defer message since it may take some time to retrieve the whole query
	deferredResponse := SingleInteractionResponse("Response will soon follow.", discordgo.InteractionResponseDeferredChannelMessageWithSource)
	if err := s.InteractionRespond(i.Interaction, deferredResponse); err != nil {
		Logger.Warn("Failed to create deferred response: ", err)
	}

	if tracks, err := b.getTracks(i.GuildID); err != nil {
		Logger.Warn("Could not retrieve playlist: ", err)
		response = SingleFollowUpResponse("An error occured trying to display playlist. Please try again and make sure the bot is connected.")
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
		Logger.Warn("Failed to create interaction response to skip command: ", err)
	}
}

func setCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b *Bot) {
	Logger.Debug("Set command executed by: ", i.Member.User.ID)

	var response *discordgo.InteractionResponse
	mode := i.ApplicationCommandData().Options[0].Name
	if err := b.setMode(i.GuildID, mode); err != nil {
		Logger.Warn("Unable to set play mode: ", err)
		response = SingleInteractionResponse("Unable to set play mode. Please try again and use one of the available modes (off, single, all).",
			discordgo.InteractionResponseChannelMessageWithSource)
	} else {
		response = SingleInteractionResponse(fmt.Sprintf("Set mode to: %v", mode),
			discordgo.InteractionResponseChannelMessageWithSource)
	}

	if err := s.InteractionRespond(i.Interaction, response); err != nil {
		Logger.Warn("Failed to create interaction response to set command: ", err)
	}
}

func seekCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b *Bot) {
	Logger.Debug("Seek command executed by: ", i.Member.User.ID)
	s.InteractionRespond(i.Interaction, SingleInteractionResponse("Unsupported", discordgo.InteractionResponseChannelMessageWithSource))
	// TODO
}

func helpCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b *Bot) {
	Logger.Debug("Help command executed by: ", i.Member.User.ID)
	s.InteractionRespond(i.Interaction, SingleInteractionResponse("Unsupported", discordgo.InteractionResponseChannelMessageWithSource))
	// TODO
}
