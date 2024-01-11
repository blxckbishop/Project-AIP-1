package main

import (
	"encoding/json"
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	SECRET_TIMETABLE_JWT = "123t"
)

func main() {
	router := http.NewServeMux()
	router.HandleFunc("/getTimetable", getTimetableRoute)
	router.HandleFunc("/setTimetable", setTimetableRoute)
	router.HandleFunc("/setComment", setCommentRoute)
	router.HandleFunc("/nextLesson", nextLessonRoute)
	router.HandleFunc("/groupLocation", groupLocationRoute)
	router.HandleFunc("/lecturerLocation", lecturerLocationRoute)
	router.HandleFunc("/timetableByDay", timetableByDayRoute)
	router.HandleFunc("/timetableToday", timetableTodayRoute)
	router.HandleFunc("/timetableTomorrow", timetableTomorrowRoute)
	router.HandleFunc("/examsByGroup", examsByGroupRoute)
	http.ListenAndServe(":8050", router)
}

// ROUTES ///////////////////////////////////////////////////

func getTimetableRoute(rw http.ResponseWriter, r *http.Request) {
	classCRUD, _ := newClassCRUD()
	classes, err := classCRUD.ReadAll()
	if err != nil {
		http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	if len(classes) > 0 {
		rw.Header().Set("Content-Type", "application/json")
		classesJSON, err := json.Marshal(classes)
		if err != nil {
			http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
			log.Println(err)
			return
		}
		rw.Write(classesJSON)
	} else {
		http.Error(rw, "Empty", http.StatusNotFound)
	}
}

func setTimetableRoute(rw http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var classes []Class
	if err := decoder.Decode(&classes); err != nil {
		http.Error(rw, "Bad Request", http.StatusBadRequest)
		log.Println(err)
		return
	}
	classCRUD, err := newClassCRUD()
	if err != nil {
		http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	deletedClasses, err := classCRUD.DeleteAll()
	if err != nil {
		http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	filteredDeletedClasses := make([]Class, 0)
	for _, deletedClass := range deletedClasses {
		if deletedClass.Comment != "" {
			filteredDeletedClasses = append(filteredDeletedClasses, deletedClass)
		}
	}
	for i, class := range classes {
		for _, filteredClass := range filteredDeletedClasses {
			if class.Group == filteredClass.Group &&
				class.LessonNumber == filteredClass.LessonNumber &&
				class.Date == filteredClass.Date {
				classes[i].Comment = filteredClass.Comment
			}
		}
	}
	for _, class := range classes {
		_, err := classCRUD.Create(class)
		if err != nil {
			http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
			log.Println(err)
			return
		}
	}
	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte("Timetable successfully set"))
}

type SetCommentRequest struct {
	Token        string `json:"token"`
	Date         string `json:"date"`
	LessonNumber string `json:"lesson_number"`
	Group        string `json:"group"`
	Comment      string `json:"comment"`
}

func setCommentRoute(wr http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var setCommentRequest SetCommentRequest
	if err := decoder.Decode(&setCommentRequest); err != nil {
		http.Error(wr, "Bad Request", http.StatusBadRequest)
		log.Println(err)
		return
	}

	if setCommentRequest.Token == "" {
		http.Error(wr, "Token parameter is required", http.StatusBadRequest)
		return
	}
	payloadJWT, err := validateJWT(setCommentRequest.Token)
	if err != nil {
		http.Error(wr, "Bad token", http.StatusForbidden)
		log.Println(err)
		return
	}

	classCRUD, err := newClassCRUD()
	if err != nil {
		http.Error(wr, "Internal Server Error", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	switch payloadJWT.Action {
	case "LecturerLeaveComment":
		if payloadJWT.FullName == "" {
			http.Error(wr, "FullName parameter is required", http.StatusBadRequest)
			return
		}
		classes, err := classCRUD.ReadByLessonNumberAndGroupAndTeacher(setCommentRequest.LessonNumber, setCommentRequest.Group, payloadJWT.FullName)
		if err != nil {
			http.Error(wr, "Not Found", http.StatusNotFound)
			log.Println(err)
			return
		}
		for i := range classes {
			classes[i].Comment = setCommentRequest.Comment
			if _, err := classCRUD.Update(classes[i]); err != nil {
				http.Error(wr, "Failed to update class", http.StatusInternalServerError)
				log.Println(err)
				return
			}
		}
	case "LecturerLeaveCommentWithDate":
		class, err := classCRUD.ReadByDateAndLessonNumberAndGroup(setCommentRequest.Date, setCommentRequest.LessonNumber, setCommentRequest.Group)
		if err != nil {
			http.Error(wr, "Not Found", http.StatusNotFound)
			log.Println(err)
			return
		}
		newClass := Class{
			Title:        class.Title,
			Type:         class.Type,
			Teacher:      class.Teacher,
			LessonNumber: class.LessonNumber,
			Group:        class.Group,
			Date:         class.Date,
			Comment:      setCommentRequest.Comment,
			Address:      class.Address,
		}
		if _, err := classCRUD.Update(newClass); err != nil {
			http.Error(wr, "Failed to update class", http.StatusInternalServerError)
			log.Println(err)
			return
		}
	default:
		http.Error(wr, "Unknown action", http.StatusForbidden)
		log.Println(err)
		return
	}

	wr.WriteHeader(http.StatusOK)
	wr.Write([]byte("User successfully update"))
}

func lecturerLocationRoute(rw http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(rw, "Token parameter is required", http.StatusBadRequest)
		return
	}
	payloadJWT, err := validateJWT(token)
	if err != nil {
		http.Error(rw, "Bad token", http.StatusForbidden)
		log.Println(err)
		return
	}

	if payloadJWT.Action != "StudentWhereTeacher" {
		http.Error(rw, "Bad action", http.StatusForbidden)
		log.Println(err)
		return
	}

	lecturer := decodeQuery(r.URL.Query().Get("lecturer"))
	if lecturer == "" {
		http.Error(rw, "Group parameter is required", http.StatusBadRequest)
		return
	}
	//currentTime := time.Date(2023, time.December, 19, 12, 23, 0, 0, time.UTC)
	currentTime := time.Now()
	var currentLessonNumber string
	if currentTime.After(time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 7, 59, 0, 0, currentTime.Location())) {
		switch {
		case currentTime.Before(time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 9, 31, 0, 0, currentTime.Location())):
			currentLessonNumber = "1"
		case currentTime.Before(time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 11, 21, 0, 0, currentTime.Location())):
			currentLessonNumber = "2"
		case currentTime.Before(time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 13, 01, 0, 0, currentTime.Location())):
			currentLessonNumber = "3"
		case currentTime.Before(time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 14, 51, 0, 0, currentTime.Location())):
			currentLessonNumber = "4"
		case currentTime.Before(time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 16, 31, 0, 0, currentTime.Location())):
			currentLessonNumber = "5"
		case currentTime.Before(time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 18, 11, 0, 0, currentTime.Location())):
			currentLessonNumber = "6"
		default:
			currentLessonNumber = "-1"
		}
	} else {
		currentLessonNumber = "-1"
	}
	today := time.Now().Format("02.01.06")
	//today := "19.12.23"
	classCRUD, _ := newClassCRUD()
	class, err := classCRUD.ReadByTeacherAndDateAndLessonNumber(lecturer, today, currentLessonNumber)
	if err != nil {
		http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	if class.Group == "" {
		http.Error(rw, "Status Not Found", http.StatusNotFound)
		return
	} else {
		classJSON, err := json.Marshal(class)
		if err != nil {
			http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
			log.Println(err)
			return
		}
		rw.Write(classJSON)
	}
}

func groupLocationRoute(rw http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(rw, "Token parameter is required", http.StatusBadRequest)
		return
	}
	payloadJWT, err := validateJWT(token)
	if err != nil {
		http.Error(rw, "Bad token", http.StatusForbidden)
		log.Println(err)
		return
	}
	if payloadJWT.Action != "LecturerWhereGroup" {
		http.Error(rw, "Bad action", http.StatusForbidden)
		log.Println(err)
		return
	}
	group := decodeQuery(r.URL.Query().Get("group"))
	if group == "" {
		http.Error(rw, "Group parameter is required", http.StatusBadRequest)
		return
	}
	//currentTime := time.Date(2023, time.December, 19, 12, 23, 0, 0, time.UTC)
	currentTime := time.Now()
	var currentLessonNumber string
	if currentTime.After(time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 7, 59, 0, 0, currentTime.Location())) {
		switch {
		case currentTime.Before(time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 9, 31, 0, 0, currentTime.Location())):
			currentLessonNumber = "1"
		case currentTime.Before(time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 11, 21, 0, 0, currentTime.Location())):
			currentLessonNumber = "2"
		case currentTime.Before(time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 13, 01, 0, 0, currentTime.Location())):
			currentLessonNumber = "3"
		case currentTime.Before(time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 14, 51, 0, 0, currentTime.Location())):
			currentLessonNumber = "4"
		case currentTime.Before(time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 16, 31, 0, 0, currentTime.Location())):
			currentLessonNumber = "5"
		case currentTime.Before(time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 18, 11, 0, 0, currentTime.Location())):
			currentLessonNumber = "6"
		default:
			currentLessonNumber = "-1"
		}
	} else {
		currentLessonNumber = "-1"
	}
	today := time.Now().Format("02.01.06")
	//today := "19.12.23"
	classCRUD, _ := newClassCRUD()
	class, err := classCRUD.ReadByGroupAndDateAndLessonNumber(group, today, currentLessonNumber)
	if err != nil {
		http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	if class.Group == "" {
		http.Error(rw, "Status Not Found", http.StatusNotFound)
		return
	} else {
		classJSON, err := json.Marshal(class)
		if err != nil {
			http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
			log.Println(err)
			return
		}
		rw.Write(classJSON)
	}

}

func nextLessonRoute(rw http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(rw, "Token parameter is required", http.StatusBadRequest)
		return
	}
	payloadJWT, err := validateJWT(token)
	if err != nil {
		http.Error(rw, "Bad token", http.StatusForbidden)
		log.Println(err)
		return
	}

	today := time.Now().Format("02.01.06")
	//today := "19.12.23"
	classCRUD, _ := newClassCRUD()
	var classes []Class

	switch payloadJWT.Action {
	case "LecturerWhereNextClass":
		if payloadJWT.FullName == "" {
			http.Error(rw, "FullName parameter is required", http.StatusBadRequest)
			return
		}
		classes, err = classCRUD.ReadByTeacherAndAfterDateInclusive(payloadJWT.FullName, today)
		if err != nil {
			http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
			log.Println(err)
			return
		}
	case "StudentWhereNextClass":
		if payloadJWT.Group == "" {
			http.Error(rw, "Group parameter is required", http.StatusBadRequest)
			return
		}
		classes, err = classCRUD.ReadByGroupAndAfterDateInclusive(payloadJWT.Group, today)
		if err != nil {
			http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
			log.Println(err)
			return
		}
	default:
		http.Error(rw, "Unknown action", http.StatusForbidden)
		log.Println(err)
		return
	}

	//currentTime := time.Date(2023, time.December, 20, 9, 10, 0, 0, time.UTC)
	currentTime := time.Now()
	var nextLessonNumber string
	switch {
	case currentTime.Before(time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 8, 0, 0, 0, currentTime.Location())):
		nextLessonNumber = "1"
	case currentTime.Before(time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 9, 50, 0, 0, currentTime.Location())):
		nextLessonNumber = "2"
	case currentTime.Before(time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 11, 30, 0, 0, currentTime.Location())):
		nextLessonNumber = "3"
	case currentTime.Before(time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 13, 20, 0, 0, currentTime.Location())):
		nextLessonNumber = "4"
	case currentTime.Before(time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 15, 0, 0, 0, currentTime.Location())):
		nextLessonNumber = "5"
	case currentTime.Before(time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 16, 40, 0, 0, currentTime.Location())):
		nextLessonNumber = "6"
	default:
		nextLessonNumber = "7"
	}
	filteredClasses := make([]Class, 0)
	for _, class := range classes {
		if class.Date == today {
			lessonNumberInt, _ := strconv.Atoi(class.LessonNumber)
			nextLessonNumberInt, _ := strconv.Atoi(nextLessonNumber)
			if lessonNumberInt >= nextLessonNumberInt {
				filteredClasses = append(filteredClasses, class)
			}
		} else {
			filteredClasses = append(filteredClasses, class)
		}
	}
	rw.Header().Set("Content-Type", "application/json")
	var nextLesson Class
	if len(filteredClasses) > 0 {
		nextLesson = filteredClasses[0]
		nextLessonJSON, err := json.Marshal(nextLesson)
		if err != nil {
			http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
			log.Println(err)
			return
		}
		rw.Write(nextLessonJSON)
	} else {
		http.Error(rw, "Empty", http.StatusNotFound)
	}
}

func examsByGroupRoute(rw http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(rw, "Token parameter is required", http.StatusBadRequest)
		return
	}
	payloadJWT, err := validateJWT(token)
	if err != nil {
		http.Error(rw, "Bad token", http.StatusForbidden)
		log.Println(err)
		return
	}

	if payloadJWT.Action != "StudentExamsSchedule" {
		http.Error(rw, "Bad action", http.StatusForbidden)
		log.Println(err)
		return
	}

	if payloadJWT.Group == "" {
		http.Error(rw, "Not enough info about group", http.StatusNotFound)
		log.Println(err)
		return
	}

	classCRUD, _ := newClassCRUD()
	classes, err := classCRUD.ReadByGroupAndType(payloadJWT.Group, "Экзамен")
	if err != nil {
		http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	classesJSON, err := json.Marshal(classes)
	if err != nil {
		http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	rw.Write(classesJSON)
}

func timetableTodayRoute(rw http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(rw, "Token parameter is required", http.StatusBadRequest)
		return
	}
	payloadJWT, err := validateJWT(token)
	if err != nil {
		http.Error(rw, "Bad token", http.StatusForbidden)
		log.Println(err)
		return
	}

	today := time.Now().Format("02.01.06")
	classCRUD, _ := newClassCRUD()

	var classes []Class

	switch payloadJWT.Action {
	case "LecturerScheduleToday":
		if payloadJWT.FullName == "" {
			http.Error(rw, "FullName parameter is required", http.StatusBadRequest)
			return
		}
		classes, err = classCRUD.ReadByDateAndTeacher(today, payloadJWT.FullName)
		if err != nil {
			http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
			log.Println(err)
			return
		}
	case "StudentScheduleToday":
		if payloadJWT.Group == "" {
			http.Error(rw, "Group parameter is required", http.StatusBadRequest)
			return
		}
		classes, err = classCRUD.ReadByDateAndGroup(today, payloadJWT.Group)
		if err != nil {
			http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
			log.Println(err)
			return
		}
	default:
		http.Error(rw, "Unknown action", http.StatusForbidden)
		log.Println(err)
		return
	}

	if len(classes) > 0 {
		rw.Header().Set("Content-Type", "application/json")
		classesJSON, err := json.Marshal(classes)
		if err != nil {
			http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
			log.Println(err)
			return
		}
		rw.Write(classesJSON)
	} else {
		http.Error(rw, "Empty", http.StatusNotFound)
	}
}

func timetableTomorrowRoute(rw http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(rw, "Token parameter is required", http.StatusBadRequest)
		return
	}
	payloadJWT, err := validateJWT(token)
	if err != nil {
		http.Error(rw, "Bad token", http.StatusForbidden)
		log.Println(err)
		return
	}

	tomorrow := time.Now().Add(24 * time.Hour).Format("02.01.06")
	classCRUD, _ := newClassCRUD()

	var classes []Class

	switch payloadJWT.Action {
	case "LecturerScheduleTomorrow":
		if payloadJWT.FullName == "" {
			http.Error(rw, "FullName parameter is required", http.StatusBadRequest)
			return
		}
		classes, err = classCRUD.ReadByDateAndTeacher(tomorrow, payloadJWT.FullName)
		if err != nil {
			http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
			log.Println(err)
			return
		}
	case "StudentScheduleTomorrow":
		if payloadJWT.Group == "" {
			http.Error(rw, "Group parameter is required", http.StatusBadRequest)
			return
		}
		classes, err = classCRUD.ReadByDateAndGroup(tomorrow, payloadJWT.Group)
		if err != nil {
			http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
			log.Println(err)
			return
		}
	default:
		http.Error(rw, "Unknown action", http.StatusForbidden)
		log.Println(err)
		return
	}

	if len(classes) > 0 {
		rw.Header().Set("Content-Type", "application/json")
		classesJSON, err := json.Marshal(classes)
		if err != nil {
			http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
			log.Println(err)
			return
		}
		rw.Write(classesJSON)
	} else {
		http.Error(rw, "Empty", http.StatusNotFound)
	}
}

func timetableByDayRoute(rw http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(rw, "Token parameter is required", http.StatusBadRequest)
		return
	}
	payloadJWT, err := validateJWT(token)
	if err != nil {
		http.Error(rw, "Bad token", http.StatusForbidden)
		log.Println(err)
		return
	}

	date := decodeQuery(r.URL.Query().Get("date"))
	classCRUD, _ := newClassCRUD()
	var classes []Class

	switch payloadJWT.Action {
	case "LecturerScheduleWeekdays":
		if payloadJWT.FullName == "" {
			http.Error(rw, "FullName parameter is required", http.StatusBadRequest)
			return
		}
		classes, err = classCRUD.ReadByDateAndTeacher(date, payloadJWT.FullName)
		if err != nil {
			http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
			log.Println(err)
			return
		}
	case "StudentScheduleWeekdays":
		if payloadJWT.Group == "" {
			http.Error(rw, "Group parameter is required", http.StatusBadRequest)
			return
		}
		classes, err = classCRUD.ReadByDateAndGroup(date, payloadJWT.Group)
		if err != nil {
			http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
			log.Println(err)
			return
		}
	default:
		http.Error(rw, "Unknown action", http.StatusForbidden)
		log.Println(err)
		return
	}

	if len(classes) > 0 {
		rw.Header().Set("Content-Type", "application/json")
		classesJSON, err := json.Marshal(classes)
		if err != nil {
			http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
			log.Println(err)
			return
		}
		rw.Write(classesJSON)
	} else {
		http.Error(rw, "Empty", http.StatusNotFound)
	}
}

// UTILS ///////////////////////////////////////////////////

type JWTPayload struct {
	GithubId int64    `bson:"github_id" json:"github_id"`
	ChatId   int64    `bson:"chat_id" json:"chat_id"`
	Roles    []string `bson:"roles" json:"roles"`
	Group    string   `bson:"group" json:"group"`
	FullName string   `bson:"full_name" json:"full_name"`
	Action   string   `bson:"action" json:"action"`
	Exp      string   `bson:"exp" json:"exp"`
}

func validateJWT(token string) (JWTPayload, error) {
	jwtToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return []byte(SECRET_TIMETABLE_JWT), nil
	})
	if err != nil {
		return JWTPayload{}, err
	}
	if claims, ok := jwtToken.Claims.(jwt.MapClaims); ok && jwtToken.Valid {
		expTime := int64(claims["exp"].(float64))
		currentTime := time.Now().Unix()

		if expTime < currentTime {
			return JWTPayload{}, errors.New("token has expired")
		}

		payload := JWTPayload{
			GithubId: int64(claims["github_id"].(float64)),
			ChatId:   int64(claims["chat_id"].(float64)),
			Roles:    getStringArrayFromInterfaceArray(claims["roles"]),
			Group:    claims["group"].(string),
			FullName: claims["full_name"].(string),
			Action:   claims["action"].(string),
			Exp:      time.Unix(expTime, 0).Format(time.RFC3339),
		}

		return payload, nil
	} else {
		return JWTPayload{}, errors.New("invalid token")
	}
}

func getStringArrayFromInterfaceArray(input interface{}) []string {
	result := []string{}
	if arr, ok := input.([]interface{}); ok {
		for _, item := range arr {
			result = append(result, item.(string))
		}
	}
	return result
}

func decodeQuery(encodedString string) string {
	decodedString, err := url.QueryUnescape(encodedString)
	if err != nil {
		return ""
	}
	return decodedString
}
