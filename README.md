# Overview

Camrecorder is meant to be used with cheap surveillance cameras which can write JPG images based on motion detection, but can't write videos.
Camrecorder records the video stream with the help of FFmpeg and uploads interval of it to the Backblaze B2 Cloud Storage when it finds a new JPG file saved,
meaning there was some motion detected.

#Usage

```
Usage:
  camrecorder <streamUrl> <campath> <backblazeAccount>:<backblazePassword>@<backblazeBucketName>/<bucketPrefix> [flags]

Flags:
  -h, --help                    help for camrecorder
      --path-to-events string   path to JPG files after date named directory (default "01/pic")
  -r, --result-path string      path to the files with resulting video events - /var/lib/camrecorder/events (default "/var/lib/camrecorder/events")
  -v, --video-path string       path to video files saved from the stream, default value - /var/lib/camrecorder/video (default "/var/lib/camrecorder/video")
  -z, --zone-id string          zone id configured on the camera, default value - UTC (default "UTC")
```
