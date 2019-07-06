package main

// rest:
// post jobs/add(link), return job id
// get  job/get(id), return job info
// get  jobs(), return job id list

// arguments:
// --port
// --dir -d

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"code.cloudfoundry.org/bytefmt"
	"github.com/gorilla/mux"
	"github.com/pborman/getopt/v2"
)

const Version = 1

const MaxQueueSize = 50
const WorkerCount = 5
const DefaultPort = 8080

type JobStatus int

const (
	Quied    JobStatus = iota
	Progress JobStatus = iota
	Done     JobStatus = iota
	Error    JobStatus = iota
)

type Job struct {
	ID         int       `json:"id"`
	Clip       int       `json:"clip"`
	Status     JobStatus `json:"status"`
	Date       string    `json:"timestamp"`
	Link       string    `json:"link"`
	Name       string    `json:"name"`
	Length     string    `json:"length"` // human-readbale
	Resolution string    `json:"resolution"`
	File       string    `json:"file"`
	FileSize   string    `json:"filesize"` // human-readbale
}

type Result struct {
	Ok    bool   `json:"ok"`
	Error string `json:"error"`
	ID    int    `json:"id"`
}

var Jobs map[int]Job = make(map[int]Job)
var JobCounter int
var Lock sync.Mutex
var Channel = make(chan int, MaxQueueSize)
var Port = DefaultPort
var Dir = "data"

func showHint() {
	//body := fmt.Sprintf(`Download server page: <a href="http://localhost:%v">localhost:%v</a>`, Port, Port)
	body := fmt.Sprintf(`application page: http://localhost:%v`, Port)
	cmd := exec.Command("notify-send", "Video downloader", body)
	cmd.Run()
}

func openFile(w http.ResponseWriter, r *http.Request) {
	file, _ := url.QueryUnescape(r.FormValue("path"))
	cmd := exec.Command("xdg-open", file)
	cmd.Run()
}

func openFolder(w http.ResponseWriter, r *http.Request) {
	str, _ := filepath.Abs(Dir)
	cmd := exec.Command("caja", str)
	cmd.Run()
}

func getJobs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Jobs)
}

func getJob(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	params := mux.Vars(r)
	ID, err := strconv.Atoi(params["id"])
	if err == nil {
		for _, item := range Jobs {
			if item.ID == ID {
				json.NewEncoder(w).Encode(item)
				return
			}
		}
	}
	json.NewEncoder(w).Encode(&Job{})
}

func removeJob(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	ID, err := strconv.Atoi(params["id"])
	if err == nil {
		Lock.Lock()
		delete(Jobs, ID)
		Lock.Unlock()
	}
}

func addJob(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	res := Result{Ok: false}

	link := r.FormValue("link")
	name := r.FormValue("name")

	log.Printf("link: %v, name: %v", link, name)

	if link != "" {
		res.Ok = true

		Lock.Lock()
		ID := JobCounter
		Jobs[ID] = Job{ID: ID, Clip: 0, Status: Quied, File: "", Link: link, Name: name}
		JobCounter++
		Lock.Unlock()

		Channel <- ID
	}
	json.NewEncoder(w).Encode(&res)
}

func durationfmt(duration_ms int) string {
	name := []string{"MS", "Sec", "Min", "Hours"}
	div := []int{1000, 60, 60, 60}
	//var res string
	var index, n int

	for index, n = range div {
		if int(duration_ms) < n {
			break
		}
		duration_ms = duration_ms / n
	}
	return strconv.Itoa(duration_ms) + name[index]
}

func grab(item *Job) {
	var grabber VideoGrabber

	grabber.SetOptions(Options{
		PrefferedVideoWidth: 1280,
		PrefferedAudioRate:  4800,
		WorkingDir:          Dir})
	grabber.OpenLink(item.Link, item.Name)
	if err := grabber.FetchInfo(); err != nil {
		item.Status = Error
		log.Printf("fetch info failed %v %v", err, item.Link)
		return
	}
	item.Date = time.Now().Format("15:04 02.01.2006")
	item.Resolution = grabber.resolution
	item.Length = durationfmt(grabber.duration)

	if err := grabber.FetchData(); err != nil {
		item.Status = Error
		log.Printf("fetch data failed %v %v", err, item.Link)
		return
	}
	item.File = grabber.file
	if info, err := os.Stat(item.File); err == nil {
		item.FileSize = bytefmt.ByteSize(uint64(info.Size()))
	}
	item.Status = Done
}

func worker(in chan int, wg *sync.WaitGroup) {
	for j := range in {
		Lock.Lock()
		item, ok := Jobs[j]
		if !ok {
			Lock.Unlock()
			continue
		}
		item.Status = Progress
		Jobs[j] = item
		Lock.Unlock()

		grab(&item)

		Lock.Lock()
		Jobs[j] = item
		Lock.Unlock()
	}
}

func main() {
	argPort := getopt.IntLong("port", 'p', DefaultPort,
		"web interface TCP port")
	argDir := getopt.StringLong("dir", 'd', "", "working directory")
	argHelp := getopt.BoolLong("help", 'h', "print help")

	getopt.Parse()
	if *argHelp {
		getopt.PrintUsage(os.Stdout)
		os.Exit(0)
	}
	Port = *argPort
	Dir = *argDir

	showHint()

	var wg sync.WaitGroup
	for w := 0; w < WorkerCount; w++ {
		go worker(Channel, &wg)
	}
	wg.Add(WorkerCount)

	router := mux.NewRouter()
	router.HandleFunc("/folder", openFolder).Methods("GET")
	router.HandleFunc("/file", openFile).Methods("GET")
	router.HandleFunc("/jobs", getJobs).Methods("GET")
	router.HandleFunc("/jobs/{id}", getJob).Methods("GET")
	router.HandleFunc("/jobs", addJob).Methods("POST")
	router.HandleFunc("/jobs/{id}", removeJob).Methods("DELETE")

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "www/index.html")
	})
	router.PathPrefix("/css").Handler(http.FileServer(http.Dir("www")))

	log.Print("server started...")
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", Port), router))
}
