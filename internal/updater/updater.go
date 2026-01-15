package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/x402-Systems/entropy/internal/config"
)

const githubAPIURL = "https://api.github.com/repos/x402-Systems/entropy/releases/latest"

type GithubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func CheckUpdateAvailable() (bool, string, error) {
	release, err := getLatestRelease()
	if err != nil {
		return false, "", err
	}

	current := config.Version
	if !strings.HasPrefix(current, "v") {
		current = "v" + current
	}

	return strings.TrimPrefix(release.TagName, "v") != strings.TrimPrefix(current, "v"), release.TagName, nil
}

func getLatestRelease() (*GithubRelease, error) {
	resp, err := http.Get(githubAPIURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("github API error: %s", resp.Status)
	}

	var release GithubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	return &release, nil
}

func RunUpdate() error {
	release, err := getLatestRelease()
	if err != nil {
		return err
	}

	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}

	var downloadURL string
	for _, asset := range release.Assets {
		name := strings.ToLower(asset.Name)
		if strings.Contains(name, runtime.GOOS) &&
			strings.Contains(name, runtime.GOARCH) &&
			strings.HasSuffix(name, ext) {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("no binary found for %s %s with %s", runtime.GOOS, runtime.GOARCH, ext)
	}

	fmt.Printf("📥 Fetching release %s...\n", release.TagName)

	cwd, _ := os.Getwd()
	tempDir := filepath.Join(cwd, ".entropy_update_tmp")
	_ = os.RemoveAll(tempDir)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create update dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	archivePath := filepath.Join(tempDir, "package"+ext)
	if err := downloadFile(downloadURL, archivePath); err != nil {
		return err
	}

	if ext == ".zip" {
		if err := extractZip(archivePath, tempDir); err != nil {
			return err
		}
	} else {
		if err := extractTarGz(archivePath, tempDir); err != nil {
			return err
		}
	}

	var newBinPath string
	filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && (info.Name() == "entropy" || info.Name() == "entropy.exe") {
			newBinPath = path
		}
		return nil
	})

	if newBinPath == "" {
		return fmt.Errorf("binary not found in archive")
	}

	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	oldPath := execPath + ".old"
	_ = os.Remove(oldPath)

	if err := os.Rename(execPath, oldPath); err != nil {
		return fmt.Errorf("failed to move current binary: %v", err)
	}

	if err := os.Rename(newBinPath, execPath); err != nil {
		os.Rename(oldPath, execPath)
		return fmt.Errorf("failed to swap binary: %v", err)
	}

	if runtime.GOOS != "windows" {
		os.Chmod(execPath, 0755)
	}

	os.Remove(oldPath)
	return nil
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func extractTarGz(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dest, header.Name)
		if header.Typeflag == tar.TypeReg {
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			io.Copy(outFile, tr)
			outFile.Close()
		}
	}
	return nil
}

func extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		path := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}
		io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
	}
	return nil
}
