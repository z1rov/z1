package main

import (
	"os"

	"github.com/z1rov/z1/internal/config"
	"github.com/z1rov/z1/internal/docker"
	"github.com/z1rov/z1/internal/storage"
	"github.com/z1rov/z1/internal/timesync"
	"github.com/z1rov/z1/internal/ui"
	"github.com/z1rov/z1/internal/updater"
)

func printUsage() {
	ui.Usage(docker.ImageExists(), docker.IsRunning())
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	switch os.Args[1] {

	case "start":
		usb := ""
		for i := 2; i < len(os.Args); i++ {
			if os.Args[i] == "--usb" && i+1 < len(os.Args) {
				usb = os.Args[i+1]
				i++
			}
		}
		docker.Start(usb)

	case "stop":
		docker.Stop()

	case "status":
		docker.Status()

	case "logs":
		follow := len(os.Args) > 2 && os.Args[2] == "-f"
		docker.Logs(follow)

	case "exec":
		if len(os.Args) < 3 {
			ui.Error("usage: z1 exec <command> [args...]")
			os.Exit(1)
		}
		docker.Exec(os.Args[2:])

	case "install":
		updater.Install()

	case "update":
		updater.Update()

	case "delete":
		docker.FullCleanup()

	case "version":
		updater.Version()

	case "relocate":
		if err := storage.Relocate(); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}

	case "synctime":
		if len(os.Args) < 3 {
			ui.Error("usage: z1 synctime <dc-ip> | z1 synctime restore")
			os.Exit(1)
		}
		if os.Args[2] == "restore" {
			timesync.Restore()
		} else {
			timesync.Sync(os.Args[2])
		}

	case "config":
		config.ShowEffective()

	case "help", "--help", "-h":
		printUsage()

	default:
		ui.Error("unknown command: " + os.Args[1])
		ui.Blank()
		printUsage()
		os.Exit(1)
	}
}
