package media

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
)

type ffprobeOutput struct {
	Streams []struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"streams"`
}

func GetVideoAspectRatio(path string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", path)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	err := cmd.Run()
	if err != nil {
		return "", err
	}

	output := ffprobeOutput{}
	err = json.Unmarshal(buf.Bytes(), &output)
	if err != nil {
		return "", err
	}

	if len(output.Streams) == 0 {
		return "", nil
	}

	width := output.Streams[0].Width
	height := output.Streams[0].Height

	if width > height {
		return "16:9", nil
	}

	if width < height {
		return "9:16", nil
	}

	return "other", nil
}

func ProcessVideoForFastStart(path string) (string, error) {
	outputPath := strings.TrimSuffix(path, ".mp4") + "_faststart.mp4"
	cmd := exec.Command("ffmpeg", "-i", path, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputPath)
	err := cmd.Run()
	if err != nil {
		return "", err
	}

	return outputPath, nil
}
