package main

import (
	"log"
	"fmt"
	"encoding/json"
	"time"
	"io/ioutil"
	"io"
	"os"
	"strings"
	"net/url"
	"net/http"
	"github.com/dustin/go-humanize"
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
		fmt.Printf("%d\n", wc.Total)

		mw.Synchronize(func() {
			downloadLabel.SetText(fmt.Sprintf("Downloading %s: %s / %s", wc.Name, humanize.Bytes(wc.Total), humanize.Bytes(wc.Size)))
			downloadProgressBar.SetValue(progress)
		})
	}
	return n, nil
}

func (wc WriteCounter) PrintProgress() {
	// Clear the line by using a character return to go back to the start and remove
	// the remaining characters by filling it with spaces
	fmt.Printf("\r%s", strings.Repeat(" ", 35))

	// Return again and print current status of download
	// We use the humanize package to print the bytes in a meaningful way (e.g. 10 MB)
	fmt.Printf("\rDownloading... %d complete", wc.Total)
}

// DownloadFile will download a url to a local file. It's efficient because it will
// write as it downloads and not load the whole file into memory. We pass an io.TeeReader
// into Copy() to report progress on the download.
func DownloadFile(filepath string, url string, size int64) error {

	// Create the file, but give it a tmp file extension, this means we won't overwrite a
	// file until it's downloaded, but we'll remove the tmp extension once downloaded.
	out, err := os.Create(filepath)
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