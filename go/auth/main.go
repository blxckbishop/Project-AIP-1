package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	CLIENT_ID            = "106e58507513887a6bf2"
	CLIENT_SECRET        = "ab34fa44a1a10b6c50fa45da190269d79229ec91"
	SECRET_TIMETABLE_JWT = "123t"
	SECRET_ADMIN_JWT     = "123a"
)

var authSessions = make(map[int64]bool)

func main() {
	router := http.NewServeMux()

	router.HandleFunc("/auth", authRoute)
	router.HandleFunc("/oauth", oauthRoute)
	router.HandleFunc("/grantJWT", grantJWTRoute)
	router.HandleFunc("/getAllUsers", getAllUsersRoute)
	router.HandleFunc("/deleteUser", deleteUserRoute)
	router.HandleFunc("/changeRoles", changeRolesRoute)
	router.HandleFunc("/changeName", changeNameRoute)
	router.HandleFunc("/changeGroup", changeGroupRoute)
	http.ListenAndServe(":8090", router)
}

var rolePermittedActionsMap = map[string]map[string]string{
	"admin": {
		"AdminSessionStart":      SECRET_ADMIN_JWT,
		"AdminSessionStartToken": SECRET_ADMIN_JWT,
	},
	"lecturer": {
		"LecturerWhereNextClass":       SECRET_TIMETABLE_JWT,
		"LecturerScheduleWeekdays":     SECRET_TIMETABLE_JWT,
		"LecturerScheduleToday":        SECRET_TIMETABLE_JWT,
		"LecturerScheduleTomorrow":     SECRET_TIMETABLE_JWT,
		"LecturerLeaveComment":         SECRET_TIMETABLE_JWT,
		"LecturerLeaveCommentWithDate": SECRET_TIMETABLE_JWT,
		"LecturerWhereGroup":           SECRET_TIMETABLE_JWT,
	},
	"student": {
		"StudentWhereNextClass":   SECRET_TIMETABLE_JWT,
		"StudentScheduleWeekdays": SECRET_TIMETABLE_JWT,
		"StudentScheduleToday":    SECRET_TIMETABLE_JWT,
		"StudentScheduleTomorrow": SECRET_TIMETABLE_JWT,
		"StudentWhereTeacher":     SECRET_TIMETABLE_JWT,
		"StudentExamsSchedule":    SECRET_TIMETABLE_JWT,
	},
}

// ROUTES ///////////////////////////////////////////////////

func authRoute(w http.ResponseWriter, r *http.Request) {
	chatId, _ := strconv.ParseInt(r.URL.Query().Get("chat_id"), 10, 64)
	authSessions[chatId] = true
	var authURL string = "https://github.com/login/oauth/authorize?client_id=" + CLIENT_ID + "&state=" + strconv.FormatInt(chatId, 10)
	fmt.Fprintf(w, "%s", authURL)
}

func oauthRoute(w http.ResponseWriter, r *http.Request) {
	responseHtml := "<html><body><h1>Вы не аутентифицированы!</h1></body></html>"
	code := r.URL.Query().Get("code")
	chatId, err := strconv.ParseInt(r.URL.Query().Get("state"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}
	_, ok := authSessions[chatId]
	if code != "" && ok {
		accessToken, err := getAccessToken(code)
		if err != nil {
			http.Error(w, "Failed to get access token", http.StatusInternalServerError)
			return
		}
		userData, err := getUserData(accessToken)
		if err != nil {
			http.Error(w, "Failed to get user data", http.StatusInternalServerError)
			return
		}
		userCRUD, _ := newUserCRUD()
		user, err := userCRUD.ReadByGithubId(userData.Id)
		if err != nil {
			newUser := User{
				GithubId: userData.Id,
				ChatId:   chatId,
				Roles:    []string{"student"},
				About: About{
					FullName: "",
					Group:    "",
				},
			}
			if _, err := userCRUD.Create(newUser); err != nil {
				http.Error(w, "Failed to create user", http.StatusInternalServerError)
				return
			}
		}
		responseHtml = "<html><body><h1>Вы аутентифицированы!</h1></body></html>"
		notifyTelegramBotAboutSuccessAuth(w, user)
		delete(authSessions, chatId)
	}
	fmt.Fprint(w, responseHtml)
}

// запросить может только доверенный токен
func grantJWTRoute(rw http.ResponseWriter, req *http.Request) {
	action := req.URL.Query().Get("action")
	if action == "" {
		http.Error(rw, "action parameter is required", http.StatusBadRequest)
		return
	}
	userCRUD, err := newUserCRUD()
	if err != nil {
		http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	githubId := req.URL.Query().Get("github_id")
	chatId := req.URL.Query().Get("chat_id")

	var user User

	if githubId != "" {
		githubIdInt, err := strconv.ParseInt(githubId, 10, 64)
		if err != nil {
			fmt.Println("Ошибка преобразования:", err)
			return
		}
		user, err = userCRUD.ReadByGithubId(githubIdInt)
		if err != nil {
			http.Error(rw, "Not Found", http.StatusNotFound)
			log.Println(err)
			return
		}
	} else if chatId != "" {
		chatIdInt, err := strconv.ParseInt(chatId, 10, 64)
		if err != nil {
			fmt.Println("Ошибка преобразования:", err)
			return
		}
		user, err = userCRUD.ReadByChatId(chatIdInt)
		if err != nil {
			http.Error(rw, "Not Found", http.StatusNotFound)
			log.Println(err)
			return
		}
	} else {
		http.Error(rw, "github_id or chat_id parameter is required", http.StatusBadRequest)
		return
	}

	isAllowed, secret := isActionAllowed(user.Roles, action)
	if !isAllowed {
		http.Error(rw, "Forbidden", http.StatusForbidden)
		return
	}

	tokenString, err := generateJWT(user, action, secret)
	if err != nil {
		http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	rw.Write([]byte(tokenString))
}

func getAllUsersRoute(rw http.ResponseWriter, req *http.Request) {
	userCRUD, _ := newUserCRUD()
	users, err := userCRUD.ReadAll()
	if err != nil {
		http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	usersJSON, err := json.Marshal(users)
	if err != nil {
		http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	rw.Write(usersJSON)
}

type DeleteUserRequest struct {
	ChatId int64 `json:"chat_id"`
}

func deleteUserRoute(rw http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var deleteUserRequest DeleteUserRequest
	if err := decoder.Decode(&deleteUserRequest); err != nil {
		http.Error(rw, "Bad Request", http.StatusBadRequest)
		log.Println(err)
		return
	}
	userCRUD, err := newUserCRUD()
	if err != nil {
		http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	_, err = userCRUD.Delete(deleteUserRequest.ChatId)
	if err != nil {
		http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte("User successfully delete"))
}

type ChangeRolesUserRequest struct {
	ChatId int64    `json:"chat_id"`
	Roles  []string `json:"roles"`
}

func changeRolesRoute(wr http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var changeRolesUserRequest ChangeRolesUserRequest
	if err := decoder.Decode(&changeRolesUserRequest); err != nil {
		http.Error(wr, "Bad Request", http.StatusBadRequest)
		log.Println(err)
		return
	}
	userCRUD, err := newUserCRUD()
	if err != nil {
		http.Error(wr, "Internal Server Error", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	user, err := userCRUD.ReadByChatId(changeRolesUserRequest.ChatId)
	if err != nil {
		http.Error(wr, "Not Found", http.StatusNotFound)
		log.Println(err)
		return
	}
	newUser := User{
		GithubId: user.GithubId,
		ChatId:   user.ChatId,
		Roles:    changeRolesUserRequest.Roles,
		About:    user.About,
	}
	if _, err := userCRUD.Update(newUser); err != nil {
		http.Error(wr, "Failed to update user", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	wr.WriteHeader(http.StatusOK)
	wr.Write([]byte("User successfully update"))
}

type SetNameRequest struct {
	ChatId   int64  `json:"chat_id"`
	FullName string `json:"full_name"`
}

func changeNameRoute(wr http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var setNameRequest SetNameRequest
	if err := decoder.Decode(&setNameRequest); err != nil {
		http.Error(wr, "Bad Request", http.StatusBadRequest)
		log.Println(err)
		return
	}
	userCRUD, err := newUserCRUD()
	if err != nil {
		http.Error(wr, "Internal Server Error", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	user, err := userCRUD.ReadByChatId(setNameRequest.ChatId)
	if err != nil {
		http.Error(wr, "Not Found", http.StatusNotFound)
		log.Println(err)
		return
	}
	newUser := User{
		GithubId: user.GithubId,
		ChatId:   user.ChatId,
		Roles:    user.Roles,
		About: About{
			FullName: setNameRequest.FullName,
			Group:    user.About.Group,
		},
	}
	if _, err := userCRUD.Update(newUser); err != nil {
		http.Error(wr, "Failed to update user", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	wr.WriteHeader(http.StatusOK)
	wr.Write([]byte("User successfully update"))
}

type SetGroupRequest struct {
	ChatId int64  `json:"chat_id"`
	Group  string `json:"group"`
}

func changeGroupRoute(wr http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var setGroupRequest SetGroupRequest
	if err := decoder.Decode(&setGroupRequest); err != nil {
		http.Error(wr, "Bad Request", http.StatusBadRequest)
		log.Println(err)
		return
	}
	userCRUD, err := newUserCRUD()
	if err != nil {
		http.Error(wr, "Internal Server Error", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	user, err := userCRUD.ReadByChatId(setGroupRequest.ChatId)
	if err != nil {
		http.Error(wr, "Not Found", http.StatusNotFound)
		log.Println(err)
		return
	}
	newUser := User{
		GithubId: user.GithubId,
		ChatId:   user.ChatId,
		Roles:    user.Roles,
		About: About{
			FullName: user.About.FullName,
			Group:    setGroupRequest.Group,
		},
	}
	if _, err := userCRUD.Update(newUser); err != nil {
		http.Error(wr, "Failed to update user", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	wr.WriteHeader(http.StatusOK)
	wr.Write([]byte("User successfully update"))
}

// UTILS ///////////////////////////////////////////////////

func notifyTelegramBotAboutSuccessAuth(w http.ResponseWriter, user User) {
	jsonData, err := json.Marshal(user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	url := "http://localhost:8091/auth"
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
}

func getAccessToken(code string) (string, error) {
	requestURL := "https://github.com/login/oauth/access_token"
	form := url.Values{}
	form.Set("client_id", CLIENT_ID)
	form.Set("client_secret", CLIENT_SECRET)
	form.Set("code", code)
	request, err := http.NewRequest("POST", requestURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	var responseJson struct {
		AccessToken string `json:"access_token"`
	}
	err = json.NewDecoder(response.Body).Decode(&responseJson)
	if err != nil {
		return "", err
	}
	return responseJson.AccessToken, nil
}

type UserData struct {
	Id int64 `json:"id"`
}

func getUserData(accessToken string) (*UserData, error) {
	requestURL := "https://api.github.com/user"
	request, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch user data, status code: %d", response.StatusCode)
	}

	var userData UserData
	err = json.NewDecoder(response.Body).Decode(&userData)
	if err != nil {
		return nil, err
	}

	return &userData, nil
}

func isActionAllowed(roles []string, action string) (bool, string) {
	for _, role := range roles {
		if actions, ok := rolePermittedActionsMap[role]; ok {
			if secretKey, exists := actions[action]; exists {
				return true, secretKey
			}
		}
	}
	return false, ""
}

func generateJWT(user User, action string, secret string) (string, error) {
	claims := jwt.MapClaims{
		"github_id": user.GithubId,
		"chat_id":   user.ChatId,
		"roles":     user.Roles,
		"group":     user.About.Group,
		"full_name": user.About.FullName,
		"action":    action,
		"exp":       time.Now().Add(5 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))

	if err != nil {
		return "", err
	}
	return tokenString, nil
}
