package gobot

import "github.com/bwmarrin/discordgo"

// For readability to shorten the code where the same responses are created

func SingleInteractionResponse(content string, interactionResponseType discordgo.InteractionResponseType) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: interactionResponseType,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   1 << 6,
		},
	}
}

func SingleFollowUpResponse(content string) *discordgo.WebhookParams {
	return &discordgo.WebhookParams{
		Content: content,
		Flags:   1 << 6,
	}
}

func SingleSelectMenuFollowUpResponse(content string, customID string, placeHolder string, options []discordgo.SelectMenuOption) *discordgo.WebhookParams {
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

func SingleEmbedFollowUpResponse(content string, title string, messageEmbedField []*discordgo.MessageEmbedField) *discordgo.WebhookParams {
	return &discordgo.WebhookParams{
		Content: content,
		Flags:   1 << 6,
		Embeds: []*discordgo.MessageEmbed{
			{
				Title:  title,
				Fields: messageEmbedField,
			},
		},
	}
}

func SingleButtonInteractionResponse(content string, buttonLabel string, url string, emojiName string, interactionResponseType discordgo.InteractionResponseType) *discordgo.InteractionResponse {
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

func SingleButtonFollowUpResponse(content string, buttonLabel string, url string, emojiName string) *discordgo.WebhookParams {
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
