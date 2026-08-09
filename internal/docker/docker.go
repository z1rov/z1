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

func ensureContainerUser(name, uid, gid string) error {
	script := fmt.Sprintf(`
Z1_USER=%s
Z1_UID=%s
Z1_GID=%s
if ! getent group "$Z1_USER" >/dev/null 2>&1; then
    groupadd -g "$Z1_GID" "$Z1_USER"
fi
if ! id -u "$Z1_USER" >/dev/null 2>&1; then
    useradd -m -u "$Z1_UID" -g "$Z1_GID" -s /bin/zsh "$Z1_USER"
fi
usermod -aG sudo "$Z1_USER" 2>/dev/null || true
echo "$Z1_USER ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/"$Z1_USER"
chmod 0440 /etc/sudoers.d/"$Z1_USER"
`, name, uid, gid)

	cmd := exec.Command("docker", "exec", "-u", "root", config.ContainerName, "sh", "-c", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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
	anvil := config.AnvilDir()

	if err := os.MkdirAll(anvil, 0755); err != nil {
		ui.Warn("could not create anvil dir: " + err.Error())
	}

	display, xauthPath, useVNC := resolveX11()

	ui.StartDetail("image", config.ImageName)
	ui.StartDetail("anvil", anvil)
	ui.StartDetail("user", fmt.Sprintf("%s (%s:%s)", name, uid, gid))
	ui.StartDetail("network", cfg.Network.Mode)

	if useVNC {
		ui.StartDetail("display", "vnc fallback")
		ui.StartDetail("vnc", "connect a vnc client to localhost:5900")
	} else {
		ui.StartDetail("display", display)
		ui.StartDetail("xauth", xauthPath)
	}

	args := []string{
		"run", "-dit",
		"--name", config.ContainerName,
		"--hostname", "z1",
		"--add-host", "z1:127.0.0.1",
		"--user", "root",
		"--cap-add", "SYS_TIME",
		"--security-opt", "seccomp=unconfined",
	}

	args = append(args, networkArgs(cfg)...)

	if useVNC {
		args = append(args, "-p", "5900:5900", "-e", "VNC_MODE=1")
	} else {
		homeXauth := "/home/" + name + "/.Xauthority"
		args = append(args,
			"-e", "DISPLAY="+display,
			"-e", "XAUTHORITY="+homeXauth,
			"-v", "/tmp/.X11-unix:/tmp/.X11-unix:rw",
			"-v", xauthPath+":"+homeXauth+":rw",
		)
	}

	args = append(args,
		"-v", "/etc/hosts:/etc/hosts",
		"-v", anvil+":/anvil",
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

	if !useVNC {
		_ = exec.Command("xhost", "+local:docker").Run()
	}

	if err := runCmd("docker", args...); err != nil {
		ui.Error("failed to start container: " + err.Error())
		os.Exit(1)
	}

	if cfg.Network.Mode == "vpn" && cfg.Network.VPNConfig != "" {
		if err := connectVPN(cfg.Network.VPNConfig); err != nil {
			ui.Warn("vpn auto-connect failed: " + err.Error())
		}
	}

	if err := ensureContainerUser(name, uid, gid); err != nil {
		ui.Warn("could not create non-root user: " + err.Error())
	}

	ui.StartDone()
	attach(name)
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

func resolveX11() (string, string, bool) {
	display := os.Getenv("DISPLAY")
	if display == "" {
		ui.Warn("$DISPLAY not set - falling back to vnc")
		return "", "", true
	}

	xauth := os.Getenv("XAUTHORITY")
	if xauth == "" {
		home, _ := os.UserHomeDir()
		xauth = home + "/.Xauthority"
	}

	if _, err := os.Stat(xauth); os.IsNotExist(err) {
		ui.Warn("xauth file not found at " + xauth + " - generating a fresh one")
		home, _ := os.UserHomeDir()
		xauth = home + "/.Xauthority"
		_ = exec.Command("touch", xauth).Run()
		if err := exec.Command("xauth", "-f", xauth, "generate", display, ".", "trusted").Run(); err != nil {
			ui.Warn("xauth generate failed: " + err.Error())
			return "", "", true
		}
	}

	if err := exec.Command("xset", "-display", display, "q").Run(); err != nil {
		ui.Warn("display " + display + " not reachable - falling back to vnc")
		return "", "", true
	}

	return display, xauth, false
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
