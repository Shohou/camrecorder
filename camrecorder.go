package main

import (
	"context"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
)

var (
	streamUrl string
	videoPath string

	campath      string
	pathToEvents string
	uploadPath   string
	cameraZoneId string

	backblazeAccount    string
	backblazePassword   string
	backblazeBucketName string
	bucketPrefix        string

	rootCmd = &cobra.Command{
		Use:   "camrecorder <streamUrl> <campath> <backblazeAccount>:<backblazePassword>@<backblazeBucketName>/<bucketPrefix>",
		Short: "camrecorder records stream of your camera and based on JPGs created as the result of motion detection, uploads videos to Backblaze B2 Cloud Storage",
		Long: `Camrecorder is meant to be used with cheap surveillance cameras
	                which can write JPG images based on motion detection,
                	but can't write videos. Camrecorder records the video stream
            	    with the help of FFmpeg and uploads interval of it to the
                    Backblaze B2 Cloud Storage when it finds a new JPG file saved,
                    meaning there was some motion detected.`,
		Args: cobra.ExactArgs(3),
		Run:  rootCommand,
	}
)

func init() {
	log.SetLevel(log.DebugLevel)

	rootCmd.PersistentFlags().StringVarP(&uploadPath, "result-path", "r", "/var/lib/camrecorder/events/", "path to the files with resulting video events - /var/lib/camrecorder/events")
	rootCmd.PersistentFlags().StringVarP(&videoPath, "video-path", "v", "/var/lib/camrecorder/video/", "path to video files saved from the stream, default value - /var/lib/camrecorder/video")
	rootCmd.PersistentFlags().StringVar(&pathToEvents, "path-to-events", "01/pic/", "path to JPG files after date named directory")
	rootCmd.PersistentFlags().StringVarP(&cameraZoneId, "zone-id", "z", "UTC", "zone id configured on the camera, default value - UTC")
}

func main() {
	err := rootCmd.Execute()
	if err != nil {
		log.Info(err)
	}
}

func rootCommand(cmd *cobra.Command, args []string) {
	streamUrl = args[0]
	campath = args[1]
	backblazeArgParser := regexp.MustCompile("^(\\w+):(\\w+)@(\\w+)/(.*)$")
	if !backblazeArgParser.MatchString(args[2]) {
		log.Infof("", args[2])
		cmd.Usage()
		return
	}
	backblazeParams := backblazeArgParser.FindStringSubmatch(args[2])
	backblazeAccount = backblazeParams[1]
	backblazePassword = backblazeParams[2]
	backblazeBucketName = backblazeParams[3]
	bucketPrefix = backblazeParams[4]
	campath = addSlash(campath)
	uploadPath = addSlash(uploadPath)
	videoPath = addSlash(videoPath)
	pathToEvents = addSlash(pathToEvents)

	printConfiguration()

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalChannel := make(chan os.Signal)
	signal.Notify(signalChannel, os.Interrupt, os.Kill)

	go func() {
		<-signalChannel
		cancel()
	}()

	wg.Add(3)
	go func() {
		defer wg.Done()
		RecordCamVideo(ctx, cancel)
	}()
	filesToUpload := make(chan FileToUpload, 3)
	go func() {
		defer wg.Done()
		MotionCut(ctx, cancel, filesToUpload)
	}()
	go func() {
		defer wg.Done()
		S3Upload(ctx, cancel, filesToUpload)
	}()

	wg.Wait()
}

func addSlash(path string) string {
	if !strings.HasSuffix(path, "/") && !strings.HasSuffix(path, "\\") {
		return path + "/"
	} else {
		return path
	}
}

func printConfiguration() {
	log.Info("streamUrl - " + streamUrl)
	log.Info("campath - " + campath)
	log.Info("backblazeAccount - " + backblazeAccount)
	log.Info("backblazePassword - " + backblazePassword)
	log.Info("backblazeBucketName - " + backblazeBucketName)
	log.Info("bucketPrefix - " + bucketPrefix)

	log.Info("result-path - " + uploadPath)
	log.Info("video-path - " + videoPath)
	log.Info("path-to-events - " + pathToEvents)
	log.Info("zone-id - " + cameraZoneId)
}
