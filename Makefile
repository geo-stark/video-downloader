all:
	go build -o bin/video-downloader -ldflags "-w" ./src
