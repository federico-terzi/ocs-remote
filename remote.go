package main

import (
	"log"
	"fmt"
	"encoding/json"
	"time"
	"io/ioutil"
	"net/http"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

var mw *walk.MainWindow
var addressTextEdit *walk.TextEdit
var connectionStatusLabel *walk.Label
var fileListBox *walk.ListBox
var downloadProgressBar *walk.ProgressBar
var downloadLabel *walk.Label

var filesModel *EnvModel

func checkConnectionStatus() bool {
	address := fmt.Sprintf("http://%s/checkocs", addressTextEdit.Text())
	resp, err := http.Get(address)
	if err != nil {
		log.Print("Error checking the address")
		return false
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print("Error checking the address")
		return false
	}

	if string(body) == "OK" {
		return true
	}

	return false
}

func connectionCheckDaemon() {
	for {
		status := checkConnectionStatus()
		message := "Status: Offline"
		if status {
			message = "Status: Connected!"
		}
		mw.Synchronize(func() {
			connectionStatusLabel.SetText(message)
		})

		time.Sleep(4 * time.Second)
	}
}

func getFiles() []string {
	var output []string

	address := fmt.Sprintf("http://%s/list", addressTextEdit.Text())
	resp, err := http.Get(address)
	if err != nil {
		log.Print("Error getting the list")
		return output
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print("Error parsing the list")
		return output
	}

	err = json.Unmarshal(body, &output)
	return output
}

func updateFileList() {
	files := getFiles()

	mw.Synchronize(func() {
		fileListBox.SetModel(NewEnvModel(files))
	})
}

func main() {
	filesModel = NewEnvModel(nil)

	_ = MainWindow{
		Title:   "Open Camera Studio Remote",
		MinSize: Size{600, 400},
		Layout:  VBox{},
		AssignTo: &mw,
		Children: []Widget{
			GroupBox{
				Title:         "Phone",
				Layout:        HBox{},
				StretchFactor: 1,
				Children: []Widget{
					HSplitter{
						Children: []Widget{
							TextEdit{
								AssignTo: &addressTextEdit,
								Text: "192.168.1.68:8000",
							},
							Label{
								AssignTo: &connectionStatusLabel,
								Text: "Status: Not connected",
							},
						},
					},
				},
			},
			GroupBox{
				Title:         "Files",
				Layout:        VBox{},
				StretchFactor: 3,
				Children: []Widget{
					ListBox{
						AssignTo: &fileListBox,
						Model:    filesModel,
					},
					Label{
						AssignTo: &downloadLabel,
						Text: "Select an action to begin the download...",
					},
					ProgressBar{
						AssignTo:      &downloadProgressBar,
						MaxValue:      100,
						MinValue:      0,
						StretchFactor: 4,
					},
					HSplitter{
						Children: []Widget{
							PushButton{
								Text: "Refresh",
								OnClicked: func() {
									go updateFileList()
								},
							},
							PushButton{
								Text: "Download Selected",
								OnClicked: func() {
									
								},
							},
							PushButton{
								Text: "Download All",
								OnClicked: func() {
									
								},
							},
						},
					},
				},
			},
			PushButton{
				Text: "SCREAM",
				OnClicked: func() {
					
				},
			},
		},
	}.Create()

	go connectionCheckDaemon()

	mw.Run()
}

type EnvItem struct {
	name  string
}

type EnvModel struct {
	walk.ListModelBase
	items []EnvItem
}

func NewEnvModel(items []string) *EnvModel {
	m := &EnvModel{items: make([]EnvItem, len(items))}

	for i, e := range items {
		m.items[i] = EnvItem{e}
	}

	return m
}

func (m *EnvModel) ItemCount() int {
	return len(m.items)
}

func (m *EnvModel) Value(index int) interface{} {
	return m.items[index].name
}