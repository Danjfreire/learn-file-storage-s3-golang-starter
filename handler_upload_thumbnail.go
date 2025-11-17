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

	const maxMemory = 10 << 20 // 10MB
	r.ParseMultipartForm(maxMemory)
	file, header, err := r.FormFile("thumbnail")

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't get thumbnail from form data", err)
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")

	if contentType != "image/png" && contentType != "image/jpeg" {
		respondWithError(w, http.StatusBadRequest, "Unsupported thumbnail format", nil)
		return
	}

	videoMetadata, err := cfg.db.GetVideo(videoID)

	if err != nil || videoMetadata.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "User cannot access video", err)
		return
	}

	fileId := make([]byte, 32)
	rand.Read(fileId)
	encodedFileId := base64.RawURLEncoding.EncodeToString(fileId)

	fileExtension := strings.Split(contentType, "/")[1]
	fileName := fmt.Sprintf("%v-%v", encodedFileId, fileExtension)
	filePath := filepath.Join(cfg.assetsRoot, fileName)

	thumbnailFile, err := os.Create(filePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to create thumbnail file", err)
		return
	}
	defer thumbnailFile.Close()

	_, err = io.Copy(thumbnailFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to save thumbnail file", err)
		return
	}

	thumbnailUrl := fmt.Sprintf("http://localhost:%v/assets/%v", cfg.port, fileName)
	videoMetadata.ThumbnailURL = &thumbnailUrl
	err = cfg.db.UpdateVideo(videoMetadata)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to update video metadata", err)
		return
	}

	respondWithJSON(w, http.StatusOK, videoMetadata)
}
