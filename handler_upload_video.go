package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/videos"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const maxVideoSize = 1 << 30 // 1GB
	r.Body = http.MaxBytesReader(w, r.Body, maxVideoSize)

	videoID, err := uuid.Parse(r.PathValue("videoID"))

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to get video", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You are not authorized to upload a video for this video", err)
		return
	}

	r.ParseMultipartForm(maxVideoSize)

	videoFile, videoFileHandler, err := r.FormFile("video")

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}

	defer videoFile.Close()

	fileType, _, err := mime.ParseMediaType(videoFileHandler.Header.Get("Content-Type"))

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "couldn't parse media type", err)
		return
	}

	if fileType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Invalid mime type", err)
		return
	}

	file, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to create temporary file", err)
		return
	}
	defer os.Remove(file.Name())
	defer file.Close()

	_, err = io.Copy(file, videoFile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to copy file", err)
		return
	}

	fastStartFileStr, err := videos.ProcessVideoForFastStart(file.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to process video for fast start", err)
		return
	}
	defer os.Remove(fastStartFileStr)

	fastStartFile, err := os.Open(fastStartFileStr)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to open fast start file", err)
		return
	}
	defer fastStartFile.Close()

	videoPrefix, err := videos.GetVideoAspectRatio(fastStartFileStr)

	_, err = fastStartFile.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to seek file", err)
		return
	}

	fileName := make([]byte, 32)
	_, err = rand.Read(fileName)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to generate random file name", err)
		return
	}
	fileNameStr := fmt.Sprintf("%s/%s.mp4", videoPrefix, hex.EncodeToString(fileName))

	_, err = cfg.s3Client.PutObject(context.Background(), &s3.PutObjectInput{Bucket: &cfg.s3Bucket, Key: &fileNameStr, Body: fastStartFile, ContentType: &fileType})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to upload file", err)
		return
	}

	videoURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, fileNameStr)
	video.VideoURL = &videoURL

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)

}
