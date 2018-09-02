package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/ericchiang/k8s"
	corev1 "github.com/ericchiang/k8s/apis/core/v1"
	"github.com/inconshreveable/log15"
	"gitlab.com/globalsafetrack/sbd"
	"gitlab.com/globalsafetrack/sbd/mux"
	yaml "gopkg.in/yaml.v2"
)

const (
	defaultListen = "127.0.0.1:2022"
	logJSON       = "json"
	logFMT        = "fmt"
	logTERM       = "term"
)

var (
	revision     string
	builddate    string
	distribution mux.Distributer
	log          log15.Logger
)

func main() {
	config := flag.String("config", "", "specify the configuration for your forwarding rules")
	health := flag.String("health", "127.0.0.1:2023", "the healtcheck URL (http)")
	stage := flag.String("stage", "test", "the name of the stage where this service is running")
	loglevel := flag.String("loglevel", "info", "the loglevel, debug|info|warn|error|crit")
	logformat := flag.String("logformat", "json", "the logformat, fmt|json|term")
	workers := flag.Int("workers", 5, "the number of workers")

	flag.Parse()

	setLogOutput(*logformat, *loglevel)

	log = log15.New("stage", *stage)

	listen := defaultListen
	if len(flag.Args()) > 0 {
		listen = flag.Arg(0)
	}

	log.Info("start service", "revision", revision, "builddate", builddate, "listen", listen)
	distribution = mux.New(*workers, log)
	if *config != "" {
		cfg, err := os.Open(*config)
		if err != nil {
			log.Crit("cannot open config file", "config", *config, "error", err)
			os.Exit(1)
		}

		var targets mux.Targets
		err = yaml.NewDecoder(cfg).Decode(&targets)
		if err != nil {
			log.Crit("cannot unmarshal data", "error", err)
			os.Exit(1)
		}
		err = distribution.WithTargets(targets)
		if err != nil {
			log.Crit("cannot use config", "error", err)
			os.Exit(1)
		}
		log.Info("change configuration", "targets", targets)
	}

	ctx := context.Background()
	client, err := k8s.NewInClusterClient()
	if err != nil {
		log.Info("no incluster config, assume standalone mode")
	} else {
		log.Info("incluster config found, assume kubernetes mode")
		go watchServices(log, ctx, client, distribution)
	}

	go runHealth(*health)
	sbd.NewService(log, listen, sbd.Logger(log, distribution))
}

func runHealth(health string) {
	http.ListenAndServe(health, http.HandlerFunc(func(rw http.ResponseWriter, rq *http.Request) {
		fmt.Fprintf(rw, "OK")
	}))
}

func watchServices(log log15.Logger, ctx context.Context, client *k8s.Client, s mux.Distributer) {
	watcher, err := client.Watch(ctx, "", &corev1.Service{})
	if err != nil {
		log.Error("cannot create watcher for services", "error", err)
		os.Exit(1)
	}
	defer watcher.Close()
	for {
		svc := new(corev1.Service)
		et, err := watcher.Next(svc)
		if err != nil {
			log.Error("watcher returned error, exiting", "error", err)
			os.Exit(1)
		}
		targets := s.Targets()
		if et == k8s.EventAdded {
			t := targetFromService(svc)
			if t != nil {
				targets = append(targets, *t)
				err = s.WithTargets(targets)
				if err != nil {
					log.Error("cannot change targets", "error", err)
				} else {
					log.Info("added new target", "targets", targets)
				}
			}
		} else if et == k8s.EventDeleted {
			var tgs []mux.Target
			for _, t := range targets {
				if t.ID == *svc.GetMetadata().Uid {
					continue
				}
				tgs = append(tgs, t)
			}
			err = s.WithTargets(tgs)
			if err != nil {
				log.Error("cannot change targets", "error", err)
			} else {
				log.Info("deleted target", "targets", targets)
			}
		} else if et == k8s.EventModified {
			t := targetFromService(svc)
			if t != nil {
				var tgs []mux.Target
				for _, tt := range targets {
					if tt.ID == *svc.GetMetadata().Uid {
						tgs = append(tgs, *t)
					} else {
						tgs = append(tgs, tt)
					}
				}
				err = s.WithTargets(tgs)
				if err != nil {
					log.Error("cannot change targets", "error", err)
				} else {
					log.Info("modifiedtarget", "targets", targets)
				}

			}
		}
	}
}

func targetFromService(s *corev1.Service) *mux.Target {
	mt := s.GetMetadata()
	if mt != nil {
		a := mt.Annotations
		if t, ok := a["protegear.io/directip-imei"]; ok {
			path := a["protegear.io/directip-path"]
			if path == "" {
				path = "/"
			}
			port := a["protegear.io/directip-port"]
			if port == "" {
				port = "8080"
			}
			ip := s.GetSpec().ClusterIP
			if ip != nil {
				log15.Info("found target", "imei", t, "path", path, "port", port, "ip", *ip)
				bk := mux.Target{
					ID:          *s.GetMetadata().Uid,
					Backend:     fmt.Sprintf("http://%s:%s%s", *ip, port, path),
					IMEIPattern: t,
				}
				return &bk
			}
		}
	}
	return nil
}

func setLogOutput(format, loglevel string) {
	h := log15.CallerFileHandler(log15.StreamHandler(os.Stdout, log15.JsonFormat()))
	switch format {
	case logFMT:
		h = log15.CallerFileHandler(log15.StreamHandler(os.Stdout, log15.LogfmtFormat()))
	case logTERM:
		h = log15.CallerFileHandler(log15.StreamHandler(os.Stdout, log15.TerminalFormat()))
	}
	lvl, e := log15.LvlFromString(loglevel)
	if e != nil {
		lvl = log15.LvlInfo
		log15.Error("cannot parse level from parameter", "level", loglevel, "error", e)
	}
	target := log15.LvlFilterHandler(lvl, h)
	log15.Root().SetHandler(target)
}
