package gobot

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

var ComponentsHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate, b *Bot){
	"selectTrack": func(s *discordgo.Session, i *discordgo.InteractionCreate, b *Bot) {
		// Get user and track IDs
		var response *discordgo.InteractionResponse
		data := i.MessageComponentData().Values
		if data == nil {
			Logger.Warn("Expected user response but values are empty. Make sure the commands are set up properly.")
			response := SingleInteractionResponse("An error occurred. The bot interaction appears to be set up incorrectly. Please try again later.",
				discordgo.InteractionResponseChannelMessageWithSource)
			if err := s.InteractionRespond(i.Interaction, response); err != nil {
				Logger.Warn("Failed to create interaction response: ", err)
			}
			return
		}
		var trackID = data[0]
		var userID = i.Member.User.ID

		selectLogger := Logger.WithFields(logrus.Fields{
			"cmp":     "select",
			"userID":  userID,
			"guildID": i.GuildID,
			"trackID": trackID,
		})
		selectLogger.Info("Select component interaction triggered.")

		// Retrieve the corresponding track from the TrackMap and respond to interaction
		query := b.TrackMap[userID]
		if query == nil {
			selectLogger.Warn("User is not registered in the track map.")
			response = SingleInteractionResponse("Could not find queries for the user. Please try a different query.",
				discordgo.InteractionResponseChannelMessageWithSource)
		} else {
			track := query[trackID]
			if track == nil {
				selectLogger.Warn("Track not found.")
				response = SingleInteractionResponse("Could not find tracks for the user. Please try a different query",
					discordgo.InteractionResponseChannelMessageWithSource)
			} else {
				selectLogger.Debug("Track ID found. Chosen title: ", track.Info().Title)
				if err := b.Play(s, i, track); err != nil {
					selectLogger.Warn("Something went wrong when trying to play chosen single-track: ", err)
					response = SingleInteractionResponse("Could not query track. Please try a different query and make sure you are connected to a voice channel.",
						discordgo.InteractionResponseChannelMessageWithSource)
				} else {
					response = SingleButtonInteractionResponse(fmt.Sprintf("Querying the track: %v", track.Info().Title), "Click here for the link",
						*track.Info().URI, "🙈", discordgo.InteractionResponseUpdateMessage)
				}
			}
		}

		if err := s.InteractionRespond(i.Interaction, response); err != nil {
			selectLogger.Warn("Failed to create interaction response: ", err)
		}
	},
}
