package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/lmittmann/tint"
	"github.com/protegear/sbd"
	"github.com/protegear/sbd/mux"
	yaml "gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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
	log          *slog.Logger
)

func main() {
	config := flag.String("config", "", "specify the configuration for your forwarding rules")
	health := flag.String("health", "127.0.0.1:2023", "the healtcheck URL (http)")
	stage := flag.String("stage", "test", "the name of the stage where this service is running")
	loglevel := flag.String("loglevel", "info", "the loglevel, debug|info|warn|error|crit")
	logformat := flag.String("logformat", "json", "the logformat, fmt|json|term")
	workers := flag.Int("workers", 5, "the number of workers")
	useproxyprotocol := flag.Bool("proxyprotocol", false, "use the proxyprotocol on the listening socket")

	flag.Parse()

	setLogOutput(*logformat, *loglevel)

	log = slog.With("stage", *stage)

	listen := defaultListen
	if len(flag.Args()) > 0 {
		listen = flag.Arg(0)
	}

	log.Info("start service", "revision", revision, "builddate", builddate, "listen", listen)
	distribution = mux.New(*workers, log)
	if *config != "" {
		cfg, err := os.Open(*config)
		if err != nil {
			log.Error("cannot open config file", "config", *config, "error", err)
			os.Exit(1)
		}

		var targets mux.Targets
		err = yaml.NewDecoder(cfg).Decode(&targets)
		if err != nil {
			log.Error("cannot unmarshal data", "error", err)
			os.Exit(1)
		}
		err = distribution.WithTargets(targets)
		if err != nil {
			log.Error("cannot use config", "error", err)
			os.Exit(1)
		}
		log.Info("change configuration", "targets", targets)
	}

	ctx := context.Background()
	client, err := rest.InClusterConfig()
	if err != nil {
		log.Info("no incluster config, assume standalone mode")
	} else {
		log.Info("incluster config found, assume kubernetes mode")
		go watchServices(ctx, log, client, distribution)
	}

	go runHealth(*health)
	sbd.NewService(log, listen, sbd.Logger(log, distribution), *useproxyprotocol)
}

func runHealth(health string) {
	http.ListenAndServe(health, http.HandlerFunc(func(rw http.ResponseWriter, rq *http.Request) {
		fmt.Fprintf(rw, "OK")
	}))
}

func watchServices(ctx context.Context, log *slog.Logger, client *rest.Config, s mux.Distributer) {
	clientset, err := kubernetes.NewForConfig(client)
	if err != nil {
		log.Error("cannot create clientset for services", "error", err)
		os.Exit(1)
	}

	watcher, err := clientset.CoreV1().Services(v1.NamespaceAll).
		Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Error("cannot create watcher for services", "error", err)
		os.Exit(1)
	}

	for event := range watcher.ResultChan() {
		svc := event.Object.(*v1.Service)
		targets := s.Targets()
		if event.Type == watch.Added {
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
		} else if event.Type == watch.Deleted {
			var tgs []mux.Target
			for _, t := range targets {
				if t.ID == string(svc.ObjectMeta.UID) {
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

		} else if event.Type == watch.Modified {
			t := targetFromService(svc)
			if t != nil {
				var tgs []mux.Target
				for _, tt := range targets {
					if tt.ID == string(svc.ObjectMeta.UID) {
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

func targetFromService(s *v1.Service) *mux.Target {
	mt := s.ObjectMeta
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
		ip := s.Spec.ClusterIP
		if ip != "" {
			slog.Info("found target", "imei", t, "path", path, "port", port, "ip", ip)
			bk := mux.Target{
				ID:          string(s.ObjectMeta.UID),
				Backend:     fmt.Sprintf("http://%s:%s%s", ip, port, path),
				IMEIPattern: t,
			}
			return &bk
		}
	}
	return nil
}

func setLogOutput(format, loglevel string) {
	lvl := slog.LevelDebug
	switch strings.ToLower(loglevel) {
	case "debug":
		lvl = slog.LevelDebug
	case "info":
		lvl = slog.LevelInfo
	case "warn":
		lvl = slog.LevelWarn
	case "error", "crit":
		lvl = slog.LevelError
	}

	if format == "logfmt" || format == "term" {
		w := os.Stdout
		slog.SetDefault(slog.New(
			tint.NewHandler(w, &tint.Options{
				AddSource:  true,
				Level:      lvl,
				TimeFormat: time.DateTime,
			}),
		))
	} else {
		w := os.Stdout
		slog.SetDefault(slog.New(
			slog.NewJSONHandler(w, &slog.HandlerOptions{
				AddSource: true,
				Level:     lvl,
			}),
		))
	}
}
