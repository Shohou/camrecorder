package main

import (
	"context"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type FileToUpload struct {
	filePathName string
	fileTime     time.Time
}

func MotionCut(ctx context.Context, cancel context.CancelFunc, filesToUpload chan FileToUpload) {
	defer cancel()
	cameraLocation, err := time.LoadLocation(cameraZoneId)
	if err != nil {
		log.Fatalf("Error while loading %s location: %s", cameraZoneId, err)
	}
	lastFileTime := time.Now().In(cameraLocation)

	for {
		select {
		case <-ctx.Done():
			return
		default: // Default is must to avoid blocking
		}

		files, err := ioutil.ReadDir(campath)
		if err != nil {
			log.Warn(err)
		}

		for _, dir := range files {
			select {
			case <-ctx.Done():
				return
			default: // Default is must to avoid blocking
			}

			if dir.IsDir() {
				dirName := dir.Name()
				year, err := strconv.Atoi(dirName[0:4])
				if err != nil {
					continue
				}
				month, err := strconv.Atoi(dirName[5:7])
				if err != nil {
					continue
				}
				day, err := strconv.Atoi(dirName[8:10])
				if err != nil {
					continue
				}
				dirDate := time.Date(year, time.Month(month), day, 0, 0, 0, 0, cameraLocation)
				lastDate := time.Date(lastFileTime.Year(), lastFileTime.Month(), lastFileTime.Day(), 0, 0, 0, 0, cameraLocation)
				if dirDate.After(lastDate) || dirDate.Equal(lastDate) {
					log.Debugf("Processing date: %s", dirDate)
					lastFileTime = processFilesForDate(ctx, campath+dir.Name(), lastFileTime, filesToUpload)
					log.Debugf("Last processed file time: %s", lastFileTime)
				}
			}
		}
		time.Sleep(time.Second * 10)
	}
}

func processFilesForDate(ctx context.Context, dirPath string, lastFileTime time.Time, filesToUpload chan FileToUpload) time.Time {
	path := dirPath + "/" + pathToEvents

	files, err := ioutil.ReadDir(path)
	if err != nil {
		log.Warn(err)
	}

	for _, file := range files {
		select {
		case <-ctx.Done():
			return lastFileTime
		default: // Default is must to avoid blocking
		}

		fileTime, err := fileToTime(file, lastFileTime.Location())
		if err == nil && fileTime.After(lastFileTime) {
			log.Infof("Found unprocessed event file: %s", file.Name())
			processedUpToTime, err := processFile(ctx, path+file.Name(), file.Name(), fileTime, filesToUpload)
			if err != nil {
				log.Errorf("Failed to process file: %s", err)
			} else {
				lastFileTime = processedUpToTime
			}
		} else if err != nil {
			log.Debugf("Skipping file %s because of error: %s", file.Name(), err)
		}
	}
	return lastFileTime
}

func fileToTime(file os.FileInfo, location *time.Location) (time.Time, error) {
	if !strings.HasSuffix(file.Name(), ".jpg") {
		return time.Time{}, errors.New("not a jpg file")
	}

	modTime := file.ModTime()
	return modTime.In(location), nil
}

func processFile(ctx context.Context, file string, fileName string, fileTime time.Time, filesToUpload chan FileToUpload) (time.Time, error) {
	now := time.Now().In(fileTime.Location())
	fileAge := now.Sub(fileTime)
	log.Debugf("File is %f seconds old", fileAge.Seconds())
	if fileAge <= time.Second*120 {
		sleepTime := time.Second*120 - fileAge
		log.Debugf("Waiting for enough time (%f seconds) to pass to find video file", sleepTime.Seconds())
		time.Sleep(sleepTime)
	}

	log.Debugf("Looking for videos for event file %s with file time %s", file, fileTime)
	files, err := ioutil.ReadDir(videoPath)
	if err != nil {
		return time.Time{}, errors.New(fmt.Sprintf("failed to read directory with videos: %s", err))
	}

	videoFiles := make([]string, 0, 3)
	lastVideoFileTime := time.Time{}
	for _, videoFile := range files {
		videoFileTime, err := videoFileToTime(videoFile.Name(), fileTime.Location())
		if err == nil && videoFileTime.After(fileTime.Add(-70*time.Second)) && videoFileTime.Before(fileTime.Add(50*time.Second)) {
			log.Debugf("Video file %s is in time range for event", videoFile.Name())
			videoFiles = append(videoFiles, videoPath+videoFile.Name())
			lastVideoFileTime = videoFileTime
		} else if err == nil && videoFileTime.Before(fileTime.Add(-24*time.Hour)) {
			log.Infof("Video file is too old, deleting: %s", videoFile.Name())
			removeErr := os.Remove(videoPath + videoFile.Name())
			if removeErr != nil {
				log.Errorf("Failed to delete file: %s", removeErr)
			}
		} else if err != nil {
			log.Errorf("Skipping video file %s because of error: %s", videoFile.Name(), err)
		} else {
			log.Debugf("Video file %s with video file time %s is NOT in time range for event", videoFile.Name(), videoFileTime)
		}
	}

	go processVideo(ctx, file, fileName, fileTime, videoFiles, filesToUpload)
	if len(videoFiles) > 0 {
		return lastVideoFileTime.Add(60 * time.Second), nil
	} else {
		return fileTime, nil
	}
}

func videoFileToTime(fileName string, location *time.Location) (time.Time, error) {
	if !strings.HasPrefix(fileName, "cam") {
		return time.Time{}, errors.New("incorrect file prefix")
	}
	year, err := strconv.Atoi(fileName[3:7])
	if err != nil {
		return time.Time{}, errors.New("file name has incorrect format: " + err.Error())
	}
	month, err := strconv.Atoi(fileName[8:10])
	if err != nil {
		return time.Time{}, errors.New("file name has incorrect format: " + err.Error())
	}
	day, err := strconv.Atoi(fileName[11:13])
	if err != nil {
		return time.Time{}, errors.New("file name has incorrect format: " + err.Error())
	}
	hour, err := strconv.Atoi(fileName[14:16])
	if err != nil {
		return time.Time{}, errors.New("file name has incorrect format: " + err.Error())
	}
	minute, err := strconv.Atoi(fileName[17:19])
	if err != nil {
		return time.Time{}, errors.New("file name has incorrect format: " + err.Error())
	}
	second, err := strconv.Atoi(fileName[20:22])
	if err != nil {
		return time.Time{}, errors.New("file name has incorrect format: " + err.Error())
	}

	return time.Date(year, time.Month(month), day, hour, minute, second, 0, time.Local).In(location), nil
}

func processVideo(ctx context.Context, eventFile string, eventFileName string, fileTime time.Time, videoFiles []string, filesToUpload chan FileToUpload) {
	eventPath := uploadPath + strconv.Itoa(fileTime.Year()) + "-" + fmt.Sprintf("%02d", fileTime.Month()) + "-" + fmt.Sprintf("%02d", fileTime.Day())
	err := os.MkdirAll(eventPath, os.FileMode(0755))
	if err != nil {
		log.Errorf("Failed to create directory %s for event: %s", eventPath, err)
		return
	}

	if len(videoFiles) > 0 {
		tmpfile, err := ioutil.TempFile("", "example")
		if err != nil {
			log.Errorf("Failed to create temporary file for ffmpeg inputs: %s", err)
		}
		defer os.Remove(tmpfile.Name())

		for _, videoFile := range videoFiles {
			tmpfile.WriteString("file '" + videoFile + "'\n")
		}
		videoFileName := eventPath + "/" + eventFileName[0:len(eventFileName)-4] + ".mkv"
		ffmpegCmd := exec.Command("ffmpeg", "-loglevel", "level+info", "-f", "concat", "-safe", "0", "-i", tmpfile.Name(), "-c", "copy", videoFileName)
		//example with transcoding
		//ffmpeg -loglevel level+info -i cam2020-02-14_00-18-50.mkv -i cam2020-02-14_00-19-50.mkv -filter_complex "[0:v:0][0:a:0][1:v:0][1:a:0]concat=n=2:v=1:a=1[outv][outa]" -map "[outv]" -map "[outa]" result_transcoded.mkv

		err = LaunchFFmpeg(ctx, eventFileName, ffmpegCmd)
		if err != nil {
			log.Debugf("["+eventFileName+"] ffmpeg finished with error: %s", err)
		}

		_, err = os.Stat(videoFileName)
		if err != nil {
			log.Errorf("Video processing for event failed, file not found: %s", err)
		}

		filesToUpload <- FileToUpload{
			filePathName: videoFileName,
			fileTime:     fileTime}
	}

	filesToUpload <- FileToUpload{
		filePathName: eventFile,
		fileTime:     fileTime}

	_, err = os.Stat(eventPath)
}
