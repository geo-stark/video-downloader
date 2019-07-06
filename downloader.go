package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/goware/urlx"
)

type Segment struct {
	Start float32 `json:"start"`
	End   float32 `json:"end"`
	URL   string  `json:"url"`
}

type Audio struct {
	ID                 string    `json:"clip_id"`
	URL                string    `json:"base_url"`
	Format             string    `json:"format"`
	MIME               string    `json:"mime_type"`
	Codecs             string    `json:"codecs"`
	Bitarte            float32   `json:"bitrate"`
	AvgBitarte         float32   `json:"avg_bitrate"`
	Duration           float32   `json:"duration"`
	Channels           int       `json:"channels"`
	SampleRate         int       `json:"sample_rate"`
	MaxSegmentDuration int       `json:"max_segment_duration"`
	InitSegment        string    `json:"init_segment"`
	Segments           []Segment `json:"segments"`
}

type Video struct {
	ID                 string    `json:"clip_id"`
	URL                string    `json:"base_url"`
	Format             string    `json:"format"`
	MIME               string    `json:"mime_type"`
	Codecs             string    `json:"codecs"`
	Bitarte            float32   `json:"bitrate"`
	AvgBitarte         float32   `json:"avg_bitrate"`
	Duration           float32   `json:"duration"`
	FramteRate         int       `json:"frame_rate"`
	Width              int       `json:"width"`
	Height             int       `json:"height"`
	MaxSegmentDuration int       `json:"max_segment_duration"`
	InitSegment        string    `json:"init_segment"`
	Segments           []Segment `json:"segments"`
}
type Clip struct {
	ID    string  `json:"clip_id"`
	URL   string  `json:"base_url"`
	Video []Video `json:"video"`
	Audio []Audio `json:"audio"`
}

type Options struct {
	PrefferedVideoWidth int
	PrefferedAudioRate  int
	WorkingDir          string
}

type VideoGrabber struct {
	link       string
	name       string
	id         string
	resolution string
	duration   int
	clip       Clip
	opts       Options

	videoIndex int
	audioIndex int
	file       string
}

//const PrefferedVideoWidth = 1280
//const PrefferedAudioRate = 48000

func downloadURL(URL string) ([]byte, error) {
	log.Printf("getting %v", URL)
	resp, err := http.Get(URL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http response not ok (%v)", resp.StatusCode)
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return bodyBytes, nil
}

func downloadSegments(baseURL string, list []Segment, init string, fileName string) error {
	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	header, err := base64.StdEncoding.DecodeString(init)
	if err != nil {
		return err
	}
	file.Write(header)

	for _, i := range list {
		data, err := downloadURL(baseURL + i.URL)
		if err != nil {
			return err
		}
		file.Write(data)
	}
	return nil
}

func muxAV(video, audio, output string) error {
	os.Remove(output)
	cmd := exec.Command("ffmpeg",
		"-i", video,
		"-i", audio,
		"-c", "copy",
		output)
	if err := cmd.Run(); err != nil {
		return err
	}

	os.Remove(audio)
	os.Remove(video)
	return nil
}

func normalizeURL(rawURL string) string {
	result, _ := urlx.Parse(rawURL)
	clenURL, _ := urlx.Normalize(result)
	return clenURL
}

func (this *VideoGrabber) SetOptions(opts Options) {
	this.opts = opts
}

func (this *VideoGrabber) OpenLink(link, name string) {
	this.link = link
	this.name = name
}

func (this *VideoGrabber) FetchInfo() error {
	log.Print("process url:", this.link)

	data, err := downloadURL(this.link)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, &this.clip)
	if err != nil {
		return err
	}

	audioIndex := -1
	audioMaxRateIndex := 0
	for index, i := range this.clip.Audio {
		if i.SampleRate == this.opts.PrefferedAudioRate {
			audioIndex = index
			break
		}
		if i.SampleRate > this.clip.Audio[audioMaxRateIndex].SampleRate {
			audioMaxRateIndex = index
		}
	}
	if audioIndex < 0 && len(this.clip.Audio) > 0 {
		audioIndex = audioMaxRateIndex
	}

	videoIndex := -1
	videoMaxWidthIndex := 0
	for index, i := range this.clip.Video {
		if i.Width == this.opts.PrefferedVideoWidth {
			videoIndex = index
			break
		}
		if i.Width > this.clip.Video[videoMaxWidthIndex].Width {
			videoMaxWidthIndex = index
		}
	}
	if videoIndex < 0 && len(this.clip.Video) > 0 {
		videoIndex = videoMaxWidthIndex
	}

	this.audioIndex = audioIndex
	this.videoIndex = videoIndex

	this.duration = int(this.clip.Video[videoIndex].Duration * 1000)
	this.resolution = fmt.Sprintf("%vx%v",
		this.clip.Video[videoIndex].Width, this.clip.Video[videoIndex].Height)

	log.Printf("clip: %v", this.clip.ID)
	log.Printf("audio index: %v", audioIndex)
	log.Printf("video index: %v", videoIndex)

	return nil
}

func (this *VideoGrabber) FetchData() error {
	basePath := this.opts.WorkingDir
	if basePath != "" && basePath[len(basePath)-1] != '/' {
		basePath += "/"
	}

	baseURL := this.link[0:strings.LastIndex(this.link, "/")]
	url := ""

	video := this.clip.Video[this.videoIndex]
	videoFile := basePath + this.clip.ID + ".video" + path.Ext(video.Segments[0].URL)
	url = normalizeURL(baseURL + "/" + this.clip.URL + video.URL)
	if err := downloadSegments(url,
		video.Segments,
		video.InitSegment,
		videoFile); err != nil {
		return err
	}
	audio := this.clip.Audio[this.audioIndex]
	audioFile := basePath + this.clip.ID + ".audio" + path.Ext(audio.Segments[0].URL)
	url = normalizeURL(baseURL + "/" + this.clip.URL + audio.URL)
	if err := downloadSegments(url,
		audio.Segments,
		audio.InitSegment,
		audioFile); err != nil {
		return err
	}
	this.file, _ = filepath.Abs(basePath + this.name + ".mkv")
	if err := muxAV(videoFile, audioFile, this.file); err != nil {
		return err
	}
	return nil
}
