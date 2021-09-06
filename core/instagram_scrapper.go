package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type InstagramScrapper struct {
	UserIDs []string
	StartupData
}

type StartupData struct {
	HMAC   string   `json:"hmac"`
	Cookie string   `json:"cookie"`
	AppID  string   `json:"appid"`
	Users  []string `json:"users"`
	//in minutes
	SleepInterval int `json:"interval"`
}

func (is *InstagramScrapper) SetStartUpData(startupData StartupData) {
	is.Cookie = startupData.Cookie
	is.AppID = startupData.AppID
	is.HMAC = startupData.HMAC
	is.UserIDs = startupData.Users
	is.SleepInterval = startupData.SleepInterval
}

// Interval is the time between each search in Instagram
func (is *InstagramScrapper) Run() {
	for {
		for _, userID := range is.UserIDs {
			is.processUser(userID, Followers)
			is.processUser(userID, Following)
		}

		duration := time.Minute * time.Duration(is.SleepInterval)

		fmt.Printf("Sleeping for: %v \n", duration)

		time.Sleep(duration)
	}
}

func (is *InstagramScrapper) processUser(userID string, listType ListType) {
	outUpdatedFollowers, outError := is.readInstagramList(userID, listType)
	lastSavedFollowers, outFileReadingErrors := is.loadLastUserMap(userID, listType)

	userMap, err := UserMap{}, errors.New("")
	userMapFromLastFile, err := UserMap{}, errors.New("")

	if err != nil {

	}

	for i := 0; i < 2; i++ {
		select {
		case userMap = <-outUpdatedFollowers:
		case err = <-outError:
			fmt.Printf("Error: %s\n", err.Error())
		case userMapFromLastFile = <-lastSavedFollowers:
		case err = <-outFileReadingErrors:
			fmt.Printf("Error: %s\n", err.Error())
		}
	}

	is.verifyDiffs(userMap, userMapFromLastFile)
}

func writeLog(file *os.File, content string) error {
	_, err := fmt.Fprintf(
		file,
		"%s: %s",
		time.Now().Format("2006-01-02 15:04:05"),
		content,
	)

	return err
}

func getUserLogFile(userID string) (*os.File, error) {
	return os.OpenFile(
		fmt.Sprintf("%s.log", userID),
		os.O_CREATE|os.O_RDWR|os.O_APPEND,
		0755,
	)
}

func writeComparisionLog(userID string, newUsers []*User, notPresent []*User, listType ListType) error {

	file, err := getUserLogFile(userID)

	defer file.Close()

	listTypeString := GetListTypeDescription(listType)

	if len(newUsers) > 0 {
		writeLog(file, fmt.Sprintf("New users in %s list: \n", listTypeString))

		for _, user := range newUsers {
			writeLog(file, fmt.Sprintf("Username: %s\t Fullname: %s\t\n", user.Username, user.FullName))
		}

	} else {
		err = writeLog(file, fmt.Sprintf("No new users in %s list\n", listTypeString))
	}

	if len(notPresent) > 0 {
		writeLog(file, fmt.Sprintf("Users that disapeared in %s list:\n", listTypeString))

		for _, user := range notPresent {
			writeLog(file, fmt.Sprintf("Username: %s\t Fullname: %s\t\n", user.Username, user.FullName))
		}

	} else {
		err = writeLog(file, fmt.Sprintf("No user disappeared in %s list\n", listTypeString))
	}

	return err
}

func (is *InstagramScrapper) verifyDiffs(updatedList UserMap, dumpedList UserMap) {

	var newUsers []*User
	var unfollowed []*User

	for k, v := range updatedList.m {
		if _, ok := dumpedList.m[k]; !ok {
			newUsers = append(newUsers, v)
		}
	}

	for k, v := range dumpedList.m {
		if _, ok := updatedList.m[k]; !ok {
			unfollowed = append(unfollowed, v)
		}
	}

	writeComparisionLog(
		updatedList.userID,
		newUsers,
		unfollowed,
		updatedList.ListType,
	)

	if len(newUsers) > 0 || len(unfollowed) > 0 {
		is.dumpUserMap(updatedList)
	}
}

func (is *InstagramScrapper) AddUser(userID string) {
	is.UserIDs = append(is.UserIDs, userID)
}

func (is *InstagramScrapper) readInstagramList(userID string, listType ListType) (<-chan UserMap, <-chan error) {
	outUsers := make(chan UserMap)
	outError := make(chan error)

	userMap := UserMap{
		userID:   userID,
		ListType: listType,
		m:        make(map[string]*User),
	}

	go func() {

		url := fmt.Sprintf(
			"https://i.instagram.com/api/v1/friendships/%s/%s/?count=5000&search_surface=follow_list_page",
			userID,
			GetListTypeDescription(listType),
		)

		client := &http.Client{}

		req, err := http.NewRequest(
			"GET",
			url,
			nil,
		)

		if err != nil {
			outError <- errors.New("error creating the request")
		}

		req.Header.Add("Cookie", is.Cookie)
		req.Header.Add("X-IG-WWW-Claim", is.HMAC)
		req.Header.Add("X-IG-App-ID", is.AppID)

		resp, err := client.Do(req)

		if err != nil {
			outError <- errors.New("error doing the request")
		}

		data, err := ioutil.ReadAll(resp.Body)

		if err != nil {
			outError <- errors.New("error reading response body")
		}

		userContainer := UserContainer{}

		json.Unmarshal(data, &userContainer)

		for _, v := range userContainer.Users {
			userMap.m[v.Username] = v
		}

		outUsers <- userMap

	}()

	return outUsers, outError
}

func (is *InstagramScrapper) dumpUserMap(userMap UserMap) error {
	file, err := os.OpenFile(
		fmt.Sprintf("%s_%d_%d.json", userMap.userID, userMap.ListType, time.Now().Unix()),
		os.O_APPEND|os.O_CREATE|os.O_RDWR,
		0755,
	)

	if err != nil {
		return err
	}

	var users []*User

	for _, v := range userMap.m {
		users = append(users, v)
	}

	json.NewEncoder(file).Encode(users)

	return nil
}

func getNewestFile(userID string, listType ListType) string {
	var lastFile string = ""
	var lastDate time.Time = time.Now().Add(time.Hour * 24 * -365)

	filepath.Walk("./", func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			if strings.Contains(info.Name(), fmt.Sprintf("%s_%d_", userID, listType)) {
				if strings.Contains(info.Name(), ".json") {
					if newDate := info.ModTime(); newDate.After(lastDate) {
						lastDate = newDate
						lastFile = info.Name()
					}
				}
			}

		}

		return nil
	})

	return lastFile
}

func (is *InstagramScrapper) loadLastUserMap(userID string, listType ListType) (<-chan UserMap, <-chan error) {
	outUsers := make(chan UserMap)
	outErr := make(chan error)

	go func() {
		lastFile := getNewestFile(userID, listType)

		if lastFile != "" {

			var users []*User

			file, err := os.OpenFile(lastFile, os.O_RDONLY, 0755)

			if err != nil {
				outErr <- fmt.Errorf("error: %v", err)
				return
			}

			json.NewDecoder(file).Decode(&users)

			userMap := UserMap{
				userID:   userID,
				ListType: listType,
				m:        make(map[string]*User),
			}

			for _, v := range users {
				userMap.m[v.Username] = v
			}

			outUsers <- userMap
		} else {
			outErr <- errors.New("found no map to read from disk.")
		}
	}()

	return outUsers, outErr
}

func NewInstagramScrapper() *InstagramScrapper {
	return &InstagramScrapper{}
}
