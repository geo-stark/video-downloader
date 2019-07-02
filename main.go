package main

// rest:
// post jobs/add(link), return job id
// get  job/get(id), return job info
// get  jobs(), return job id list

// arguments:
// REST port
// output folder

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strconv"

	"github.com/gorilla/mux"
)

const Port = 8080

type JobStatus int

const (
	Quied    JobStatus = iota
	Progress JobStatus = iota
	Done     JobStatus = iota
	Error    JobStatus = iota
)

type Job struct {
	ID     int       `json:"id"`
	Clip   int       `json:"clip"`
	Status JobStatus `json:"status"`
	Date   int       `json:"timestamp"`
	Link   string    `json:"link"`
	Name   string    `json:"name"`
	File   string    `json:"file"`
}

type Result struct {
	Ok    bool   `json:"ok"`
	Error string `json:"error"`
	ID    int    `json:"id"`
}

var Jobs []Job
var ActiveJobCount int

func showHint() {
	body := fmt.Sprintf(`Download server page: <a href="http://localhost:%v">localhost:%v</a>`, Port, Port)
	cmd := exec.Command("notify-send", "Video downloader", body)
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

func addJob(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	res := Result{Ok: false}

	link := r.FormValue("link")
	name := r.FormValue("name")

	log.Printf("link: %v, name: %v", link, name)

	if link != "" {
		res.Ok = true

		ID := len(Jobs)
		Jobs = append(Jobs, Job{ID: ID, Clip: 0, Status: Quied, File: "", Link: link, Name: name})

	}
	json.NewEncoder(w).Encode(&res)
}

func main() {
	showHint()

	Jobs = append(Jobs, Job{ID: 0, Clip: 0, Status: Quied, File: "", Link: "www.shit.com"})

	router := mux.NewRouter()
	router.HandleFunc("/jobs", getJobs).Methods("GET")
	router.HandleFunc("/jobs/{id}", getJob).Methods("GET")
	router.HandleFunc("/jobs", addJob).Methods("POST")
	//router.HandleFunc("/jobs/{id}", deleteBook).Methods("DELETE")

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "www/index.html")
	})
	router.PathPrefix("/css").Handler(http.FileServer(http.Dir("www")))

	log.Print("server started...")
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", Port), router))
}
