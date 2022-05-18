package gobot

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

var ComponentsHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate, b *Bot){
	"selectTrack": func(s *discordgo.Session, i *discordgo.InteractionCreate, b *Bot) {
		// get user and track IDs
		var response *discordgo.InteractionResponse
		data := i.MessageComponentData()
		var trackID = data.Values[0]
		var userID = i.Member.User.ID
		Logger.Info("Looking for track with ID ", trackID, " queried by user ", userID)

		// retrieve the corresponding track from the TrackMap and respond to interaction
		query := b.TrackMap[userID]
		if query == nil {
			Logger.Warn("User ", userID, " is not registered in the track map.")
			response = &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Could not find queries for the user. Please try a different query.",
					Flags:   1 << 6,
				},
			}
		} else {
			track := query[trackID]
			if track == nil {
				Logger.Warn("Track with ID ", trackID, " not found for the user ", userID)
				response = &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Could not find tracks for the user. Please try a different query",
						Flags:   1 << 6,
					},
				}
			} else {
				if err := b.Play(s, i, track); err != nil {
					Logger.Warn("Something went wrong when trying to play chosen single-track: ", err)
					response = &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: "Could not query track. Please try a different query",
							Flags:   1 << 6,
						},
					}
				} else {
					if err != nil {
						Logger.Info("An error occured while trying to edit interaction: ", err)
					}

					response = &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseUpdateMessage,
						Data: &discordgo.InteractionResponseData{
							Content: fmt.Sprintf("Querying the track: %v", track.Info().Title),
							Components: []discordgo.MessageComponent{
								discordgo.ActionsRow{
									Components: []discordgo.MessageComponent{
										discordgo.Button{
											Emoji: discordgo.ComponentEmoji{
												Name: "🙈",
											},

											Label: "Click here for the link",
											Style: discordgo.LinkButton,
											URL:   *track.Info().URI,
										},
									},
								},
							},
						},
					}
				}
			}
		}

		if err := s.InteractionRespond(i.Interaction, response); err != nil {
			Logger.Warn("Failed to create single-track interaction response: ", err)
		}
	},
}
