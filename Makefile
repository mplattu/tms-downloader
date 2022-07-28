# Give path of your go executable
# GO=/usr/lib/go-1.10/bin/go
GO=go

tms-downloader: src/*
	cd src/tms-downloader && GOOS=linux GOARCH=amd64 CGO_ENABLED=1 $(GO) build -o ../../bin/tms-downloader tms-downloader
