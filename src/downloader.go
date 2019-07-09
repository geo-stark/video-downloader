package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"code.cloudfoundry.org/bytefmt"
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
	playlist   bool
	link       string
	name       string
	extention  string
	id         string
	resolution string
	duration   string
	clip       Clip
	opts       Options

	videoIndex int
	audioIndex int
	file       string
	fileSize   string
}

const RetryCount = 2

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

func downloadURWriter(URL string, writer io.Writer) error {
	log.Printf("getting %v", URL)
	resp, err := http.Get(URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http response not ok (%v)", resp.StatusCode)
	}
	_, err = io.Copy(writer, resp.Body)
	if err != nil {
		return err
	}
	return nil
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
		err := downloadURWriter(baseURL+i.URL, file)
		if err != nil {
			return err
		}
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
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %v", err, string(output))
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

// duration in milliseconds
func durationfmt(duration int) string {
	name := []string{"MS", "Sec", "Min", "Hours"}
	div := []int{1000, 60, 60, 60}
	var index, n int

	for index, n = range div {
		if int(duration) < n {
			break
		}
		duration = duration / n
	}
	return strconv.Itoa(duration) + name[index]
}

func (this *VideoGrabber) loadPlaylist() {
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

	this.duration = durationfmt(int(this.clip.Video[videoIndex].Duration * 1000))
	this.resolution = fmt.Sprintf("%vx%v",
		this.clip.Video[videoIndex].Width, this.clip.Video[videoIndex].Height)

	log.Printf("clip: %v", this.clip.ID)
	log.Printf("audio index: %v", audioIndex)
	log.Printf("video index: %v", videoIndex)
}

func (this *VideoGrabber) SetOptions(opts Options) {
	this.opts = opts
}

func (this *VideoGrabber) OpenLink(link, name string) {
	this.link = link
	this.name = name
}
func test() error {
	URL := "https://static.fazarosta.com/media/uploads/Ochisheniye_ot_programm[6_modul].mp3"
	//URL := "https://r1---sn-nx8xon3t-83vl.googlevideo.com/videoplayback?expire=1562646445&ei=TcMjXZynDcOgyAXc_Ym4DA&ip=46.39.228.66&id=o-AHAbUMDusLu_BnyEiqsz89Vav7U2vkKr_SWAmG_CVfai&itag=22&source=youtube&requiressl=yes&mm=31%2C29&mn=sn-nx8xon3t-83vl%2Csn-n8v7kn76&ms=au%2Crdu&mv=m&mvi=0&pl=23&initcwndbps=1627500&mime=video%2Fmp4&ratebypass=yes&dur=233.546&lmt=1562585734467288&mt=1562624742&fvip=1&c=WEB&txp=3516222&sparams=expire%2Cei%2Cip%2Cid%2Citag%2Csource%2Crequiressl%2Cmime%2Cratebypass%2Cdur%2Clmt&sig=ALgxI2wwRgIhAImg-gZJB3BmqIXcCIi0Ri90NPOrXXJCv7CTNnEoi45ZAiEAqtFxVxyr_KwRD-tOGro9Wk46WTfcrjt2BZaiA9QwAWE%3D&lsparams=mm%2Cmn%2Cms%2Cmv%2Cmvi%2Cpl%2Cinitcwndbps&lsig=AHylml4wRQIgMofi0M0Rej8O_sNJ3MC0W3urSfbNVV54_ikYRLOerKUCIQCr-iQKeGABu_OpxhZ2AzeXvvkkDVKMSxaXP-aBItBEnA%3D%3D"
	//URL := "https://94skyfiregce-vimeo.akamaized.net/exp=1562629073~acl=%2F343499093%2F%2A~hmac=a2ce49265f8d5477ac93060b768889bed0adcc79c865708b5a2192d83734ada5/343499093/sep/video/1377344913,1377344906,1377344905,1377344897,1377344895/master.json?base64_init=1"
	log.Printf("getting %v", URL)

	extention := ""
	if URL[len(URL)-4] == '.' {
		extention = URL[len(URL)-4:]
	} else {
		extention = ".mp4"
	}

	log.Printf("getting %v", extention)
	resp, err := http.Get(URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http response not ok (%v)", resp.StatusCode)
	}

	return nil
}

func (this *VideoGrabber) FetchInfo() error {
	log.Print("process url:", this.link)

	var err error
	var resp *http.Response
	for i := 0; i < RetryCount; i++ {
		resp, err = http.Get(this.link)
		if err != nil {
			continue
		}
		break
	}
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http response not ok (%v)", resp.StatusCode)
	}

	if len(resp.Header["Content-Type"]) == 0 {
		return fmt.Errorf("no content type field found")
	}

	content := resp.Header["Content-Type"][0]
	if strings.Contains(content, "json") {
		if data, err := ioutil.ReadAll(resp.Body); err == nil {
			err = json.Unmarshal(data, &this.clip)
			if err == nil {
				this.playlist = true
				this.extention = ".mkv"
				this.loadPlaylist()
				return nil
			}
		}
		return err
	}
	if strings.Contains(content, "video") || strings.Contains(content, "audio") {
		this.playlist = false
		if this.link[len(this.link)-4] == '.' {
			this.extention = this.link[len(this.link)-4:]
		} else {
			this.extention = ".mp4"
		}
		this.fileSize = bytefmt.ByteSize(uint64(resp.ContentLength))
	}
	return nil
}

func (this *VideoGrabber) FetchData() error {
	basePath := this.opts.WorkingDir
	if basePath != "" && basePath[len(basePath)-1] != '/' {
		basePath += "/"
	}
	this.file, _ = filepath.Abs(basePath + this.name + this.extention)

	if this.playlist {
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
		if err := muxAV(videoFile, audioFile, this.file); err != nil {
			return err
		}
	} else {
		file, err := os.Create(this.file)
		if err != nil {
			return err
		}
		defer file.Close()
		err = downloadURWriter(this.link, file)
		if err != nil {
			return err
		}
	}
	if info, err := os.Stat(this.file); err == nil {
		this.fileSize = bytefmt.ByteSize(uint64(info.Size()))
	}
	return nil
}
