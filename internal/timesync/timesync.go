package timesync

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/z1rov/z1/internal/config"
	"github.com/z1rov/z1/internal/docker"
	"github.com/z1rov/z1/internal/ui"
)

func Sync(target string) {
	if target == "" {
		ui.Error("usage: z1 synctime <dc-ip>")
		os.Exit(1)
	}

	if os.Getuid() != 0 {
		ui.Banner()
		ui.StorageErr("this command requires root privileges")
		fmt.Printf("\n  %s[Fix]%s Re-run as:\n\n", ui.ClrInfo, ui.ClrReset)
		fmt.Printf("    %ssudo z1 synctime %s%s\n\n", ui.ClrOk, target, ui.ClrReset)
		ui.Divider()
		fmt.Println()
		os.Exit(1)
	}

	if !docker.ImageExists() {
		ui.Error("z1 image not found locally - run: z1 install")
		os.Exit(1)
	}

	if !docker.Exists() {
		ui.Error("z1 container does not exist - run: z1 start")
		os.Exit(1)
	}

	if !docker.IsRunning() {
		ui.Error("z1 container is not running - run: z1 start")
		os.Exit(1)
	}

	ui.Banner()
	fmt.Printf("  %s[Info]%s Clock synchronization with %s%s%s\n\n", ui.ClrInfo, ui.ClrReset, ui.ClrWarn, target, ui.ClrReset)

	ui.StorageStep("Disabling host NTP sync...")
	if err := runCmd("timedatectl", "set-ntp", "false"); err != nil {
		ui.StorageWarn("could not disable host NTP: " + err.Error())
	} else {
		ui.StorageOk("host NTP disabled")
	}

	ui.StorageStep(fmt.Sprintf("Running ntpdate against %s...", target))
	if err := runCmd("docker", "exec", config.ContainerName, "ntpdate", target); err != nil {
		ui.StorageErr("ntpdate failed: " + err.Error())
		fmt.Println()
		ui.Divider()
		fmt.Println()
		os.Exit(1)
	}
	ui.StorageOk("clock synchronized")

	fmt.Println()
	ui.StorageWarn("host NTP remains disabled to keep the clock aligned with the target")
	fmt.Printf("  %s[*]%s Re-enable it when you're done with: %sz1 synctime restore%s\n", ui.ClrDimStr, ui.ClrReset, ui.ClrOk, ui.ClrReset)

	fmt.Println()
	ui.Divider()
	fmt.Println()
}

func Restore() {
	if os.Getuid() != 0 {
		ui.Banner()
		ui.StorageErr("this command requires root privileges")
		fmt.Printf("\n  %s[Fix]%s Re-run as:\n\n", ui.ClrInfo, ui.ClrReset)
		fmt.Printf("    %ssudo z1 synctime restore%s\n\n", ui.ClrOk, ui.ClrReset)
		ui.Divider()
		fmt.Println()
		os.Exit(1)
	}

	ui.Banner()
	ui.StorageStep("Re-enabling host NTP sync...")
	if err := runCmd("timedatectl", "set-ntp", "true"); err != nil {
		ui.StorageErr("could not re-enable host NTP: " + err.Error())
		fmt.Println()
		ui.Divider()
		fmt.Println()
		os.Exit(1)
	}
	ui.StorageOk("host NTP sync re-enabled")

	fmt.Println()
	ui.Divider()
	fmt.Println()
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
