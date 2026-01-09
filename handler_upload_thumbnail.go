package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	// TODO: implement the upload here
	const max_memory = 10 << 20

	r.ParseMultipartForm(max_memory)

	file, handler, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}

	defer file.Close()

	fileExt := handler.Header.Get("Content-Type")
	// Parse the extension from Content-Type header (e.g., "image/png" -> "png")
	parts := strings.Split(fileExt, "/")
	if len(parts) > 1 {
		fileExt = parts[1]
	}

	if fileExt != "png" && fileExt != "jpg" && fileExt != "jpeg" {
		respondWithError(w, http.StatusBadRequest, "Invalid file extension", err)
		return
	}
	image, err := io.ReadAll(file)

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to read file", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to get video", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You are not authorized to upload a thumbnail for this video", err)
		return
	}

	fileName := make([]byte, 32)
	_, err = rand.Read(fileName)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to generate random file name", err)
		return
	}
	fileNameStr := base64.URLEncoding.EncodeToString(fileName)

	thumbnailPath := filepath.Join(cfg.assetsRoot, fmt.Sprintf("%s.%s", fileNameStr, fileExt))

	os.WriteFile(thumbnailPath, image, 0777)

	thumbnailURL := fmt.Sprintf("http://localhost:%s/assets/%s.%s", cfg.port, fileNameStr, fileExt)

	video.ThumbnailURL = &thumbnailURL

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
