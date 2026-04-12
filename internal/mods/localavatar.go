package mods

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
)

const localAvatarFilename = "my_avatar.webp"

// LocalAvatarPath returns the path to the locally cached user avatar,
// or empty string if it doesn't exist yet.
func LocalAvatarPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	path := filepath.Join(dir, "lildisc", localAvatarFilename)
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}

// FetchAndSaveAvatar downloads the avatar for the given user ID and hash
// using an authenticated request. Discord CDN may require auth for the
// user's own avatar. Safe to call from a goroutine.
func FetchAndSaveAvatar(token, userID, avatarHash string) {
	if userID == "" || avatarHash == "" {
		return
	}

	dir, err := os.UserConfigDir()
	if err != nil {
		slog.Warn("FetchAndSaveAvatar: no config dir", "err", err)
		return
	}

	destDir := filepath.Join(dir, "lildisc")
	os.MkdirAll(destDir, 0o755)
	destPath := filepath.Join(destDir, localAvatarFilename)

	// Try multiple formats with auth header.
	base := "https://cdn.discordapp.com/avatars/" + userID + "/" + avatarHash
	urls := []string{
		base + ".png?size=256",
		base + ".webp?size=256",
		base + ".gif?size=256",
		base + ".png",
		base + ".webp",
		base + ".gif",
	}

	for _, url := range urls {
		slog.Info("FetchAndSaveAvatar: trying", "url", url)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			slog.Debug("FetchAndSaveAvatar: not found", "status", resp.StatusCode, "url", url)
			continue
		}

		f, err := os.Create(destPath)
		if err != nil {
			resp.Body.Close()
			slog.Warn("FetchAndSaveAvatar: create failed", "err", err)
			return
		}

		// Cap at 8MB. Discord avatars are at most ~256KB; anything larger is
		// either a misbehaving CDN response or hostile input we shouldn't write.
		_, copyErr := io.Copy(f, io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		f.Close()

		if copyErr != nil {
			os.Remove(destPath)
			slog.Warn("FetchAndSaveAvatar: write failed", "err", copyErr)
			return
		}

		slog.Info("FetchAndSaveAvatar: saved", "path", destPath, "source", url)
		return
	}

	slog.Warn("FetchAndSaveAvatar: all formats failed",
		"user_id", userID, "avatar_hash", avatarHash)
}

// RefreshAvatar deletes the locally cached avatar so it gets re-fetched
// on next startup.
func RefreshAvatar() {
	dir, err := os.UserConfigDir()
	if err != nil {
		return
	}
	path := filepath.Join(dir, "lildisc", localAvatarFilename)
	os.Remove(path)
	slog.Info("RefreshAvatar: cleared local avatar cache")
}
