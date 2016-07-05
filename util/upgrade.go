package util

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/google/go-github/github"
)

// Adapted from github.com/apex/apex

var UpdateAvailable *github.RepositoryRelease

func DoUpgrade(version string) error {
	fmt.Printf("current release is v%s\n", version)

	if err := CheckUpdate(version); err != nil || UpdateAvailable == nil {
		return err
	}

	asset := findAsset(UpdateAvailable)
	if asset == nil {
		return errors.New("cannot find binary for your system")
	}

	// create tmp file
	tmpPath := filepath.Join(os.TempDir(), "hilbertspace-upgrade")
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0755)
	if err != nil {
		return err
	}

	// download binary
	fmt.Printf("downloading %s\n", *asset.BrowserDownloadURL)
	res, err := http.Get(*asset.BrowserDownloadURL)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// copy it down
	_, err = io.Copy(f, res.Body)
	if err != nil {
		return err
	}

	// replace it
	cmdPath := FindHilbertspace()
	if len(cmdPath) == 0 {
		return errors.New("Cannot find hilbertspace binary")
	}

	fmt.Printf("replacing %s\n", cmdPath)
	err = os.Rename(tmpPath, cmdPath)
	if err != nil {
		return err
	}

	fmt.Println("visit https://pearson.com/hilbert-space/releases for the changelog")
	go func() {
		time.Sleep(time.Second * 3)
		os.Exit(0)
	}()

	return nil
}

func FindHilbertspace() string {
	cmdPath, _ := exec.LookPath("hilbertspace")

	if len(cmdPath) == 0 {
		cmdPath, _ = filepath.Abs(os.Args[0])
	}

	return cmdPath
}

// findAsset returns the binary for this platform.
func findAsset(release *github.RepositoryRelease) *github.ReleaseAsset {
	for _, asset := range release.Assets {
		if *asset.Name == fmt.Sprintf("hilbertspace_%s_%s", runtime.GOOS, runtime.GOARCH) {
			return &asset
		}
	}

	return nil
}

func CheckUpdate(version string) error {
	// fetch releases
	gh := github.NewClient(nil)
	releases, _, err := gh.Repositories.ListReleases("gamunu", "hilbertspace", nil)
	if err != nil {
		return err
	}

	UpdateAvailable = nil
	if len(releases) > 0 && (*releases[0].TagName)[1:] != version {
		UpdateAvailable = &releases[0]
	}

	return nil
}
