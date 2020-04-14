package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"time"
)

func RecordCamVideo(ctx context.Context, cancel context.CancelFunc) {
	defer cancel()
	for {
		ffmpegCmd := exec.Command("ffmpeg", "-use_wallclock_as_timestamps", "1", "-i", streamUrl,
			"-vcodec", "copy", "-acodec", "copy", "-map", "0", "-f", "segment", "-segment_time", "60", "-strftime", "1",
			"-reset_timestamps", "1", "-loglevel", "level+info", "-nostats", videoPath+"cam%Y-%m-%d_%H-%M-%S.mkv")

		signalChannel := make(chan os.Signal)
		signal.Notify(signalChannel, os.Interrupt, os.Kill)

		go func() {
			<-signalChannel
			if ffmpegCmd.Process != nil {
				fmt.Printf("Killing stream ffmpeg")
				ffmpegCmd.Process.Kill()
			}
		}()

		err := LaunchFFmpeg(ctx, "stream", ffmpegCmd)
		if err != nil {
			log.Debugf("[stream] ffmpeg finished with error: %s", err)
			time.Sleep(time.Second * 10)
		}
	}
}

func LaunchFFmpeg(ctx context.Context, logPrefix string, ffmpegCmd *exec.Cmd) error {
	stdoutPipe, err := ffmpegCmd.StdoutPipe()
	if err != nil {
		return errors.New("Failed to open stdout pipe: " + err.Error())
	}

	stderrPipe, err := ffmpegCmd.StderrPipe()
	if err != nil {
		return errors.New("Failed to open stderr pipe: " + err.Error())
	}

	log.Debugf("["+logPrefix+"] Running command: %s", ffmpegCmd)
	if err := ffmpegCmd.Start(); err != nil {
		return errors.New("Failed to start ffmpeg command: " + err.Error())
	}

	stdout := bufio.NewScanner(stdoutPipe)
	stderr := bufio.NewScanner(stderrPipe)

	go logstd(logPrefix, stdout)
	go logstd(logPrefix, stderr)

	go func() {
		ffmpegCmd.Wait()
	}()

	for ffmpegCmd.ProcessState == nil || !ffmpegCmd.ProcessState.Exited() {
		select {
		case <-ctx.Done():
			return ffmpegCmd.Process.Kill()
		default: // Default is must to avoid blocking
		}
		time.Sleep(time.Second * 1)
	}

	exitCode := ffmpegCmd.ProcessState.ExitCode()
	if exitCode != 0 {
		return errors.New("ffmpeg command exited with exit code: " + strconv.Itoa(exitCode))
	}
	log.Debugf("[" + logPrefix + "] ffmpeg finished successfully")
	return nil
}

func logstd(logPrefix string, stdout *bufio.Scanner) {
	for stdout.Scan() {
		log.Info("[" + logPrefix + "] " + stdout.Text())
	}
	if err := stdout.Err(); err != nil {
		log.Errorf("["+logPrefix+"] error: %s", err)
	}
}
