package gocommon

import (
	"context"
	"os"

	"github.com/fsnotify/fsnotify"
	"github.com/godbus/dbus/v5"
	"github.com/sirupsen/logrus"
)

func WatchDBusSignal(ctx context.Context, iface, objPath string, handlerFunc func(s *dbus.Signal)) {
	conn, err := generateDBUSConnection()
	if err != nil {
		panic(err)
	}

	matchInterFace := dbus.WithMatchInterface(iface)
	matchObjPath := dbus.WithMatchObjectPath(dbus.ObjectPath(objPath))
	err = conn.AddMatchSignalContext(ctx, matchObjPath, matchInterFace)
	if err != nil {
		logrus.Errorf("Add match signal failed. err: %v", err)
		panic(err)
	}

	signals := make(chan *dbus.Signal, 2)
	conn.Signal(signals)

	logrus.Infof("Watch DBus signal with interface: %s, object path: %s", iface, objPath)
	for {
		select {
		case signalContent := <-signals:
			logrus.Debugf("Got signal: %+v", signalContent)
			handlerFunc(signalContent)
		case <-ctx.Done():
			return
		}
	}
}

func generateDBUSConnection() (*dbus.Conn, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		logrus.Warnf("Init DBus connection failed. err: %v", err)
		return nil, err
	}

	return conn, nil
}

func WatchFileChange(ctx context.Context, handlerFunc func(eventType string), monitorTargets []string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logrus.Errorf("failed to creating a fsnotify watcher: %v", err)
		panic(err)
	}
	defer watcher.Close()

	for _, target := range monitorTargets {
		_, err = os.Stat(target)
		if err != nil {
			logrus.Errorf("failed to stat file/directory %s: %v", target, err)
			continue
		}
		err := watcher.Add(target)
		if err != nil {
			logrus.Errorf("failed to add file/directory %s to watcher: %v", target, err)
			continue
		}
	}

	for {
		select {
		case event := <-watcher.Events:
			logrus.Debugf("event: %+v", event)
			if event.Op == fsnotify.Write {
				handlerFunc(event.Name)
			}
		case <-ctx.Done():
			return
		}
	}
}
