package gobot

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/disgoorg/disgolink/lavalink"
)

var CommandsHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate, b *Bot){
	"play":    playCommand,
	"leave":   leaveCommand,
	"skip":    skipCommand,
	"show":    showCommand,
	"setmode": setmodeCommand,
	"help":    helpCommand,
}

func playCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b *Bot) {
	Logger.Debug("Play command executed by: ", i.Member.User.ID)

	// Defer message since it may take some time to retrieve yt queries
	deferredResponse := singleInteractionResponse("Response will soon follow.", discordgo.InteractionResponseDeferredChannelMessageWithSource)
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
				response = singleFollowUpResponse("An error occured trying to play the track " + track.Info().Title + ". Please try again.")
			} else {
				// Initial response
				response = singleButtonFollowUpResponse("Adding the song to queue: "+query, "Link to your song :)", *track.Info().URI, "🤷")
			}
			if _, err := s.FollowupMessageCreate(i.Interaction, true, response); err != nil {
				Logger.Warn("Something went wrong when interacting with play command: ", err)
			}
		},
		func(playlist lavalink.AudioPlaylist) {
			// Directly queue playlist
			if err := b.Play(s, i, playlist.Tracks()...); err != nil {
				Logger.Warn("Error occured while trying to play single track: ", err)
				response = singleFollowUpResponse("An error occured trying to play the playlist " + playlist.Name() + ". Please try again.")
			} else {
				// Initial response
				response = singleButtonFollowUpResponse("Adding the song to queue: "+query, "Link to your playlist :)", query, "🤷")
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
			response := singleSelectMenuFollowUpResponse("Please choose a song from the menu.", "selectTrack", "Choose your desired youtube video 👇", options)
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
			response = singleButtonFollowUpResponse("No matches found for your query.", "Try again or something", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "🤷")
			if _, err := s.FollowupMessageCreate(i.Interaction, true, response); err != nil {
				Logger.Warn("Failed to create follow up message for empty query matches: ", err)
			}
		},
		func(ex lavalink.FriendlyException) {
			response = singleButtonFollowUpResponse("Error while loading your queried track.", "Try again or something", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "🤷")
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

		response = singleInteractionResponse("行ってきます、ご主人様", discordgo.InteractionResponseChannelMessageWithSource)

	} else {
		response = singleInteractionResponse("I'm not connected to any voice channel. Why are you trying to make me leave? :/",
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
		response = singleInteractionResponse("I'm not connected. Why would you do that? 😢", discordgo.InteractionResponseChannelMessageWithSource)
	} else if isPlaying {
		if err := b.skip(s, i.GuildID); err != nil {
			Logger.Warn("Bot was unable to skip the song: ", err)
			response = singleInteractionResponse("I failed to skip the song. 本当に御免なさい、ご主人様 😭", discordgo.InteractionResponseChannelMessageWithSource)
		} else {
			response = singleInteractionResponse("Skipping song. 🤫", discordgo.InteractionResponseChannelMessageWithSource)
		}
	} else {
		response = singleInteractionResponse("There are no songs to skip. Why would you do that? 😢", discordgo.InteractionResponseChannelMessageWithSource)
	}

	if err := s.InteractionRespond(i.Interaction, response); err != nil {
		Logger.Warn("Failed to create interaction response to skip command: ", err)
	}
}

func showCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b *Bot) {
	Logger.Debug("Show command executed by: ", i.Member.User.ID)

	var response *discordgo.WebhookParams
	// Defer message since it may take some time to retrieve the whole query
	deferredResponse := singleInteractionResponse("Response will soon follow.", discordgo.InteractionResponseDeferredChannelMessageWithSource)
	if err := s.InteractionRespond(i.Interaction, deferredResponse); err != nil {
		Logger.Warn("Failed to create deferred response: ", err)
	}

	if tracks, err := b.getTracks(i.GuildID); err != nil {
		Logger.Warn("Could not retrieve playlist: ", err)
		response = singleFollowUpResponse("An error occured trying to display playlist. Please try again and make sure the bot is connected.")
	} else if tracks != nil {
		var messageEmbedField []*discordgo.MessageEmbedField
		// TODO append playing track to playlist
		var displayNumber int
		if len(tracks) > 5 {
			displayNumber = 5
		} else {
			displayNumber = len(tracks)
		}

		for i := 0; i < displayNumber; i++ {
			messageEmbedField = append(messageEmbedField, &discordgo.MessageEmbedField{
				Name:   fmt.Sprintf("%v %d. song", NumberEmojiMap[i], i+1),
				Value:  tracks[i].Info().Title,
				Inline: false,
			})
		}

		response = singleEmbedFollowUpResponse("Songs in the playlist are listed in the following.", "Header of the playlist:", messageEmbedField)
	} else {
		response = singleFollowUpResponse("Playlist is empty.")
	}

	if _, err := s.FollowupMessageCreate(i.Interaction, true, response); err != nil {
		Logger.Warn("Failed to create interaction response to skip command: ", err)
	}
}

func setmodeCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b *Bot) {
	//TODO
}

func helpCommand(s *discordgo.Session, i *discordgo.InteractionCreate, b *Bot) {
	// TODO
}

func singleInteractionResponse(content string, interactionResponseType discordgo.InteractionResponseType) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: interactionResponseType,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   1 << 6,
		},
	}
}

func singleFollowUpResponse(content string) *discordgo.WebhookParams {
	return &discordgo.WebhookParams{
		Content: content,
		Flags:   1 << 6,
	}
}

func singleSelectMenuFollowUpResponse(content string, customID string, placeHolder string, options []discordgo.SelectMenuOption) *discordgo.WebhookParams {
	return &discordgo.WebhookParams{
		Content: content,
		Flags:   1 << 6,
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						CustomID:    customID,
						Placeholder: placeHolder,
						Options:     options,
					},
				},
			},
		},
	}
}

func singleEmbedFollowUpResponse(content string, title string, messageEmbedField []*discordgo.MessageEmbedField) *discordgo.WebhookParams {
	return &discordgo.WebhookParams{
		Content: content,
		Flags:   1 << 6,
		Embeds: []*discordgo.MessageEmbed{
			&discordgo.MessageEmbed{
				Title:  title,
				Fields: messageEmbedField,
			},
		},
	}
}

func singleButtonInteractionResponse(content string, buttonLabel string, url string, emojiName string, interactionResponseType discordgo.InteractionResponseType) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: interactionResponseType,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   1 << 6,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    buttonLabel,
							Style:    discordgo.LinkButton,
							Disabled: false,
							URL:      url,
							Emoji: discordgo.ComponentEmoji{
								Name: emojiName,
							},
						},
					},
				},
			},
		},
	}
}

func singleButtonFollowUpResponse(content string, buttonLabel string, url string, emojiName string) *discordgo.WebhookParams {
	return &discordgo.WebhookParams{
		Content: content,
		Flags:   1 << 6,
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    buttonLabel,
						Style:    discordgo.LinkButton,
						Disabled: false,
						URL:      url,
						Emoji: discordgo.ComponentEmoji{
							Name: emojiName,
						},
					},
				},
			},
		},
	}
}
