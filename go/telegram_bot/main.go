package main

import (
	"encoding/json"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
)

var telegramBotApi *tgbotapi.BotAPI

type pair struct {
	GithubID int64
	Roles    []string
}

var authSessions = make(map[int64]pair)

func getDefaultKeyboard(roles []string) tgbotapi.ReplyKeyboardMarkup {
	var keyboardRows [][]tgbotapi.KeyboardButton
	for _, role := range roles {
		switch role {
		case "student":
			keyboardRows = append(keyboardRows, studentKeyboardRow1, studentKeyboardRow2)
		case "lecturer":
			keyboardRows = append(keyboardRows, lecturerKeyboardRow1, lecturerKeyboardRow2)
		case "admin":
			keyboardRows = append(keyboardRows, adminKeyboardRow)
		}
	}
	keyboardRows = append(keyboardRows, allKeyboardRow)
	return tgbotapi.NewReplyKeyboard(keyboardRows...)
}

const (
	AdminSessionStart      = "Начать сеанс администрирования"
	AdminSessionStartToken = "Начать сеанс администрирования (token)"

	StudentWhereNextClass   = "(студент) Где следующая пара"
	StudentScheduleWeekdays = "(студент) Расписание на дни недели"
	StudentScheduleToday    = "(студент) Расписание на сегодня"
	StudentScheduleTomorrow = "(студент) Расписание на завтра"
	StudentWhereTeacher     = "(студент) Где преподаватель"
	StudentExamsSchedule    = "(студент) Когда экзамены"

	LecturerWhereNextClass       = "(преподаватель) Где следующая пара"
	LecturerScheduleWeekdays     = "(преподаватель) Расписание на дни недели"
	LecturerScheduleToday        = "(преподаватель) Расписание на сегодня"
	LecturerScheduleTomorrow     = "(преподаватель) Расписание на завтра"
	LecturerLeaveComment         = "(преподаватель) Оставить комментарий к [номер] паре [для группы]"
	LecturerLeaveCommentWithDate = "(преподаватель) Оставить комментарий к [номер] паре [для группы] [дата]"
	LecturerWhereGroup           = "(преподаватель) Где группа / подгруппа"

	AllChangeName  = "Изменить ФИО"
	AllChangeGroup = "Изменить группу"
	AllLogout      = "Выйти"

	Monday    = "Понедельник"
	Tuesday   = "Вторник"
	Wednesday = "Среда"
	Thursday  = "Четверг"
	Friday    = "Пятница"
)

var adminKeyboardRow = tgbotapi.NewKeyboardButtonRow(
	tgbotapi.NewKeyboardButton(AdminSessionStart),
	tgbotapi.NewKeyboardButton(AdminSessionStartToken),
)

var studentKeyboardRow1 = tgbotapi.NewKeyboardButtonRow(
	tgbotapi.NewKeyboardButton(StudentWhereNextClass),
	tgbotapi.NewKeyboardButton(StudentScheduleWeekdays),
	tgbotapi.NewKeyboardButton(StudentScheduleToday),
)
var studentKeyboardRow2 = tgbotapi.NewKeyboardButtonRow(
	tgbotapi.NewKeyboardButton(StudentScheduleTomorrow),
	tgbotapi.NewKeyboardButton(StudentWhereTeacher),
	tgbotapi.NewKeyboardButton(StudentExamsSchedule),
)

var lecturerKeyboardRow1 = tgbotapi.NewKeyboardButtonRow(
	tgbotapi.NewKeyboardButton(LecturerWhereNextClass),
	tgbotapi.NewKeyboardButton(LecturerScheduleWeekdays),
	tgbotapi.NewKeyboardButton(LecturerScheduleToday),
	tgbotapi.NewKeyboardButton(LecturerScheduleTomorrow),
)

var lecturerKeyboardRow2 = tgbotapi.NewKeyboardButtonRow(
	tgbotapi.NewKeyboardButton(LecturerLeaveComment),
	tgbotapi.NewKeyboardButton(LecturerLeaveCommentWithDate),
	tgbotapi.NewKeyboardButton(LecturerWhereGroup),
)

var allKeyboardRow = tgbotapi.NewKeyboardButtonRow(
	tgbotapi.NewKeyboardButton(AllChangeName),
	tgbotapi.NewKeyboardButton(AllChangeGroup),
	tgbotapi.NewKeyboardButton(AllLogout),
)

var weekdaysKeyboard = tgbotapi.NewReplyKeyboard(tgbotapi.NewKeyboardButtonRow(
	tgbotapi.NewKeyboardButton(Monday),
	tgbotapi.NewKeyboardButton(Tuesday),
	tgbotapi.NewKeyboardButton(Wednesday),
	tgbotapi.NewKeyboardButton(Thursday),
	tgbotapi.NewKeyboardButton(Friday),
))

var noneKeyboard = tgbotapi.NewRemoveKeyboard(true)

var chatSessions = make(map[int64][]string)

func sendDefaultMessage(chatId int64) {
	roles := authSessions[chatId].Roles
	msg := tgbotapi.NewMessage(chatId, "Выберите действие!")
	msg.ReplyMarkup = getDefaultKeyboard(roles)
	if _, err := telegramBotApi.Send(msg); err != nil {
		log.Printf("Error sending message to chat %d: %v", chatId, err)
	}
	delete(chatSessions, chatId)
}

func sendMessage(chatId int64, message string) {
	msg := tgbotapi.NewMessage(chatId, message)
	msg.ReplyMarkup = noneKeyboard
	if _, err := telegramBotApi.Send(msg); err != nil {
		log.Printf("Error sending message to chat %d: %v", chatId, err)
	}
}

func sendErrorMessage(chatId int64, message string) {
	msg := tgbotapi.NewMessage(chatId, message)
	msg.ReplyMarkup = noneKeyboard
	if _, err := telegramBotApi.Send(msg); err != nil {
		log.Printf("Error sending message to chat %d: %v", chatId, err)
	}
}

//func process

func processAction(chatId int64, action string, callMethod func(string) (string, error)) {
	jwt, err := handleGrantPermission(chatId, action)
	if err == nil {
		message, err := callMethod(jwt)
		if err != nil {
			sendErrorMessage(chatId, "Ошибка выполнения запроса")
		} else {
			sendMessage(chatId, message)
		}
	}
	sendDefaultMessage(chatId)
}

func handleGrantPermission(chatId int64, action string) (string, error) {
	jwt, isPermitted, err := processGrantPermission(chatId, action)
	if err != nil {
		sendErrorMessage(chatId, "Error processing action: "+err.Error())
		return "", err
	}

	if !isPermitted {
		sendErrorMessage(chatId, "You don't have permission to perform this action.")
		return "", fmt.Errorf("You don't have permission to perform this action.")
	}
	return jwt, nil
}

func processGrantPermission(chatId int64, action string) (string, bool, error) {
	var githubId = authSessions[chatId].GithubID

	url := fmt.Sprintf("http://localhost:8090/grantJWT?github_id=%d&action=%s", githubId, action)
	client := http.Client{}
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", false, err
	}
	response, err := client.Do(request)
	if err != nil {
		return "", false, err
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusOK:
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return "", false, err
		}
		return string(body), true, nil
	case http.StatusForbidden:
		return "", false, nil
	default:
		return "", false, fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}
}

func main() {
	var err error
	telegramBotApi, err = tgbotapi.NewBotAPI("6684308640:AAGMlLQkfy3d-WTTdNo-UYCUKLQEjJeC2Cs")
	if err != nil {
		log.Panic(err)
		return
	}
	log.Printf("Authorized on account %s", telegramBotApi.Self.UserName)
	setupServer()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 10
	updates, _ := telegramBotApi.GetUpdatesChan(u)
	for update := range updates {
		if update.Message == nil {
			continue
		}
		var chatId = update.Message.Chat.ID

		msg := tgbotapi.NewMessage(chatId, "")
		log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

		if hasSessionForChat(chatId) {
			log.Printf("Сессия для чата %d есть", chatId)

			chatSessions[chatId] = append(chatSessions[chatId], update.Message.Text)

			switch chatSessions[chatId][0] {
			case AdminSessionStart:
				processAction(chatId, "AdminSessionStart", callAdminSessionStart)

			case AdminSessionStartToken:
				if len(chatSessions[chatId]) > 1 {
					log.Printf("Выполнить запрос с токеном %s", chatSessions[chatId][1])
					processAction(chatId, "AdminSessionStartToken", func(jwt string) (string, error) {
						return callAdminSessionStartToken(jwt, chatSessions[chatId][1])
					})
				} else {
					msg.Text = string("Введи токен:")
					msg.ReplyMarkup = noneKeyboard
					if _, err := telegramBotApi.Send(msg); err != nil {
						log.Printf("Error sending message to chat %d: %v", update.Message.Chat.ID, err)
					}
				}

			case StudentWhereNextClass:
				processAction(chatId, "StudentWhereNextClass", callStudentWhereNextClass)

			case StudentScheduleWeekdays:
				if len(chatSessions[chatId]) > 1 {
					day := -1
					switch chatSessions[chatId][1] {
					case Monday:
						day = 1
					case Tuesday:
						day = 2
					case Wednesday:
						day = 3
					case Thursday:
						day = 4
					case Friday:
						day = 5
					}
					if day == -1 {
						chatSessions[chatId] = chatSessions[chatId][:len(chatSessions[chatId])-1]
						msg.Text = string("Введен неверный день недели. Введи день недели еще раз:")
						msg.ReplyMarkup = weekdaysKeyboard
						if _, err := telegramBotApi.Send(msg); err != nil {
							log.Printf("Error sending message to chat %d: %v", update.Message.Chat.ID, err)
						}
					} else {
						log.Printf("Выполнить запрос с днем недели %s %d", chatSessions[chatId][1], day)
						processAction(chatId, "StudentScheduleWeekdays", func(jwt string) (string, error) {
							return callStudentScheduleWeekdays(jwt, day)
						})
					}
				} else {
					msg.Text = string("Введи день недели:")
					msg.ReplyMarkup = weekdaysKeyboard
					if _, err := telegramBotApi.Send(msg); err != nil {
						log.Printf("Error sending message to chat %d: %v", update.Message.Chat.ID, err)
					}
				}

			case StudentScheduleToday:
				processAction(chatId, "StudentScheduleToday", callStudentScheduleToday)

			case StudentScheduleTomorrow:
				processAction(chatId, "StudentScheduleTomorrow", callStudentScheduleTomorrow)

			case StudentWhereTeacher:
				if len(chatSessions[chatId]) > 1 {
					log.Printf("Выполнить запрос с ФИО преподавателя %s", chatSessions[chatId][1])
					processAction(chatId, "StudentWhereTeacher", func(jwt string) (string, error) {
						return callStudentWhereTeacher(jwt, chatSessions[chatId][1])
					})
				} else {
					msg.Text = string("Введи ФИО преподавателя:")
					msg.ReplyMarkup = noneKeyboard
					if _, err := telegramBotApi.Send(msg); err != nil {
						log.Printf("Error sending message to chat %d: %v", update.Message.Chat.ID, err)
					}
				}

			case StudentExamsSchedule:
				processAction(chatId, "StudentExamsSchedule", callStudentExamsSchedule)

			case LecturerWhereNextClass:
				processAction(chatId, "LecturerWhereNextClass", callLecturerWhereNextClass)

			case LecturerScheduleWeekdays:
				if len(chatSessions[chatId]) > 1 {
					day := -1
					switch chatSessions[chatId][1] {
					case Monday:
						day = 1
					case Tuesday:
						day = 2
					case Wednesday:
						day = 3
					case Thursday:
						day = 4
					case Friday:
						day = 5
					}
					if day == -1 {
						chatSessions[chatId] = chatSessions[chatId][:len(chatSessions[chatId])-1]
						msg.Text = string("Введен неверный день недели. Введи день недели еще раз:")
						msg.ReplyMarkup = weekdaysKeyboard
						if _, err := telegramBotApi.Send(msg); err != nil {
							log.Printf("Error sending message to chat %d: %v", update.Message.Chat.ID, err)
						}
					} else {
						log.Printf("Выполнить запрос с днем недели %s %d", chatSessions[chatId][1], day)
						processAction(chatId, "LecturerScheduleWeekdays", func(jwt string) (string, error) {
							return callLecturerScheduleWeekdays(jwt, day)
						})
					}
				} else {
					msg.Text = string("Введи день недели:")
					msg.ReplyMarkup = weekdaysKeyboard
					if _, err := telegramBotApi.Send(msg); err != nil {
						log.Printf("Error sending message to chat %d: %v", update.Message.Chat.ID, err)
					}
				}

			case LecturerScheduleToday:
				processAction(chatId, "LecturerScheduleToday", callLecturerScheduleToday)

			case LecturerScheduleTomorrow:
				processAction(chatId, "LecturerScheduleTomorrow", callLecturerScheduleTomorrow)

			case LecturerLeaveComment:
				if len(chatSessions[chatId]) > 1 {
					if len(chatSessions[chatId]) > 2 {
						if len(chatSessions[chatId]) > 3 {
							log.Printf("Выполнить запрос с группой %s и номером пары %s коммент %s", chatSessions[chatId][1], chatSessions[chatId][2], chatSessions[chatId][3])
							processAction(chatId, "LecturerLeaveComment", func(jwt string) (string, error) {
								return callLecturerLeaveComment(jwt, chatSessions[chatId][1], chatSessions[chatId][2], chatSessions[chatId][3])
							})
						} else {
							msg.Text = string("Введи комментарий:")
							msg.ReplyMarkup = noneKeyboard
							if _, err := telegramBotApi.Send(msg); err != nil {
								log.Printf("Error sending message to chat %d: %v", update.Message.Chat.ID, err)
							}
						}
					} else {
						msg.Text = string("Введи номер пары:")
						msg.ReplyMarkup = noneKeyboard
						if _, err := telegramBotApi.Send(msg); err != nil {
							log.Printf("Error sending message to chat %d: %v", update.Message.Chat.ID, err)
						}
					}
				} else {
					msg.Text = string("Введи группу:")
					msg.ReplyMarkup = noneKeyboard
					if _, err := telegramBotApi.Send(msg); err != nil {
						log.Printf("Error sending message to chat %d: %v", update.Message.Chat.ID, err)
					}
				}

			case LecturerLeaveCommentWithDate:
				if len(chatSessions[chatId]) > 1 {
					if len(chatSessions[chatId]) > 2 {
						if len(chatSessions[chatId]) > 3 {
							if len(chatSessions[chatId]) > 4 {
								log.Printf("Выполнить запрос с группой %s и номером пары %s и датой %s коммент %s", chatSessions[chatId][1], chatSessions[chatId][2], chatSessions[chatId][3], chatSessions[chatId][4])
								processAction(chatId, "LecturerLeaveCommentWithDate", func(jwt string) (string, error) {
									return callLecturerLeaveCommentWithDate(jwt, chatSessions[chatId][1], chatSessions[chatId][2], chatSessions[chatId][3], chatSessions[chatId][4])
								})
							} else {
								msg.Text = string("Введи комментарий:")
								msg.ReplyMarkup = noneKeyboard
								if _, err := telegramBotApi.Send(msg); err != nil {
									log.Printf("Error sending message to chat %d: %v", update.Message.Chat.ID, err)
								}
							}
						} else {
							msg.Text = string("Введи дату:")
							msg.ReplyMarkup = noneKeyboard
							if _, err := telegramBotApi.Send(msg); err != nil {
								log.Printf("Error sending message to chat %d: %v", update.Message.Chat.ID, err)
							}
						}
					} else {
						msg.Text = string("Введи номер пары:")
						msg.ReplyMarkup = noneKeyboard
						if _, err := telegramBotApi.Send(msg); err != nil {
							log.Printf("Error sending message to chat %d: %v", update.Message.Chat.ID, err)
						}
					}
				} else {
					msg.Text = string("Введи группу:")
					msg.ReplyMarkup = noneKeyboard
					if _, err := telegramBotApi.Send(msg); err != nil {
						log.Printf("Error sending message to chat %d: %v", update.Message.Chat.ID, err)
					}
				}

			case LecturerWhereGroup:
				if len(chatSessions[chatId]) > 1 {
					log.Printf("Выполнить запрос с группой %s", chatSessions[chatId][1])
					processAction(chatId, "LecturerWhereGroup", func(jwt string) (string, error) {
						return callLecturerWhereGroup(jwt, chatSessions[chatId][1])
					})
				} else {
					msg.Text = string("Введи группу:")
					msg.ReplyMarkup = noneKeyboard
					if _, err := telegramBotApi.Send(msg); err != nil {
						log.Printf("Error sending message to chat %d: %v", update.Message.Chat.ID, err)
					}
				}

			case AllChangeName:
				if len(chatSessions[chatId]) > 1 {
					log.Printf("Выполнить запрос с ФИО %s", chatSessions[chatId][1])
					message, err := callAllChangeName(chatId, chatSessions[chatId][1])
					if err != nil {
						sendErrorMessage(chatId, "Ошибка выполнения запроса")
					} else {
						sendMessage(chatId, message)
					}
					sendDefaultMessage(chatId)
				} else {
					msg.Text = string("Введи ФИО:")
					msg.ReplyMarkup = noneKeyboard
					if _, err := telegramBotApi.Send(msg); err != nil {
						log.Printf("Error sending message to chat %d: %v", update.Message.Chat.ID, err)
					}
				}

			case AllChangeGroup:
				if len(chatSessions[chatId]) > 1 {
					log.Printf("Выполнить запрос с группой %s", chatSessions[chatId][1])
					message, err := callAllChangeGroup(chatId, chatSessions[chatId][1])
					if err != nil {
						sendErrorMessage(chatId, "Ошибка выполнения запроса")
					} else {
						sendMessage(chatId, message)
					}
					sendDefaultMessage(chatId)
				} else {
					msg.Text = string("Введи группу:")
					msg.ReplyMarkup = noneKeyboard
					if _, err := telegramBotApi.Send(msg); err != nil {
						log.Printf("Error sending message to chat %d: %v", update.Message.Chat.ID, err)
					}
				}

			case AllLogout:
				log.Printf("Выход...")
				delete(authSessions, update.Message.Chat.ID)
				delete(chatSessions, update.Message.Chat.ID)
				msg.Text = string("Выход выполнен")
				msg.ReplyMarkup = noneKeyboard
				if _, err := telegramBotApi.Send(msg); err != nil {
					log.Printf("Error sending message to chat %d: %v", update.Message.Chat.ID, err)
				}
			default:
				sendDefaultMessage(chatId)

			}
		} else {
			log.Printf("Сессии для чата %d нет", chatId)

			client := http.Client{}
			requestURL := fmt.Sprintf("http://localhost:8090/auth?chat_id=%d", update.Message.Chat.ID)
			request, _ := http.NewRequest("GET", requestURL, nil)
			response, _ := client.Do(request)
			resBody, _ := io.ReadAll(response.Body)
			if len(resBody) != 0 {
				msg.Text = string(resBody)
				telegramBotApi.Send(msg)
			}
			defer response.Body.Close()
		}
	}
}

func setupServer() {
	http.HandleFunc("/auth", authHandler)
	go func() { http.ListenAndServe(":8091", nil) }() // Запуск сервера на порту 8091
}

// ROUTES ///////////////////////////////////////////////////

type About struct {
	FullName string `bson:"full_name" json:"full_name"`
	Group    string `bson:"group" json:"group"`
}

type User struct {
	GithubId int64    `bson:"github_id" json:"github_id"`
	ChatId   int64    `bson:"chat_id" json:"chat_id"`
	Roles    []string `bson:"roles" json:"roles"` // student lecturer admin
	About    About    `bson:"about" json:"about"`
}

func authHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Handling /auth request")
	var user User
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("github_id: %s, chat_id: %d", user.GithubId, user.ChatId)
	authSessions[user.ChatId] = pair{
		GithubID: user.GithubId,
		Roles:    user.Roles,
	}

	botMessage := tgbotapi.NewMessage(user.ChatId, "Вы успешно авторизировались!")
	botMessage.ReplyMarkup = noneKeyboard
	if _, err := telegramBotApi.Send(botMessage); err != nil {
		log.Printf("Error sending message to chat %d: %v", user.ChatId, err)
	}
	sendDefaultMessage(user.ChatId)
}

// UTILS ///////////////////////////////////////////////////

func hasSessionForChat(chatId int64) bool {
	log.Printf("[%d] %s", chatId, authSessions)

	for session, _ := range authSessions {
		if chatId == session {
			return true
		}
	}
	return false
}

func encodeQuery(input string) string {
	return url.QueryEscape(input)
}
