package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/media"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid video ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "couldn't validate JWT", err)
		return
	}

	const maxMemory = 1 << 30 // 1GB
	r.ParseMultipartForm(maxMemory)
	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "couldn't get video from form data", err)
		return
	}

	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	mimeType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to parse media type", err)
		return
	}

	tmpFilePath := "tubely-upload.mp4"
	tmpFile, err := os.CreateTemp("", tmpFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't create temp file", err)
		return
	}

	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	_, err = io.Copy(tmpFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to copy video to temp file", err)
		return
	}

	aspectRatio, err := media.GetVideoAspectRatio(tmpFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to determine video aspect ratio", err)
		return
	}

	var filePrefix string
	switch aspectRatio {
	case "16:9":
		filePrefix = "landscape"
	case "9:16":
		filePrefix = "portrait"
	default:
		filePrefix = "other"
	}

	tmpFile.Seek(0, io.SeekStart)
	rndKey := uuid.New().String()
	fileKey := fmt.Sprintf("%v/%v.mp4", filePrefix, rndKey)
	s3Config := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &fileKey,
		Body:        tmpFile,
		ContentType: &mimeType,
	}

	_, err = cfg.s3Client.PutObject(r.Context(), &s3Config)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to upload video to S3", err)
		return
	}

	videoUrl := fmt.Sprintf("https://%v.s3.%v.amazonaws.com/%v", cfg.s3Bucket, cfg.s3Region, fileKey)
	videoMetadata, err := cfg.db.GetVideo(videoID)
	if err != nil || videoMetadata.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "user cannot access video", err)
		return
	}

	videoMetadata.VideoURL = &videoUrl
	err = cfg.db.UpdateVideo(videoMetadata)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to update video metadata", err)
		return
	}

	w.WriteHeader(http.StatusOK)
}
