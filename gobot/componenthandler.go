package gobot

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

var ComponentsHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate, b *Bot){
	"selectTrack": func(s *discordgo.Session, i *discordgo.InteractionCreate, b *Bot) {
		// Get user and track IDs
		var response *discordgo.InteractionResponse
		data := i.MessageComponentData()
		var trackID = data.Values[0]
		var userID = i.Member.User.ID
		Logger.Info("Looking for track with ID ", trackID, " queried by user ", userID)

		// Retrieve the corresponding track from the TrackMap and respond to interaction
		query := b.TrackMap[userID]
		if query == nil {
			Logger.Warn("User ", userID, " is not registered in the track map.")
			response = SingleInteractionResponse("Could not find queries for the user. Please try a different query.",
				discordgo.InteractionResponseChannelMessageWithSource)
		} else {
			track := query[trackID]
			if track == nil {
				Logger.Warn("Track with ID ", trackID, " not found for the user ", userID)
				response = SingleInteractionResponse("Could not find tracks for the user. Please try a different query",
					discordgo.InteractionResponseChannelMessageWithSource)
			} else {
				Logger.Info("Track with ID ", trackID, " found. Chosen title: ", track.Info().Title)
				if err := b.Play(s, i, track); err != nil {
					Logger.Warn("Something went wrong when trying to play chosen single-track: ", err)
					response = SingleInteractionResponse("Could not query track. Please try a different query and make sure you are connected to a voice channel.",
						discordgo.InteractionResponseChannelMessageWithSource)
				} else {
					response = SingleButtonInteractionResponse(fmt.Sprintf("Querying the track: %v", track.Info().Title), "Click here for the link",
						*track.Info().URI, "🙈", discordgo.InteractionResponseUpdateMessage)
				}
			}
		}

		if err := s.InteractionRespond(i.Interaction, response); err != nil {
			Logger.Warn("Failed to create single-track interaction response: ", err)
		}
	},
}
