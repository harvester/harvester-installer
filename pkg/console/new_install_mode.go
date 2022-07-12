package console

import (
	"context"
	"fmt"
	"github.com/harvester/harvester-installer/pkg/config"
	"github.com/jroimartin/gocui"
	yipSchema "github.com/mudler/yip/pkg/schema"
	"os"
	"os/exec"
	"sync"
)

func configureInstalledNode(g *gocui.Gui, hvstConfig *config.HarvesterConfig, webhooks RendererWebhooks) error {
	// copy cosConfigFile
	// copy hvstConfigFile and break execution here
	ctx := context.TODO()
	webhooks.Handle(EventInstallStarted)

	// skip rancherd and network config in the cos config
	cosConfig, cosConfigFile, hvstConfigFile, err := generateTempConfigFiles(hvstConfig)
	if err != nil {
		printToPanel(g, err.Error(), installPanel)
		return err
	}

	defer os.Remove(cosConfigFile)
	defer os.Remove(hvstConfigFile)

	err = applyRancherdConfig(ctx, g, hvstConfig, cosConfig)
	if err != nil {
		printToPanel(g, fmt.Sprintf("error applying rancherd config :%v", err), installPanel)
		return err
	}

	err = restartCoreServices()
	if err != nil {
		printToPanel(g, fmt.Sprintf("error restarting core services: %v", err), installPanel)
	}
	return err
}

func apply(ctx context.Context, g *gocui.Gui, configFile string, stage string) error {
	cmd := exec.CommandContext(ctx, "/usr/bin/yip", "-s", stage, configFile)
	cmd.Env = os.Environ()
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	defer stderr.Close()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	defer stdout.Close()

	var wg sync.WaitGroup
	var writeLock sync.Mutex

	wg.Add(2)
	go func() {
		defer wg.Done()
		printToPanelAndLog(g, installPanel, "[stderr]", stderr, &writeLock)
	}()

	go func() {
		defer wg.Done()
		printToPanelAndLog(g, installPanel, "[stdout]", stdout, &writeLock)
	}()

	if err := cmd.Start(); err != nil {
		return err
	}

	wg.Wait()
	return cmd.Wait()
}

func applyDummyRoute() error {
	cmd := exec.Command("/usr/sbin/harv-dummy-iface")
	_, err := cmd.Output()
	return err
}

func restartCoreServices() error {
	cmd := exec.Command("/usr/sbin/harv-restart-services")
	_, err := cmd.Output()
	return err
}

func applyRancherdConfig(ctx context.Context, g *gocui.Gui, hvstConfig *config.HarvesterConfig, cosConfig *yipSchema.YipConfig) error {

	conf, err := config.GenerateRancherdConfig(hvstConfig)
	if err != nil {
		return err
	}

	for _, v := range conf.Stages["live"] {
		cosConfig.Stages["initramfs"] = append(cosConfig.Stages["initramfs"], v)
	}

	// additional config to copy files over to persist the new changes
	cosConfigFile, err := saveTemp(cosConfig, "cos")
	if err != nil {
		return err
	}

	hvstConfigFile, err := saveTemp(hvstConfig, "hvst")
	if err != nil {
		return err
	}

	copyFiles := yipSchema.Stage{
		Name: "copy files",
		Commands: []string{
			fmt.Sprintf("cp %s /oem/99_custom.yaml", cosConfigFile),
			fmt.Sprintf("cp %s /oem/harvester.config", hvstConfigFile),
		},
	}

	conf.Stages["finalise"] = append(conf.Stages["finalise"], copyFiles)

	liveCosConfig, err := saveTemp(conf, "live")
	if err != nil {
		return err
	}
	//defer os.Remove(liveCosConfig)

	// apply live stage to configure node
	err = apply(ctx, g, liveCosConfig, "live")
	if err != nil {
		return err
	}

	// apply finalise stage to copy contents
	// this will persist content across reboots
	return apply(ctx, g, liveCosConfig, "finalise")
}
