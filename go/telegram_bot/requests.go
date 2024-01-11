package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Class struct {
	Title        string `bson:"title" json:"title"`
	Type         string `bson:"type" json:"type"`
	Teacher      string `bson:"teacher" json:"teacher"`
	LessonNumber string `bson:"lesson_number" json:"lesson_number"`
	Group        string `bson:"group" json:"group"`
	Date         string `bson:"date" json:"date"`
	Comment      string `bson:"comment" json:"comment"`
	Address      string `bson:"address" json:"address"`
}

func callAdminSessionStart(jwt string) (string, error) {
	client := http.Client{}
	requestURL := fmt.Sprintf("http://localhost:8060/startAdminSession?token=%s", jwt)
	response, err := client.Get(requestURL)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}
	adminSessionLink, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	expirationComment := fmt.Sprintf("\nЭта ссылка действительна 5 минут")
	return string(adminSessionLink) + expirationComment, nil
}

func callAdminSessionStartToken(jwt string, requestToken string) (string, error) {
	client := http.Client{}
	requestURL := fmt.Sprintf("http://localhost:8060/startAdminSession?token=%s&request_token=%s", jwt, requestToken)
	response, err := client.Get(requestURL)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}
	return "Сессия администрирования активирована", nil
}

func callStudentWhereNextClass(jwt string) (string, error) {
	client := http.Client{}
	requestURL := fmt.Sprintf("http://localhost:8050/nextLesson?token=%s", jwt)
	request, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return "", err
	}

	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		return "Занятий не найдено.", nil
	}

	if response.StatusCode == http.StatusBadRequest {
		return "Недостаточно данных. Заполни профиль.", nil
	}

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}

	var nextClass Class
	err = json.NewDecoder(response.Body).Decode(&nextClass)
	if err != nil {
		return "", err
	}

	message := fmt.Sprintf("Next class:\nTitle: %s\nType: %s\nTeacher: %s\nLesson Number: %s\nGroup: %s\nDate: %s\nComment: %s\nAddress: %s",
		nextClass.Title, nextClass.Type, nextClass.Teacher, nextClass.LessonNumber, nextClass.Group, nextClass.Date, nextClass.Comment, nextClass.Address)

	return message, nil
}

func callStudentScheduleWeekdays(jwt string, dayNumber int) (string, error) {
	currentTime := time.Now()
	weekday := int(currentTime.Weekday())
	daysUntilTargetDay := (dayNumber - weekday + 7) % 7
	targetDate := currentTime.AddDate(0, 0, daysUntilTargetDay).Format("02.01.06")

	client := http.Client{}
	requestURL := fmt.Sprintf("http://localhost:8050/timetableByDay?token=%s&date=%s", jwt, encodeQuery(targetDate))
	request, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return "", err
	}

	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		return "Занятий не найдено.", nil
	}

	if response.StatusCode == http.StatusBadRequest {
		return "Недостаточно данных. Заполни профиль.", nil
	}

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}

	var classes []Class
	err = json.NewDecoder(response.Body).Decode(&classes)
	if err != nil {
		return "", err
	}

	message := fmt.Sprintf("Schedule for %s:\n", targetDate)
	for _, class := range classes {
		message += fmt.Sprintf("\nTitle: %s\nType: %s\nTeacher: %s\nLesson Number: %s\nGroup: %s\nDate: %s\nComment: %s\nAddress: %s\n",
			class.Title, class.Type, class.Teacher, class.LessonNumber, class.Group, class.Date, class.Comment, class.Address)
	}
	return message, nil
}

func callStudentScheduleToday(jwt string) (string, error) {
	client := http.Client{}
	requestURL := fmt.Sprintf("http://localhost:8050/timetableToday?token=%s", jwt)
	request, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return "", err
	}

	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return "Занятий не найдено.", nil
	}

	if response.StatusCode == http.StatusBadRequest {
		return "Недостаточно данных. Заполни профиль.", nil
	}

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}
	var classes []Class
	err = json.NewDecoder(response.Body).Decode(&classes)
	if err != nil {
		return "", err
	}
	message := "Today's schedule:\n"
	for _, class := range classes {
		message += fmt.Sprintf("\nTitle: %s\nType: %s\nTeacher: %s\nLesson Number: %s\nGroup: %s\nDate: %s\nComment: %s\nAddress: %s\n",
			class.Title, class.Type, class.Teacher, class.LessonNumber, class.Group, class.Date, class.Comment, class.Address)
	}

	return message, nil
}

func callStudentScheduleTomorrow(jwt string) (string, error) {
	client := http.Client{}
	requestURL := fmt.Sprintf("http://localhost:8050/timetableTomorrow?token=%s", jwt)
	request, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return "", err
	}

	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return "Занятий не найдено.", nil
	}

	if response.StatusCode == http.StatusBadRequest {
		return "Недостаточно данных. Заполни профиль.", nil
	}

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}
	var classes []Class
	err = json.NewDecoder(response.Body).Decode(&classes)
	if err != nil {
		return "", err
	}
	message := "Tomorrow's schedule:\n"
	for _, class := range classes {
		message += fmt.Sprintf("\nTitle: %s\nType: %s\nTeacher: %s\nLesson Number: %s\nGroup: %s\nDate: %s\nComment: %s\nAddress: %s\n",
			class.Title, class.Type, class.Teacher, class.LessonNumber, class.Group, class.Date, class.Comment, class.Address)
	}

	return message, nil
}

func callStudentWhereTeacher(jwt string, fullName string) (string, error) {
	client := http.Client{}
	requestURL := fmt.Sprintf("http://localhost:8050/lecturerLocation?token=%s&lecturer=%s", jwt, encodeQuery(fullName))
	request, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return "", err
	}

	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return "Занятий нет.", nil
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}

	var class Class
	err = json.NewDecoder(response.Body).Decode(&class)
	if err != nil {
		return "", err
	}

	message := fmt.Sprintf("Class for teacher:\nTitle: %s\nType: %s\nTeacher: %s\nLesson Number: %s\nGroup: %s\nDate: %s\nComment: %s\nAddress: %s",
		class.Title, class.Type, class.Teacher, class.LessonNumber, class.Group, class.Date, class.Comment, class.Address)

	return message, nil
}

func callStudentExamsSchedule(jwt string) (string, error) {
	client := http.Client{}
	requestURL := fmt.Sprintf("http://localhost:8050/examsByGroup?token=%s", jwt)
	request, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return "", err
	}

	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return "Экзаменов нет.", nil
	}
	if response.StatusCode == http.StatusBadRequest {
		return "Недостаточно данных. Заполни профиль.", nil
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}
	var classes []Class
	err = json.NewDecoder(response.Body).Decode(&classes)
	if err != nil {
		return "", err
	}
	message := "Exams schedule:\n"
	for _, class := range classes {
		message += fmt.Sprintf("\nTitle: %s\nType: %s\nTeacher: %s\nLesson Number: %s\nGroup: %s\nDate: %s\nComment: %s\nAddress: %s\n",
			class.Title, class.Type, class.Teacher, class.LessonNumber, class.Group, class.Date, class.Comment, class.Address)
	}

	return message, nil
}

func callLecturerWhereNextClass(jwt string) (string, error) {
	return callStudentWhereNextClass(jwt)
}

func callLecturerScheduleWeekdays(jwt string, dayNumber int) (string, error) {
	return callStudentScheduleWeekdays(jwt, dayNumber)
}

func callLecturerScheduleToday(jwt string) (string, error) {
	return callStudentScheduleToday(jwt)
}

func callLecturerScheduleTomorrow(jwt string) (string, error) {
	return callStudentScheduleTomorrow(jwt)
}

type SetCommentRequest struct {
	Token        string `json:"token"`
	Date         string `json:"date"`
	LessonNumber string `json:"lesson_number"`
	Group        string `json:"group"`
	Comment      string `json:"comment"`
}

func callLecturerLeaveComment(jwt string, group string, lessonNumber string, comment string) (string, error) {
	return callLecturerLeaveCommentWithDate(jwt, group, lessonNumber, "", comment)
}

func callLecturerLeaveCommentWithDate(jwt string, group string, lessonNumber string, date string, comment string) (string, error) {
	requestPayload := SetCommentRequest{
		Token:        jwt,
		Date:         date,
		LessonNumber: lessonNumber,
		Group:        group,
		Comment:      comment,
	}
	requestBody, err := json.Marshal(requestPayload)
	if err != nil {
		return "", err
	}
	client := http.Client{}
	requestURL := "http://localhost:8050/setComment"
	request, err := http.NewRequest("POST", requestURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return "Занятие не найдено.", nil
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}
	return "Комментарий успешно оставлен", nil
}

func callLecturerWhereGroup(jwt string, group string) (string, error) {
	client := http.Client{}
	requestURL := fmt.Sprintf("http://localhost:8050/groupLocation?token=%s&group=%s", jwt, encodeQuery(group))
	request, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return "", err
	}

	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return "Занятий нет.", nil
	}

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}

	var class Class
	err = json.NewDecoder(response.Body).Decode(&class)
	if err != nil {
		return "", err
	}

	message := fmt.Sprintf("Class for group:\nTitle: %s\nType: %s\nTeacher: %s\nLesson Number: %s\nGroup: %s\nDate: %s\nComment: %s\nAddress: %s",
		class.Title, class.Type, class.Teacher, class.LessonNumber, class.Group, class.Date, class.Comment, class.Address)

	return message, nil
}

type SetNameRequest struct {
	ChatId   int64  `json:"chat_id"`
	FullName string `json:"full_name"`
}

func callAllChangeName(chatId int64, fullName string) (string, error) {
	requestPayload := SetNameRequest{
		ChatId:   chatId,
		FullName: fullName,
	}
	requestBody, err := json.Marshal(requestPayload)
	if err != nil {
		return "", err
	}
	client := http.Client{}
	requestURL := "http://localhost:8090/changeName"
	request, err := http.NewRequest("POST", requestURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}
	return "ФИО успешно изменено", nil
}

type SetGroupRequest struct {
	ChatId int64  `json:"chat_id"`
	Group  string `json:"group"`
}

func callAllChangeGroup(chatId int64, group string) (string, error) {
	requestPayload := SetGroupRequest{
		ChatId: chatId,
		Group:  group,
	}
	requestBody, err := json.Marshal(requestPayload)
	if err != nil {
		return "", err
	}
	client := http.Client{}
	requestURL := "http://localhost:8090/changeGroup"
	request, err := http.NewRequest("POST", requestURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}
	return "Группа успешно изменена", nil
}
