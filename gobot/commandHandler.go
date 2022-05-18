package gobot

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/disgoorg/disgolink/lavalink"
)

var CommandsHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate, b *Bot){
	"play": func(s *discordgo.Session, i *discordgo.InteractionCreate, b *Bot) {

		// get input string from play command
		query := fmt.Sprintf("%v", i.ApplicationCommandData().Options[0].Value)
		Logger.Debug("Song queried: ", query)

		// if query is url, add ytsearch for lavalink
		if !urlPattern.MatchString(query) {
			query = "ytsearch:" + query
		}

		//TODO filter for length > 100

		// handle different return values from lavalink and play track(s) ...
		_ = b.Link.BestRestClient().LoadItemHandler(context.TODO(), query, lavalink.NewResultHandler(
			func(track lavalink.AudioTrack) {
				// directly queue track if it is a single track
				if err := b.Play(s, i, track); err != nil {
					Logger.Warn("Error occured while trying to play single track: ", err)
				}

				// initial response
				err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Adding the song to queue: " + query,
						Flags:   1 << 6,
						Components: []discordgo.MessageComponent{
							discordgo.ActionsRow{
								Components: []discordgo.MessageComponent{
									discordgo.Button{
										Label:    "Link to your song :)",
										Style:    discordgo.LinkButton,
										Disabled: false,
										URL:      *track.Info().URI,
										Emoji: discordgo.ComponentEmoji{
											Name: "🤷",
										},
									},
								},
							},
						},
					},
				})
				if err != nil {
					Logger.Warn("Something went wrong when interacting with play command: ", err)
				}

			},
			func(playlist lavalink.AudioPlaylist) {
				// directly queue playlist
				if err := b.Play(s, i, playlist.Tracks()...); err != nil {
					Logger.Warn("Error occured while trying to play tracks of playlist: ", err)
				}
				// initial response
				err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Adding the song to queue: " + query,
						Flags:   1 << 6,
						Components: []discordgo.MessageComponent{
							discordgo.ActionsRow{
								Components: []discordgo.MessageComponent{
									discordgo.Button{
										Label:    "Link to your playlist :)",
										Style:    discordgo.LinkButton,
										Disabled: false,
										URL:      "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
										Emoji: discordgo.ComponentEmoji{
											Name: "🤷",
										},
									},
								},
							},
						},
					},
				})
				if err != nil {
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
						//Description: fmt.Sprintf("yt search result number %d", i),
					})
				}

				response := &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Please choose a song from the menu.",
						Flags:   1 << 6,
						Components: []discordgo.MessageComponent{
							discordgo.ActionsRow{
								Components: []discordgo.MessageComponent{
									discordgo.SelectMenu{
										CustomID:    "selectTrack",
										Placeholder: "Choose your desired youtube video 👇",
										Options:     options,
									},
								},
							},
						},
					},
				}

				if err := s.InteractionRespond(i.Interaction, response); err != nil {
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
				_, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
					Content: "No matches found for your query.",
					Flags:   1 << 6,
					Components: []discordgo.MessageComponent{
						discordgo.ActionsRow{
							Components: []discordgo.MessageComponent{
								discordgo.Button{
									Label:    "Try again or something",
									Style:    discordgo.LinkButton,
									Disabled: false,
									URL:      "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
									Emoji: discordgo.ComponentEmoji{
										Name: "🤷",
									},
								},
							},
						},
					},
				})

				if err != nil {
					Logger.Warn("Failed to create follow up message for empty query matches.")
				}
			},
			func(ex lavalink.FriendlyException) {
				_, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
					Content: "Error while loading your queried track.",
					Flags:   1 << 6,
					Components: []discordgo.MessageComponent{
						discordgo.ActionsRow{
							Components: []discordgo.MessageComponent{
								discordgo.Button{
									Label:    "Try again or something",
									Style:    discordgo.LinkButton,
									Disabled: false,
									URL:      "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
									Emoji: discordgo.ComponentEmoji{
										Name: "🤷",
									},
								},
							},
						},
					},
				})

				if err != nil {
					Logger.Warn("Failed to create follow up message for track loading error.")
				}
			},
		))
	},
	"leave": func(s *discordgo.Session, i *discordgo.InteractionCreate, b *Bot) {
		Logger.Debug("Trying to leave voice channel.")

		if err := b.leave(s, i.GuildID); err != nil {
			Logger.Warn("Bot was unable to leave voice channel: ", err)
		}

		response := &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "行ってきます、ご主人様",
				Flags:   1 << 6,
			},
		}

		//TODO fix play -> leave -> play bug?
		// destroy, reconfig playermanager + queue
		if err := b.leave(s, i.GuildID); err != nil {
			Logger.Warn("Bot was unable to leave voice channel: ", err)
		}

		if err := b.PlayerManagers[i.GuildID].Player.Destroy(); err != nil {
			Logger.Warn("Bot was unable to destroy player upon leaving: ", err)
		}

		if err := s.InteractionRespond(i.Interaction, response); err != nil {
			Logger.Warn("Failed to create interaction response to leave command: ", err)
		}
	},
	"skip": func(s *discordgo.Session, i *discordgo.InteractionCreate, b *Bot) {
		Logger.Debug("Trying to skip song.")

		if err := b.skip(s, i.GuildID); err != nil {
			Logger.Warn("Bot was unable to skip the song: ", err)
		}

		response := &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Skipping song.",
				Flags:   1 << 6,
			},
		}

		if err := s.InteractionRespond(i.Interaction, response); err != nil {
			Logger.Warn("Failed to create interaction response to skip command: ", err)
		}
	},
	// "playlist": func(s *discordgo.Session, i *discordgo.InteractionCreate, b *Bot) {
	// 	Logger.Debug("Trying to display playlist.")

	// 	tracks := b.PlayerManagers[i.GuildID].getAllTracks()
	// 	if tracks != nil {
	// 		var options []discordgo.SelectMenuOption
	// 			for i := 0; i < 5; i++ {
	// 				options = append(options, discordgo.SelectMenuOption{
	// 					Label: tracks[i].Info().Title,
	// 					Value: tracks[i].Info().Identifier,
	// 					Emoji: discordgo.ComponentEmoji{
	// 						Name: NumberEmojiMap[i+1],
	// 					},
	// 					Default: false,
	// 					//Description: fmt.Sprintf("yt search result number %d", i),
	// 				})
	// 			}

	// 			response := &discordgo.InteractionResponse{
	// 				Type: discordgo.InteractionResponseChannelMessageWithSource,
	// 				Data: &discordgo.InteractionResponseData{
	// 					Content: "Please choose a song from the menu.",
	// 					Flags:   1 << 6,
	// 					Components: []discordgo.MessageComponent{
	// 						discordgo.ActionsRow{
	// 							Components: []discordgo.MessageComponent{
	// 								discordgo.SelectMenu{
	// 									CustomID:    "selectTrack",
	// 									Placeholder: "Choose your desired youtube video 👇",
	// 									Options:     options,
	// 								},
	// 							},
	// 						},
	// 					},
	// 				},
	// 			}
	// 	}

	// 	// TODO select menu for bot queue
	// },
	// TODO show queue
	// TODO help command
	// TODO pause?
	// TODO seek?
	// TODO
	// TODO set play mode
	// TODO avatar update
}
