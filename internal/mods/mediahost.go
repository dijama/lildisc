package mods

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotkit/app/prefs"
)

var enableMediaHost = prefs.NewBool(true, prefs.PropMeta{
	Name:        "External Media Host Fallback",
	Section:     "Mods",
	Description: "When a file is larger than Discord's upload limit, automatically upload it to litterbox.catbox.moe (72h expiry) and paste the URL into the composer instead. Falls back to 0x0.st if litterbox is unavailable.",
})

// mediaHostClient is a separate http.Client with a generous timeout because
// the shared httpClient (10s) is too short for multi-megabyte uploads.
// Cancellation goes through the request context.
var mediaHostClient = &http.Client{Timeout: 5 * time.Minute}

const (
	litterboxAPIURL  = "https://litterbox.catbox.moe/resources/internals/api.php"
	litterboxExpiry  = "72h"
	litterboxMaxSize = 1 << 30 // 1 GiB
	zeroXMaxSize     = 512 << 20 // 512 MiB

	// hostMaxResponseSize bounds how much of the host's response we'll read.
	// Both litterbox and 0x0.st return a single URL on one line; 1 KiB is
	// 4-8x what any legitimate response will be.
	hostMaxResponseSize = 1024
)

// MediaUploadRequest describes a single file to push to an external host.
// Open is called once when the upload starts; the returned reader is fully
// drained, then closed.
type MediaUploadRequest struct {
	Name     string
	MIMEType string
	Size     int64
	Open     func() (io.ReadCloser, error)
}

// UploadToMediaHost uploads the request to an external file host on a
// background goroutine and invokes onResult on the GTK main thread when
// the upload terminates. On success, url is set and err is nil; on
// failure, url is empty and err is non-nil.
//
// The upload tries litterbox.catbox.moe first (72h expiry, no auth, 1 GB
// limit), then falls back to 0x0.st (size-based persistent retention,
// 512 MB limit). Both are anonymous, no API keys, no fingerprinting risk
// beyond a plain multipart POST.
func UploadToMediaHost(ctx context.Context, req MediaUploadRequest, onResult func(url string, err error)) {
	if !enableMediaHost.Value() {
		glib.IdleAdd(func() { onResult("", errors.New("media host fallback disabled in preferences")) })
		return
	}
	if req.Size <= 0 {
		glib.IdleAdd(func() { onResult("", errors.New("file is empty")) })
		return
	}

	go func() {
		url, err := uploadWithFallback(ctx, req)
		// Always marshal the result back to the GTK main thread; consumers
		// touch widgets in the callback.
		glib.IdleAdd(func() { onResult(url, err) })
	}()
}

func uploadWithFallback(ctx context.Context, req MediaUploadRequest) (string, error) {
	if req.Size <= litterboxMaxSize {
		slog.Info("mediahost: uploading to litterbox", "name", req.Name, "size", req.Size)
		url, err := uploadToLitterbox(ctx, req)
		if err == nil {
			slog.Info("mediahost: litterbox upload succeeded", "name", req.Name, "url", url)
			return url, nil
		}
		slog.Warn("mediahost: litterbox upload failed, trying 0x0.st",
			"name", req.Name, "err", err)
	}

	if req.Size > zeroXMaxSize {
		return "", fmt.Errorf("file is %d bytes, exceeds all configured host limits", req.Size)
	}

	slog.Info("mediahost: uploading to 0x0.st", "name", req.Name, "size", req.Size)
	url, err := uploadToZeroX(ctx, req)
	if err != nil {
		slog.Warn("mediahost: 0x0.st upload failed", "name", req.Name, "err", err)
		return "", fmt.Errorf("all media hosts failed; last error: %w", err)
	}
	slog.Info("mediahost: 0x0.st upload succeeded", "name", req.Name, "url", url)
	return url, nil
}

// uploadToLitterbox does a multipart POST to the litterbox API.
//
// API: POST https://litterbox.catbox.moe/resources/internals/api.php
//
//	reqtype=fileupload
//	time=72h     (one of 1h, 12h, 24h, 72h)
//	fileToUpload=<binary>
//
// Returns the URL on success as the entire response body.
func uploadToLitterbox(ctx context.Context, req MediaUploadRequest) (string, error) {
	return doMultipartUpload(ctx, litterboxAPIURL, "fileToUpload", req, func(mw *multipart.Writer) error {
		if err := mw.WriteField("reqtype", "fileupload"); err != nil {
			return err
		}
		return mw.WriteField("time", litterboxExpiry)
	})
}

// uploadToZeroX does a multipart POST to 0x0.st.
//
// API: POST https://0x0.st
//
//	file=<binary>
//
// Returns the URL on success as the entire response body.
func uploadToZeroX(ctx context.Context, req MediaUploadRequest) (string, error) {
	return doMultipartUpload(ctx, "https://0x0.st", "file", req, nil)
}

// doMultipartUpload streams the file via an io.Pipe so we never buffer the
// whole payload in memory — important for the gigabyte case. The form
// fields written by extraFields (if any) are emitted before the file part.
func doMultipartUpload(
	ctx context.Context,
	endpoint string,
	fileFieldName string,
	req MediaUploadRequest,
	extraFields func(*multipart.Writer) error,
) (string, error) {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)

	go func() {
		// Any error from this goroutine is propagated through the pipe.
		writeErr := func(err error) {
			pw.CloseWithError(err)
		}

		if extraFields != nil {
			if err := extraFields(mw); err != nil {
				writeErr(fmt.Errorf("extra fields: %w", err))
				return
			}
		}

		rc, err := req.Open()
		if err != nil {
			writeErr(fmt.Errorf("open file: %w", err))
			return
		}
		defer rc.Close()

		fw, err := mw.CreateFormFile(fileFieldName, req.Name)
		if err != nil {
			writeErr(fmt.Errorf("create form file: %w", err))
			return
		}
		if _, err := io.Copy(fw, rc); err != nil {
			writeErr(fmt.Errorf("copy file body: %w", err))
			return
		}
		if err := mw.Close(); err != nil {
			writeErr(fmt.Errorf("close multipart writer: %w", err))
			return
		}
		pw.Close()
	}()

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, pr)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := mediaHostClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("post: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, hostMaxResponseSize+1))
	bodyStr := strings.TrimSpace(string(body))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("host returned HTTP %d: %s", resp.StatusCode, bodyStr)
	}
	if !strings.HasPrefix(bodyStr, "https://") {
		return "", fmt.Errorf("host returned unexpected body: %q", bodyStr)
	}
	return bodyStr, nil
}
