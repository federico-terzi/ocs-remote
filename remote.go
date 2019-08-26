package main

import (
	"log"
	"fmt"
	"encoding/json"
	"time"
	"io/ioutil"
	"io"
	"os"
	"net/url"
	"net/http"
	"github.com/dustin/go-humanize"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

// TO COMPILE
/*
rsrc -manifest remote.manifest -o rsrc.syso -ico icon.ico
go build
.\ocs-remote.exe
*/

var mw *walk.MainWindow
var addressTextEdit *walk.TextEdit
var connectionStatusLabel *walk.Label
var fileListBox *walk.ListBox
var downloadProgressBar *walk.ProgressBar
var downloadLabel *walk.Label
var sequenceTextEdit *walk.TextEdit
var downloadFolderTextEdit *walk.TextEdit

var filesModel *EnvModel

func getAddress(path string) string {
	return fmt.Sprintf("http://%s/%s", addressTextEdit.Text(), path)
}

func checkConnectionStatus() bool {
	address := getAddress("checkocs")
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

type ListedFile struct {
	Name string `json:name`
	Size int64  `json:size`
}

func getFiles() []ListedFile {
	var output []ListedFile

	address := getAddress("list")
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
		filesModel = NewEnvModel(files)
		fileListBox.SetModel(filesModel)
	})
}

type DownloadEntry struct {
	Url string
	Name string
	Size int64
}
var downloadQueue = make(chan DownloadEntry, 200)

func downloadWorker() {
	for {
		download := <-downloadQueue
		fmt.Println("Download "+download.Url)
		DownloadFile(download.Name, download.Url, download.Size)
	}
}

type WriteCounter struct {
	Size uint64
	Total uint64
	Count int
	Name string
}

func (wc *WriteCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.Total += uint64(n)
	wc.Count++
	if wc.Count > 100 || wc.Total == wc.Size {
		progress := int((float32(wc.Total) / float32(wc.Size))*100)
		wc.Count = 0

		mw.Synchronize(func() {
			downloadLabel.SetText(fmt.Sprintf("Downloading %s: %s / %s, [ %d video remaining ]", wc.Name, humanize.Bytes(wc.Total), humanize.Bytes(wc.Size), len(downloadQueue)))
			downloadProgressBar.SetValue(progress)
		})
	}
	return n, nil
}

func DownloadFile(filepath string, url string, size int64) error {

	// Create the file, but give it a tmp file extension, this means we won't overwrite a
	// file until it's downloaded, but we'll remove the tmp extension once downloaded.
	out, err := os.Create(downloadFolderTextEdit.Text() + "\\" + filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create our progress reporter and pass it to be used alongside our writer
	counter := &WriteCounter{Size: uint64(size), Name: filepath}
	_, err = io.Copy(out, io.TeeReader(resp.Body, counter))
	if err != nil {
		return err
	}

	return nil
}

func deleteVideo(name string) bool {
	address := getAddress("delete?file="+url.QueryEscape(name))
	resp, err := http.Get(address)
	if err != nil {
		log.Print("Error deleting the file")
		return false
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print("Error deleting the file")
		return false
	}

	if string(body) == "OK" {
		return true
	}

	return false
}

func startRecording(name string) bool {
	address := getAddress("start?name="+url.QueryEscape(name))
	resp, err := http.Get(address)
	if err != nil {
		log.Print("Error starting the recording")
		return false
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print("Error starting the recording")
		return false
	}

	if string(body) == "OK" {
		return true
	}

	return false
}

func stopRecording() bool {
	address := getAddress("stop")
	resp, err := http.Get(address)
	if err != nil {
		log.Print("Error stop the recording")
		return false
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print("Error stop the recording")
		return false
	}

	if string(body) == "OK" {
		return true
	}

	return false
}

func main() {
	filesModel = NewEnvModel(nil)

	_ = MainWindow{
		Title:   "Open Camera Studio Remote",
		Size: Size{600, 600},
		Layout:  VBox{},
		AssignTo: &mw,
		Children: []Widget{
			GroupBox{
				Title:         "Phone",
				Layout:        HBox{},
				StretchFactor: 0,
				Children: []Widget{
					TextEdit{
						AssignTo: &addressTextEdit,
						Text: "192.168.1.68:8000",
						CompactHeight: true,
					},
					Label{
						AssignTo: &connectionStatusLabel,
						Text: "Status: Not connected",
					},
				},
			},
			GroupBox{
				Title:         "Files",
				Layout:        VBox{},
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
					TextEdit{
						AssignTo: &downloadFolderTextEdit,
						Text: "D:\\Downloads",
						CompactHeight: true,
					},
					GroupBox{
						Title:         "Actions",
						Layout:        HBox{},
						StretchFactor: 0,
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
									i := fileListBox.CurrentIndex()
									if i < 0 {
										return
									}

									downloadLink := fmt.Sprintf("%s%s", getAddress("download?file="), url.QueryEscape(filesModel.items[i].name))
									downloadQueue <- DownloadEntry{Url: downloadLink, Name: filesModel.items[i].name, Size: filesModel.items[i].size}
								},
							},
							PushButton{
								Text: "Download All",
								OnClicked: func() {
									for _, e := range filesModel.items {
										downloadLink := fmt.Sprintf("%s%s", getAddress("download?file="), url.QueryEscape(e.name))
										downloadQueue <- DownloadEntry{Url: downloadLink, Name: e.name, Size: e.size}
									}
								},
							},
							PushButton{
								Text: "Delete Selected",
								OnClicked: func() {
									i := fileListBox.CurrentIndex()
									if i < 0 {
										return
									}

									res := walk.MsgBox(mw, "Delete Confirmation", "Are you sure to delete the selected video?", walk.MsgBoxYesNo)
									if res == 6 {
										deleteVideo(filesModel.items[i].name)
									}

									go updateFileList()
								},
							},
							PushButton{
								Text: "Delete All",
								OnClicked: func() {
									res := walk.MsgBox(mw, "Delete Confirmation", "Are you sure to delete all the videos?", walk.MsgBoxYesNo)
									if res != 6 {
										return
									}
									
									res = walk.MsgBox(mw, "Delete Confirmation", "Are you REALLY sure to delete all the videos?", walk.MsgBoxYesNo)
									if res != 6 {
										return
									}

									for _, e := range filesModel.items {
										deleteVideo(e.name)
									}
									
									go updateFileList()
								},
							},
						},
					},
				},
			},
			GroupBox{
				Title:         "Recording",
				Layout:        HBox{},
				StretchFactor: 1,
				Children: []Widget{
					TextEdit{
						AssignTo: &sequenceTextEdit,
						Text: "sequence1",
						Font: Font{
							Family:    "Arial",
							PointSize: 20,
						},
						CompactHeight: true,
					},
				},
			},
			GroupBox{
				Title:         "Controls",
				Layout:        HBox{},
				StretchFactor: 2,
				Children: []Widget{
					PushButton{
						Text: "Start",
						Font: Font{
							Family:    "Arial",
							PointSize: 20,
							Bold:      true,
						},
						OnClicked: func() {
							startRecording(sequenceTextEdit.Text())
						},
					},
					PushButton{
						Text: "Retry",
						Font: Font{
							Family:    "Arial",
							PointSize: 20,
							Bold:      true,
						},
						OnClicked: func() {
							stopRecording()
							startRecording(sequenceTextEdit.Text())
							go updateFileList()
						},
					},
					PushButton{
						Text: "Stop",
						Font: Font{
							Family:    "Arial",
							PointSize: 20,
							Bold:      true,
						},
						OnClicked: func() {
							stopRecording()
							go updateFileList()
						},
					},
				},
			},
		},
	}.Create()

	go connectionCheckDaemon()
	go downloadWorker()

	mw.Run()
}

type EnvItem struct {
	name  string
	size  int64
}

type EnvModel struct {
	walk.ListModelBase
	items []EnvItem
}

func NewEnvModel(items []ListedFile) *EnvModel {
	m := &EnvModel{items: make([]EnvItem, len(items))}

	for i, e := range items {
		m.items[i] = EnvItem{e.Name, e.Size}
	}

	return m
}

func (m *EnvModel) ItemCount() int {
	return len(m.items)
}

func (m *EnvModel) Value(index int) interface{} {
	return fmt.Sprintf("%s [ %s ]", m.items[index].name, humanize.Bytes(uint64(m.items[index].size)))
}