package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	api "github.com/XiaoMengXinX/NeteaseCloudApi-Go/utils"
	"github.com/urfave/cli/v2"
)

// MUSIC_A : 默认cookie值
var MUSIC_A = "4ee5f776c9ed1e4d5f031b09e084c6cb333e43ee4a841afeebbef9bbf4b7e4152b51ff20ecb9e8ee9e89ab23044cf50d1609e4781e805e73a138419e5583bc7fd1e5933c52368d9127ba9ce4e2f233bf5a77ba40ea6045ae1fc612ead95d7b0e0edf70a74334194e1a190979f5fc12e9968c3666a981495b33a649814e309366"

// MUSIC_U : 自定义cookie，优先级大于MUSIC_U
var MUSIC_U string

// delimiter : 默认musicid分隔符
var delimiter = "-"

// defaultPort : 默认http服务监听端口
var defaultPort = "8000"

func main() {
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "MUSIC_A",
				Aliases:     []string{"A"},
				Value:       "4ee5f776c9ed1e4d5f031b09e084c6cb333e43ee4a841afeebbef9bbf4b7e4152b51ff20ecb9e8ee9e89ab23044cf50d1609e4781e805e73a138419e5583bc7fd1e5933c52368d9127ba9ce4e2f233bf5a77ba40ea6045ae1fc612ead95d7b0e0edf70a74334194e1a190979f5fc12e9968c3666a981495b33a649814e309366",
				Usage:       "获取网易云评论使用的Cookie",
				Destination: &MUSIC_A,
				DefaultText: "internal",
			},
			&cli.StringFlag{
				Name:        "MUSIC_U",
				Aliases:     []string{"U"},
				Value:       "",
				Usage:       "获取网易云评论使用的Cookie",
				Destination: &MUSIC_U,
				DefaultText: "none",
			},
			&cli.StringFlag{
				Name:        "delimiter",
				Aliases:     []string{"d"},
				Value:       "-",
				Usage:       "默认MusicID分隔符",
				Destination: &delimiter,
			},
			&cli.StringFlag{
				Name: "port	",
				Aliases:     []string{"p"},
				Value:       "8000",
				Usage:       "默认http服务监听端口",
				Destination: &defaultPort,
			},
		},
		Action: func(c *cli.Context) error {
			mux := http.NewServeMux()
			mux.HandleFunc("/", Respond)
			log.Printf("Start listening at 0.0.0.0:%s", defaultPort)
			http.ListenAndServe(":"+defaultPort, mux)
			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func Respond(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	var id, responseType string
	if _, ok := r.Form["id"]; ok {
		id = string(r.Form["id"][0])
	}
	if _, ok := r.Form["type"]; ok {
		responseType = string(r.Form["type"][0])
	}
	w.WriteHeader(200)
	id = DelimiteID(id)
	if IsNum(id) {
		jsonStr, errCode := RandomComment(id)
		if jsonStr != "" {
			fmt.Fprintf(w, jsonStr)
		} else {
			switch {
			case errCode == 0:
				fmt.Fprintf(w, `{"text":"Error Code: 0\n无法获取评论，未知错误。","by":"null","from":"null","time":["null"]}`)
			case errCode == 2:
				fmt.Fprintf(w, `{"text":"Error Code: 2\n该歌曲评论为空","by":"null","from":"null","time":["null"]}`)
			case errCode == 30:
				fmt.Fprintf(w, `{"text":"Error Code: 30\n该歌曲评论缓存为空","by":"null","from":"null","time":["null"]}`)
			case errCode == 4:
				fmt.Fprintf(w, `{"text":"错误的MusicID : %v","by":"null","from":"null","time":["null"]}`, id)
			default:
				fmt.Fprintf(w, `{"text":"Error Code: %v","by":"null","from":"null","time":["null"]}`, errCode)
			}
		}
	} else {
		fmt.Fprintf(w, `{"text":"错误的MusicID : %v","by":"null","from":"null","time":["null"]}`, id)
	}
	_ = responseType
}

func RandomComment(id string) (result string, errCode int) {
	jsonBuffer, errCode := CheckCommentsCache(id)
	var jsonData []struct {
		Text string   `json:"text"`
		By   string   `json:"by"`
		From string   `json:"from"`
		Time []string `json:"time"`
	}
	if errCode == 1 {
		json.Unmarshal(jsonBuffer, &jsonData)
		rand.Seed(time.Now().UnixNano())
		randNum := rand.Intn(len(jsonData))
		randomJSON, err := json.MarshalIndent(jsonData[randNum], "", "    ")
		if err != nil {
			log.Printf("Failed to get random json - %v\n", err)
			return "", 0
		}
		return string(randomJSON), 1
	}
	return "", errCode
}

func CheckCommentsCache(id string) (result []byte, errorCode int) {
	dateToday := time.Now().Format("2006.01.02")
	CheckPathIsExist("cache/")
	CheckPathIsExist("cache/" + dateToday)
	fileName := fmt.Sprintf("cache/%v/%v.json", dateToday, id)
	fileExist := CheckFileIsExist(fileName)
	var file *os.File
	if fileExist {
		defer file.Close()
		f, err := os.Stat(fileName)
		if f.Size() == 0 {
			log.Printf("%v is empty \n", fileName)
			return nil, 30 // Error Code: 30, 文件为空
		}
		file, err = os.Open(fileName)
		if err != nil {
			log.Printf("Failed to open file \"%v\" - %v\n", fileName, err)
			return nil, 31 // Error Code: 31, 打开文件失败
		}
		jsonBuffer, err := ioutil.ReadAll(file)
		return jsonBuffer, 1
	} else {
		defer file.Close()
		file, err := os.Create(fileName)
		if err != nil {
			log.Printf("Failed to create file \"%v\" - %v\n", fileName, err)
			return nil, 32 // Error Code: 32, 创建文件失败
		}
		jsonBuffer, errCode := FetchNeteaseComments(id)
		_, err = io.WriteString(file, string(jsonBuffer))
		if err != nil {
			log.Printf("Failed to write file \"%v\" - %v\n", fileName, err)
			return nil, 33 // Error Code: 33, 写文件失败
		}
		return jsonBuffer, errCode
	}
}

func CheckFileIsExist(fileName string) bool {
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		return false
	}
	return true
}

func CheckPathIsExist(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		err := os.Mkdir(path, os.ModePerm)
		if err != nil {
			log.Printf("Create dictionary %v failed - %v\n", path, err)
		}
		return false
	}
	log.Printf("Error - %v\n", err)
	return false
}

func FetchNeteaseComments(id string) (result []byte, errorCode int) {
	var options map[string]interface{}
	options = make(map[string]interface{})
	var cookies map[string]interface{}
	cookies = make(map[string]interface{})
	if MUSIC_U != "" {
		cookies["MUSIC_U"] = MUSIC_A
	} else {
		cookies["MUSIC_A"] = MUSIC_A
	}
	options["cookie"] = cookies
	options["sortType"] = 1
	comments := api.GetSongComments(id, options)
	dateToday := time.Now().Format("2006.01.02")
	if fmt.Sprintf("%v", comments["body"].(map[string]interface{})["code"]) != "200" {
		return nil, 4
	}
	if len(comments["body"].(map[string]interface{})["data"].(map[string]interface{})["comments"].([]interface{})) < 1 {
		return nil, 2 // Error Code: 2, 该歌曲评论为空
	}
	songName := GetSongName(id)
	var jsonData []interface{} = make([]interface{}, len(comments["body"].(map[string]interface{})["data"].(map[string]interface{})["comments"].([]interface{})))
	for i := 0; i < len(comments["body"].(map[string]interface{})["data"].(map[string]interface{})["comments"].([]interface{})); i++ {
		commentContent := fmt.Sprintf("%s", comments["body"].(map[string]interface{})["data"].(map[string]interface{})["comments"].([]interface{})[i].(map[string]interface{})["content"])
		userName := fmt.Sprintf("%s", comments["body"].(map[string]interface{})["data"].(map[string]interface{})["comments"].([]interface{})[i].(map[string]interface{})["user"].(map[string]interface{})["nickname"])
		tm := time.Unix(int64(comments["body"].(map[string]interface{})["data"].(map[string]interface{})["comments"].([]interface{})[i].(map[string]interface{})["time"].(float64))/1000, 0)
		writeTime := tm.Format("2006.01.02")
		var timeArray = []string{writeTime, dateToday}
		jsonObject := struct {
			Text string   `json:"text"`
			By   string   `json:"by"`
			From string   `json:"from"`
			Time []string `json:"time"`
		}{commentContent, userName, songName, timeArray}
		jsonData[i] = jsonObject
		//fmt.Printf("%v\n%v\n%v\n%v\n", commentContent, userName, writeTime, dateToday)
	}
	jsonBuffer, err := json.MarshalIndent(jsonData, "", "    ")
	if err != nil {
		log.Printf("Failed to marshal json data - %v\n", err)
		return nil, 0 // Error Code: 0, 未知错误
	}
	return jsonBuffer, 1 // Error Code: 1, 成功
}

func GetSongName(id string) (songName string) {
	var options map[string]interface{}
	options = make(map[string]interface{})
	result := api.GetSongDetail(id, options)
	if result == nil {
		return ""
	}
	if _, ok := result["body"].(map[string]interface{})["songs"].([]interface{})[0].(map[string]interface{})["name"].(string); ok {
		songName = result["body"].(map[string]interface{})["songs"].([]interface{})[0].(map[string]interface{})["name"].(string)
	}
	return songName
}

func IsNum(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

func DelimiteID(id string) (NewID string) {
	ids := strings.Split(id, delimiter)
	rand.Seed(time.Now().UnixNano())
	randNum := rand.Intn(len(ids))
	NewID = ids[randNum]
	return NewID
}
