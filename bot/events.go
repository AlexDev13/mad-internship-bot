package bot

import (
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/maddevsio/mad-internship-bot/model"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	log "github.com/sirupsen/logrus"
)

func (b *Bot) handleUpdate(update tgbotapi.Update) error {
	localizer := i18n.NewLocalizer(b.bundle, "en_US")

	message := update.Message

	if message == nil {
		message = update.EditedMessage
	}

	if message.Chat.Type == "private" {
		ok, errors := isStandup(message.Text)
		if !ok {
			text, err := localizer.Localize(&i18n.LocalizeConfig{
				DefaultMessage: &i18n.Message{
					ID:    "not standup",
					Other: "Seems like this is not a standup, double check keywords for errors",
				},
			})
			if err != nil {
				log.Error(err)
			}
			text += strings.Join(errors, "\n")
			msg := tgbotapi.NewMessage(message.Chat.ID, text)
			msg.ReplyToMessageID = message.MessageID
			_, err = b.tgAPI.Send(msg)
			return err
		}

		advises, _ := analyzeStandup(message.Text)

		text := "Это хороший стендап который не стыдно постить в группу!"

		if len(advises) != 0 {
			text = "Чтобы стендап был более полезен вот несколько советов: \n" + strings.Join(advises, "\n")
		}

		msg := tgbotapi.NewMessage(message.Chat.ID, text)
		msg.ReplyToMessageID = message.MessageID
		_, err := b.tgAPI.Send(msg)
		return err
	}

	if message.From.IsBot {
		return nil
	}

	containsPR, prs := containsPullRequests(message.Text)
	if containsPR {
		for _, pr := range prs {
			warnings := analyzePullRequest(pr)
			if len(warnings) == 0 {
				msg := tgbotapi.NewMessage(message.Chat.ID, *pr.HTMLURL+" - хороший PR, можно смотреть!")
				msg.ReplyToMessageID = message.MessageID
				msg.DisableWebPagePreview = true
				b.tgAPI.Send(msg)
			} else {
				text := *pr.HTMLURL + " - PR надо поправить. Найдены недочёты: \n"
				text += strings.Join(warnings, "\n")
				msg := tgbotapi.NewMessage(message.Chat.ID, text)
				msg.ReplyToMessageID = message.MessageID
				msg.DisableWebPagePreview = true
				b.tgAPI.Send(msg)
			}
		}
	}

	if message.IsCommand() {
		return b.HandleCommand(update)
	}

	if message.Text != "" {
		return b.HandleMessageEvent(message)
	}

	if message.LeftChatMember != nil {
		return b.HandleChannelLeftEvent(update)
	}

	if message.NewChatMembers != nil {
		return b.HandleChannelJoinEvent(update)
	}

	return nil
}

//HandleMessageEvent function to analyze and save standups
func (b *Bot) HandleMessageEvent(message *tgbotapi.Message) error {

	if !strings.Contains(message.Text, b.tgAPI.Self.UserName) {
		return nil
	}

	ok, _ := isStandup(message.Text)

	if !ok {
		return fmt.Errorf("Message is not a standup")
	}

	st, err := b.db.SelectStandupByMessageID(message.MessageID, message.Chat.ID)
	if err != nil {
		log.Info("standup does not yet exist, create new standup")
		_, err := b.db.CreateStandup(&model.Standup{
			MessageID: message.MessageID,
			Created:   time.Now().UTC(),
			Modified:  time.Now().UTC(),
			Username:  message.From.UserName,
			Text:      message.Text,
			ChatID:    message.Chat.ID,
		})

		if err != nil {
			return err
		}

		advises, _ := analyzeStandup(message.Text)

		text := "Спасибо, стендап принят, и, кажется, он классный!"

		if len(advises) != 0 {
			text = "Стендап принимается, но позволь дать пару советов: \n" + strings.Join(advises, "\n")
		}

		msg := tgbotapi.NewMessage(message.Chat.ID, text)
		msg.ReplyToMessageID = message.MessageID
		_, err = b.tgAPI.Send(msg)
		return err
	}

	_, err = b.db.UpdateStandup(st)
	if err != nil {
		log.Error("Could not update standup: ", err)
		return err
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, "Cтендап обновлён!")
	msg.ReplyToMessageID = message.MessageID
	_, err = b.tgAPI.Send(msg)
	return err
}

//HandleChannelLeftEvent function to remove bot and standupers from channels
func (b *Bot) HandleChannelLeftEvent(event tgbotapi.Update) error {
	member := event.Message.LeftChatMember
	// if user is a bot
	if member.UserName == b.tgAPI.Self.UserName {
		team := b.findTeam(event.Message.Chat.ID)
		if team == nil {
			return fmt.Errorf("Could not find sutable team")
		}
		team.Stop()

		err := b.db.DeleteGroupStandupers(event.Message.Chat.ID)
		if err != nil {
			return err
		}
		err = b.db.DeleteGroup(team.Group.ID)
		if err != nil {
			return err
		}
		return nil
	}

	standuper, err := b.db.FindStanduper(member.UserName, event.Message.Chat.ID)
	if err != nil {
		return nil
	}
	err = b.db.DeleteStanduper(standuper.ID)
	if err != nil {
		return err
	}
	return nil
}

//HandleChannelJoinEvent function to add bot and standupers t0 channels
func (b *Bot) HandleChannelJoinEvent(event tgbotapi.Update) error {
	for _, member := range *event.Message.NewChatMembers {
		// if user is a bot
		if member.UserName == b.tgAPI.Self.UserName {

			_, err := b.db.FindGroup(event.Message.Chat.ID)
			if err != nil {
				log.Info("Could not find the group, creating...")
				group, err := b.db.CreateGroup(&model.Group{
					ChatID:          event.Message.Chat.ID,
					Title:           event.Message.Chat.Title,
					Username:        event.Message.Chat.UserName,
					Description:     event.Message.Chat.Description,
					StandupDeadline: "10:00",
					TZ:              "Asia/Bishkek", // default value...
					Language:        "ru_RU",        // default value...
				})
				if err != nil {
					return err
				}

				b.watchersChan <- group
			}
			// Send greeting message after success group save
			text := "Всем привет! Я буду помогать вам не забывать о сдаче стендапов вовремя. За все мои ошибки отвечает @anatoliyfedorenko :)"
			_, err = b.tgAPI.Send(tgbotapi.NewMessage(event.Message.Chat.ID, text))
			return err
		}

		if member.IsBot {
			//Skip adding bot to standupers
			return nil
		}
		//if it is a regular user, greet with welcoming message and add to standupers
		_, err := b.db.FindStanduper(member.UserName, event.Message.Chat.ID) // user[1:] to remove leading @
		if err == nil {
			return nil
		}

		_, err = b.db.CreateStanduper(&model.Standuper{
			UserID:       member.ID,
			Username:     member.UserName,
			ChatID:       event.Message.Chat.ID,
			LanguageCode: member.LanguageCode,
			TZ:           "Asia/Bishkek", // default value...
		})
		if err != nil {
			log.Error("CreateStanduper failed: ", err)
			return nil
		}

		group, err := b.db.FindGroup(event.Message.Chat.ID)
		if err != nil {
			group, err = b.db.CreateGroup(&model.Group{
				ChatID:          event.Message.Chat.ID,
				Title:           event.Message.Chat.Title,
				Description:     event.Message.Chat.Description,
				StandupDeadline: "10:00",
				TZ:              "Asia/Bishkek", // default value...
				Language:        "ru_RU",        // default value...
			})
			if err != nil {
				return err
			}
		}

		var welcome, onbording, deadline, closing string

		welcome = fmt.Sprintf("Привет, @%v! Добро пожаловать в %v!\n", member.UserName, event.Message.Chat.Title)

		onbording = strings.Replace(b.c.OnbordingMessage, `"`, "", -1) + "\n\n"

		if group.StandupDeadline != "" {
			deadline = fmt.Sprintf("Срок сдачи стендапов ежедневно до %s. В выходные пишите стендапы по желанию.\n\n", group.StandupDeadline)
		}

		standups := "Если сомневаетесь в стендапе, напишите мне в личку, я проверю, всё ли в порядке. Не стесняйтесь \n"

		closing = "Я менеджер, который не принимает отговорок. Если вы пропустили стендап два раза, я удалю вас из группы на третий пропуск. Если по каким-либо серьезным причинам нужно перестать ждать стендапы от вас, сделайте /leave .\n\nЗа все мои ошибки отвечает @anatoliyfedorenko"

		text := welcome + onbording + deadline + standups + closing

		_, err = b.tgAPI.Send(tgbotapi.NewMessage(event.Message.Chat.ID, text))
		return err
	}
	return nil
}
