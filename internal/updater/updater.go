package updater

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/z1rov/z1/internal/config"
	"github.com/z1rov/z1/internal/docker"
	"github.com/z1rov/z1/internal/storage"
	"github.com/z1rov/z1/internal/ui"
)

func Install() {
	if err := config.WriteTemplate(); err != nil {
		ui.Warn("could not write config template: " + err.Error())
	}

	spin := ui.NewSpinner("fetching version")
	remote, _ := RemoteVersion()
	spin.Stop()

	ui.Banner()
	fmt.Printf("  %s[Info]%s installing %s\n\n", ui.ClrInfo, ui.ClrReset, config.ImageName)

	ui.StorageStep("Checking available disk space...")
	if err := storage.EnsureSpace(); err != nil {
		ui.StorageErr(err.Error())
		fmt.Println()
		ui.Divider()
		fmt.Println()
		os.Exit(1)
	}
	fmt.Println()

	if err := docker.Pull(); err != nil {
		ui.Error("pull failed: " + err.Error())
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("  %s[%-13s]%s%s::%s %s%s%s\n",
		ui.ClrMeta, "version", ui.ClrReset,
		ui.ClrDimStr, ui.ClrReset,
		ui.ClrOk, remote, ui.ClrReset)
	fmt.Println()
	ui.Ok("z1 installed - run: z1 start")
	ui.Divider()
	fmt.Println()
}

func Update() {
	spin := ui.NewSpinner("checking versions")
	remote, errR := RemoteVersion()
	local, errL := LocalVersion()
	spin.Stop()

	ui.Banner()
	fmt.Printf("  %s[Info]%s checking for updates\n\n", ui.ClrInfo, ui.ClrReset)

	if errR != nil {
		ui.Error("could not reach remote: " + errR.Error())
		os.Exit(1)
	}
	if errL != nil {
		fmt.Printf("  %s[%-13s]%s%s::%s %s%s%s\n",
			ui.ClrErr, "local", ui.ClrReset,
			ui.ClrDimStr, ui.ClrReset,
			ui.ClrErr, "not installed", ui.ClrReset)
		fmt.Println()
		ui.Warn("no local image found - run: z1 install")
		ui.Divider()
		fmt.Println()
		os.Exit(1)
	}

	fmt.Printf("  %s[%-13s]%s%s::%s %s%s%s\n",
		ui.ClrInfo, "local", ui.ClrReset,
		ui.ClrDimStr, ui.ClrReset,
		ui.ClrWarn, local, ui.ClrReset)
	fmt.Printf("  %s[%-13s]%s%s::%s %s%s%s\n",
		ui.ClrMeta, "remote", ui.ClrReset,
		ui.ClrDimStr, ui.ClrReset,
		ui.ClrOk, remote, ui.ClrReset)
	fmt.Println()

	if local == remote {
		fmt.Printf("  %s[Info]%s %salready up to date%s\n",
			ui.ClrInfo, ui.ClrReset, ui.ClrInfo, ui.ClrReset)
		ui.Divider()
		fmt.Println()
		return
	}

	fmt.Printf("  %s[~]%s update available: %s%s%s -> %s%s%s\n",
		ui.ClrWarn, ui.ClrReset,
		ui.ClrWarn, local, ui.ClrReset,
		ui.ClrOk, remote, ui.ClrReset)
	fmt.Println()

	ui.StorageStep("Checking available disk space...")
	if err := storage.EnsureSpace(); err != nil {
		ui.StorageErr(err.Error())
		fmt.Println()
		ui.Divider()
		fmt.Println()
		os.Exit(1)
	}
	fmt.Println()

	if err := docker.Pull(); err != nil {
		ui.Error("update failed: " + err.Error())
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("  %s[Info]%s Cleaning up old image layers...\n", ui.ClrInfo, ui.ClrReset)
	docker.PruneImages()

	fmt.Println()
	fmt.Printf("  %s[+]%s %supdated%s %s%s%s -> %s%s%s\n",
		ui.ClrOk, ui.ClrReset,
		ui.ClrInfo, ui.ClrReset,
		ui.ClrWarn, local, ui.ClrReset,
		ui.ClrOk, remote, ui.ClrReset)
	ui.Divider()
	fmt.Println()
}

func Version() {
	spin := ui.NewSpinner("fetching versions")
	remote, errR := RemoteVersion()
	local, errL := LocalVersion()
	spin.Stop()

	ui.VersionScreen(local, errL == nil, remote, errR == nil)
}

func RemoteVersion() (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(config.VersionURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	version := strings.TrimSpace(string(body))
	if version == "" {
		return "", fmt.Errorf("empty version file")
	}
	return version, nil
}

func LocalVersion() (string, error) {
	out, err := exec.Command(
		"docker", "run", "--rm", "--entrypoint", "cat",
		config.ImageName, "/z1/version/version.txt",
	).Output()
	if err != nil {
		return "", fmt.Errorf("image not found locally")
	}

	version := strings.TrimSpace(string(out))
	if version == "" {
		return "", fmt.Errorf("empty local version file")
	}
	return version, nil
}
