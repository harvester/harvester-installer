package console

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/jroimartin/gocui"
	"github.com/pkg/errors"
	"github.com/rancher/harvester-installer/pkg/console/log"
)

const (
	harvesterURL        = "harvesterURL"
	harvesterStatus     = "harvesterStatus"
	colorRed        int = 1
	colorGreen      int = 2
	colorYellow     int = 3
)

var installed = false

func layoutDashboard(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	if v, err := g.SetView("url", maxX/3, maxY/3, maxX/3*2, maxY/3+5); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		v.Wrap = true
		ip, err := getHarvesterIP()
		if err != nil {
			return fmt.Errorf("failed to get IP: %v", err)
		}
		fmt.Fprintf(v, "Harvester management URL:\n\nhttps://%s:8443", strings.TrimSpace(string(ip)))
	}
	if v, err := g.SetView("status", maxX/3, maxY/3+5, maxX/3*2, maxY/3+10); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		v.Wrap = true
		go syncHarvesterStatus(context.Background(), g)
	}
	if v, err := g.SetView("footer", 0, maxY-2, maxX, maxY); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		fmt.Fprintf(v, "<Use F12 to switch between Harvester console and Shell>")
	}

	return nil
}

func getHarvesterIP() (string, error) {
	// ip, err := exec.Command("/bin/sh", "-c", `sudo cat /etc/rancher/k3s/k3s-service.env|grep K3S_URL|grep -Eo "([0-9]{1,3}[\.]){3}[0-9]{1,3}"`).CombinedOutput()
	// if err != nil {
	// 	return "", errors.Wrap(err, string(ip))
	// }
	// if string(ip) != "" {
	// 	return string(ip), nil
	// }
	//it's the master,get the node ip
	ip, err := exec.Command("/bin/sh", "-c", `ifconfig eth0| grep -Eo 'inet (addr:)?([0-9]*\.){3}[0-9]*' | grep -Eo '([0-9]*\.){3}[0-9]*'`).CombinedOutput()
	return string(ip), err
}

func syncHarvesterStatus(ctx context.Context, g *gocui.Gui) {
	//sync status at the begining
	doSyncHarvesterStatus(g)

	syncDuration := 5 * time.Second
	ticker := time.NewTicker(syncDuration)
	go func() {
		<-ctx.Done()
		ticker.Stop()
	}()
	for range ticker.C {
		doSyncHarvesterStatus(g)
	}
}

func doSyncHarvesterStatus(g *gocui.Gui) {
	status := getHarvesterStatus()
	log.Debug(g, status)
	g.Update(func(g *gocui.Gui) error {
		v, err := g.View("status")
		if err != nil {
			return err
		}
		v.Clear()
		fmt.Fprintln(v, "Current status: "+status)
		return nil
	})
}

func k8sIsReady() bool {
	_, err := exec.Command("/bin/sh", "-c", `kubectl get cs`).CombinedOutput()
	if err != nil {
		return false
	}
	return true
}

func nodeIsReady() bool {
	output, err := exec.Command("/bin/sh", "-c", `kubectl get no -o jsonpath='{.items[*].metadata.name}'`).CombinedOutput()
	if err != nil {
		return false
	}
	if string(output) == "" {
		//no node is added
		return false
	}
	return true
}

func chartIsInstalled() bool {
	output, err := exec.Command("/bin/sh", "-c", `kubectl get po -n kube-system -l "job-name=helm-install-harvester" -o jsonpath='{.items[*].status.phase}'`).CombinedOutput()
	if err != nil {
		return false
	}
	if string(output) == "Succeeded" {
		return true
	}
	return false
}

func harvesterPodStatus() (string, error) {
	output, err := exec.Command("/bin/sh", "-c", `kubectl get po -n harvester-system -l "app.kubernetes.io/name=harvester" -o jsonpath='{.items[*].status.phase}'`).CombinedOutput()
	if err != nil {
		return "", errors.Wrap(err, string(output))
	}
	return string(output), nil
}

func getHarvesterStatus() string {
	if !installed {
		if !k8sIsReady() || !nodeIsReady() || !chartIsInstalled() {
			return "Harvester is installing"
		}
	}
	installed = true
	status, err := harvesterPodStatus()
	if err != nil {
		wrapColor(err.Error(), colorRed)
	}
	if status == "" {
		status = wrapColor("Unknown", colorYellow)
	} else if status == "Running" {
		status = wrapColor(status, colorGreen)
	} else {
		status = wrapColor(status, colorYellow)
	}
	return status
}

func wrapColor(s string, color int) string {
	return fmt.Sprintf("\033[3%d;7m%s\033[0m", color, s)
}
