package docker

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/z1rov/z1/internal/config"
	"github.com/z1rov/z1/internal/ui"
)

var hostDevices = []string{
	"/dev/net/tun",
}

func hostUser() (string, string, string) {
	name := "z1user"
	if u, err := user.Current(); err == nil && u.Username != "" {
		name = u.Username
	}
	uid := strconv.Itoa(os.Getuid())
	gid := strconv.Itoa(os.Getgid())
	return name, uid, gid
}

func networkArgs(cfg *config.Config) []string {
	switch cfg.Network.Mode {
	case "bridge":
		if cfg.Network.Name != "" {
			return []string{"--network", cfg.Network.Name}
		}
		return []string{}
	case "vpn":
		args := []string{"--cap-add", "NET_ADMIN"}
		if cfg.Network.Name != "" {
			args = append([]string{"--network", cfg.Network.Name}, args...)
		}
		return args
	default:
		return []string{"--network", "host"}
	}
}
func Start(usbDevice string) {
	ui.StartHeader()

	if !ImageExists() {
		ui.Error("image not found locally - run: z1 install")
		os.Exit(1)
	}

	name, uid, gid := hostUser()

	if IsRunning() {
		ui.Warn("container already running - attaching")
		attach(name)
		return
	}

	if Exists() {
		_ = exec.Command("docker", "rm", "-f", config.ContainerName).Run()
	}

	cfg := config.Load()

	hostHome, err := os.UserHomeDir()
	if err != nil || hostHome == "" {
		hostHome = "/root"
	}
	homeShare := filepath.Join(hostHome, "z1-workspace")
	if err := os.MkdirAll(homeShare, 0777); err != nil {
		ui.Warn("could not create home share dir: " + err.Error())
	}
	_ = os.Chmod(homeShare, 0777)

	ui.StartDetail("image", config.ImageName)
	ui.StartDetail("home", homeShare)
	ui.StartDetail("user", fmt.Sprintf("%s (%s:%s)", name, uid, gid))
	ui.StartDetail("network", cfg.Network.Mode)

	args := []string{
		"run", "-dit",
		"--name", config.ContainerName,
		"--hostname", "z1",
		"--add-host", "z1:127.0.0.1",
		"--user", "root",
		"--cap-add", "SYS_TIME",
		"--security-opt", "seccomp=unconfined",
		"-e", "Z1_USER=" + name,
		"-e", "Z1_UID=" + uid,
		"-e", "Z1_GID=" + gid,
	}

	args = append(args, networkArgs(cfg)...)

	args = append(args,
		"-v", "/etc/hosts:/etc/hosts",
		"-v", homeShare+":/home/"+name,
	)

	for _, m := range cfg.Mounts {
		parts := strings.SplitN(m, ":", 3)
		if len(parts) < 2 {
			ui.Warn("skipping invalid mount: " + m)
			continue
		}
		mode := "rw"
		if len(parts) == 3 {
			mode = parts[2]
		}
		mountArg := fmt.Sprintf("%s:%s:%s", parts[0], parts[1], mode)
		args = append(args, "-v", mountArg)
		ui.StartDetail("mount", mountArg)
	}

	for k, v := range cfg.Env {
		args = append(args, "-e", k+"="+v)
		ui.StartDetail("env", k+"="+v)
	}

	if cfg.Network.Mode == "vpn" {
		for _, dev := range resolveDevices() {
			args = append(args, "--device", dev)
			ui.StartDetail("device", dev)
		}
	}

	if usbDevice != "" {
		usbPath, err := resolveUSBDevice(usbDevice)
		if err != nil {
			ui.Warn("usb passthrough failed: " + err.Error())
		} else {
			args = append(args, "--device", usbPath+":"+usbPath)
			ui.StartDetail("usb", usbDevice+" -> "+usbPath)
		}
	}

	args = append(args, config.ImageName)

	if err := runCmd("docker", args...); err != nil {
		ui.Error("failed to start container: " + err.Error())
		os.Exit(1)
	}

	if cfg.Network.Mode == "vpn" && cfg.Network.VPNConfig != "" {
		if err := connectVPN(cfg.Network.VPNConfig); err != nil {
			ui.Warn("vpn auto-connect failed: " + err.Error())
		}
	}

	ok, crashed := waitForUser(name, 60*time.Second)
	if !ok {
		if crashed {
			ui.Error("container exited during provisioning - last logs:")
			Logs(false)
			os.Exit(1)
		}
		ui.Warn("timed out waiting for user " + name + " to be provisioned inside the container")
	}

	ui.StartDone()
	attach(name)
}

func waitForUser(name string, timeout time.Duration) (bool, bool) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !IsRunning() {
			return false, true
		}
		cmd := exec.Command("docker", "exec", config.ContainerName, "id", "-u", name)
		if err := cmd.Run(); err == nil {
			return true, false
		}
		time.Sleep(300 * time.Millisecond)
	}
	return false, false
}

func resolveDevices() []string {
	var devices []string
	for _, dev := range hostDevices {
		if _, err := os.Stat(dev); err == nil {
			devices = append(devices, dev)
		} else {
			ui.Warn("device not found, skipping: " + dev)
		}
	}
	return devices
}

func resolveUSBDevice(idPair string) (string, error) {
	parts := strings.SplitN(idPair, ":", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid usb id format, expected vendor:product")
	}
	vendor := strings.ToLower(parts[0])
	product := strings.ToLower(parts[1])

	out, err := exec.Command("lsusb").Output()
	if err != nil {
		return "", fmt.Errorf("lsusb not available: %w", err)
	}

	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, vendor+":"+product) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		bus := fields[1]
		devField := strings.TrimSuffix(fields[3], ":")
		return fmt.Sprintf("/dev/bus/usb/%s/%s", bus, devField), nil
	}

	return "", fmt.Errorf("no usb device found matching %s", idPair)
}

func connectVPN(path string) error {
	base := filepath.Base(path)
	dst := "/etc/wireguard/" + base

	cpCmd := exec.Command("docker", "cp", path, config.ContainerName+":"+dst)
	if err := cpCmd.Run(); err != nil {
		return fmt.Errorf("copy vpn config: %w", err)
	}

	upCmd := exec.Command("docker", "exec", "-u", "root", config.ContainerName, "wg-quick", "up", dst)
	upCmd.Stdout = os.Stdout
	upCmd.Stderr = os.Stderr
	return upCmd.Run()
}

func attach(userName string) {
	cfg := config.Load()
	shell := cfg.Shell
	if shell == "" {
		shell = "zsh"
	}

	if !IsRunning() {
		ui.Error("container is not running")
		return
	}

	ok, crashed := waitForUser(userName, 20*time.Second)
	if !ok {
		if crashed {
			ui.Error("container exited before shell attach - last logs:")
		} else {
			ui.Error("user " + userName + " is not ready inside the container yet - last logs:")
		}
		Logs(false)
		return
	}

	cmd := exec.Command("docker", "exec", "-it", "-u", userName, config.ContainerName, shell)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		ui.Error("failed to attach shell: " + err.Error())
	}
}

func Stop() {
	ui.StopHeader()

	if !Exists() {
		ui.Warn("container does not exist")
		return
	}

	if err := runCmd("docker", "stop", config.ContainerName); err != nil {
		ui.Error("failed to stop container: " + err.Error())
		os.Exit(1)
	}

	ui.StopDone()
}

func Status() {
	if !Exists() {
		ui.Warn("container does not exist - run: z1 start")
		return
	}

	state := "stopped"
	if IsRunning() {
		state = "running"
	}

	ui.KV("container", config.ContainerName, ui.ClrInfo)
	ui.KV("state", state, statusColor(state))
}

func Logs(follow bool) {
	if !Exists() {
		ui.Error("container does not exist - run: z1 start")
		os.Exit(1)
	}

	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	args = append(args, config.ContainerName)

	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

func Exec(args []string) {
	if !IsRunning() {
		ui.Error("container is not running - run: z1 start")
		os.Exit(1)
	}

	name, _, _ := hostUser()
	dockerArgs := append([]string{"exec", "-it", "-u", name, config.ContainerName}, args...)

	cmd := exec.Command("docker", dockerArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

func Pull() error {
	cmd := exec.Command("docker", "pull", config.ImageName)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return err
	}

	buf := make([]byte, 4096)
	var line []byte
	for {
		n, rerr := stdout.Read(buf)
		if n > 0 {
			for _, b := range buf[:n] {
				if b == '\n' || b == '\r' {
					if len(line) > 0 {
						printPullLine(string(line))
						line = line[:0]
					}
					continue
				}
				line = append(line, b)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			break
		}
	}
	if len(line) > 0 {
		printPullLine(string(line))
	}

	return cmd.Wait()
}

func printPullLine(line string) {
	id := line
	status := line
	for i := 0; i < len(line); i++ {
		if line[i] == ':' {
			id = line[:i]
			if i+2 <= len(line) {
				status = line[i+2:]
			}
			break
		}
	}
	ui.LayerLine(id, status)
}

func PruneImages() {
	_ = runCmd("docker", "image", "prune", "-f")
}

func FullCleanup() {
	if Exists() {
		_ = runCmd("docker", "rm", "-f", config.ContainerName)
	}
	_ = runCmd("docker", "rmi", "-f", config.ImageName)
	ui.Ok("removed z1 container and image")
}

func ImageExists() bool {
	out, err := exec.Command("docker", "images", "-q", config.ImageName).Output()
	if err != nil {
		return false
	}
	return len(out) > 0
}

func Exists() bool {
	out, err := exec.Command("docker", "ps", "-a", "-q", "-f", "name=^"+config.ContainerName+"$").Output()
	if err != nil {
		return false
	}
	return len(out) > 0
}

func IsRunning() bool {
	out, err := exec.Command("docker", "ps", "-q", "-f", "name=^"+config.ContainerName+"$").Output()
	if err != nil {
		return false
	}
	return len(out) > 0
}

func statusColor(state string) string {
	if state == "running" {
		return ui.ClrOk
	}
	return ui.ClrWarn
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %v: %w", name, args, err)
	}
	return nil
}
