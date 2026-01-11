package videos

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

type Stream struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type VideoData struct {
	Stream []Stream `json:"streams"`
}

func GetVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	var cmdOutput bytes.Buffer
	cmd.Stdout = &cmdOutput

	err := cmd.Run()
	if err != nil {
		return "", err
	}

	var videoData VideoData

	err = json.Unmarshal(cmdOutput.Bytes(), &videoData)

	if err != nil {
		return "", err
	}

	if len(videoData.Stream) == 0 {
		return "", nil
	}

	gcf := gcf(videoData.Stream[0].Width, videoData.Stream[0].Height)

	if videoData.Stream[0].Width/gcf > videoData.Stream[0].Height/gcf {
		return "landscape", nil
	} else {
		return "portrait", nil
	}

}

func ProcessVideoForFastStart(filePath string) (string, error) {
	newPath := "/tmp/processing-tubely-upload.mp4"
	fmt.Printf("%v", filePath)
	fmt.Printf("%v", newPath)
	cmd := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-y", newPath)
	err := cmd.Run()

	if err != nil {
		return "", err
	}

	return newPath, nil

}
