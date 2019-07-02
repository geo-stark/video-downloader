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

const PrefferedVideoWidth = 1280
const PrefferedAudioRate = 48000

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

func grabVideo(URL string) error {
	log.Print("process url:", URL)

	baseURL := URL[0:strings.LastIndex(URL, "/")]

	data, err := downloadURL(URL)
	if err != nil {
		return err
	}

	var clip Clip
	err = json.Unmarshal(data, &clip)
	if err != nil {
		return err
	}

	audioIndex := -1
	audioMaxRateIndex := 0
	for index, i := range clip.Audio {
		if i.SampleRate == PrefferedAudioRate {
			audioIndex = index
			break
		}
		if i.SampleRate > clip.Audio[audioMaxRateIndex].SampleRate {
			audioMaxRateIndex = index
		}
	}
	if audioIndex < 0 && len(clip.Audio) > 0 {
		audioIndex = audioMaxRateIndex
	}

	videoIndex := -1
	videoMaxWidthIndex := 0
	for index, i := range clip.Video {
		if i.Width == PrefferedVideoWidth {
			videoIndex = index
			break
		}
		if i.Width > clip.Video[videoMaxWidthIndex].Width {
			videoMaxWidthIndex = index
		}
	}
	if videoIndex < 0 && len(clip.Video) > 0 {
		videoIndex = videoMaxWidthIndex
	}

	log.Printf("clip: %v", clip.ID)
	log.Printf("audio index: %v", audioIndex)
	log.Printf("video index: %v", videoIndex)

	url := ""

	video := clip.Video[videoIndex]
	videoFile := clip.ID + ".video" + path.Ext(video.Segments[0].URL)
	url = normalizeURL(baseURL + "/" + clip.URL + video.URL)
	if err := downloadSegments(url,
		video.Segments,
		video.InitSegment,
		videoFile); err != nil {
		return err
	}
	audio := clip.Audio[audioIndex]
	audioFile := clip.ID + ".audio" + path.Ext(audio.Segments[0].URL)
	url = normalizeURL(baseURL + "/" + clip.URL + audio.URL)
	if err := downloadSegments(url,
		audio.Segments,
		audio.InitSegment,
		audioFile); err != nil {
		return err
	}

	if err = muxAV(videoFile, audioFile, clip.ID+".mkv"); err != nil {
		return err
	}
	log.Print("done")

	return nil
}

func main12() {
	/*	data, err := ioutil.ReadFile("data/master.json")
		if err != nil {
			log.Fatal(err)
		}
	*/
	if err := grabVideo("https://61skyfiregce-vimeo.akamaized.net/exp=1562016350~acl=%2F336337660%2F%2A~hmac=e007cdbfaa3a196d785457d29e02dccf0dfe6a63be99254edd1e86e6a9e4be0c/336337660/sep/video/1332495808,1332495803,1332495801,1332495798,1332495794/master.json?base64_init=1"); err != nil {
		log.Fatal(err)
	}

}
