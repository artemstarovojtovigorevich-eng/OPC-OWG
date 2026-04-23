package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"opcua-proxy21/internal/config"
	"opcua-proxy21/internal/logger"
	"opcua-proxy21/internal/opcua"
	"opcua-proxy21/internal/sender"
	"opcua-proxy21/internal/server"
	"opcua-proxy21/internal/storage"
	"opcua-proxy21/pkg/cert"
)

type App struct {
	cfg       *config.Config
	log       *logger.Logger
	opcClient *opcua.Client
	reader    *opcua.Reader
	sender    *sender.UDStreamSender
	certMgr   *cert.CertManager
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Config validation failed: %v\n", err)
		os.Exit(1)
	}

	log, err := logger.New(cfg.GetLogLevel(), cfg.GetLogEncoding())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	log.Info("Starting OPC UA Proxy",
		"endpoint", cfg.GetOPCEndpoint(),
		"udp", cfg.GetUDPDest(),
		"sourceID", cfg.GetSourceID(),
		"pollInterval", cfg.GetPollInterval(),
	)

	if err := storage.Init(); err != nil {
		log.Fatal("Failed to init storage", "error", err)
	}
	defer storage.Close()

	nodeCount, err := storage.NodeCount()
	if err != nil {
		log.Error("Failed to get node count", "error", err)
	}

	log.Info("Storage initialized", "nodeCount", nodeCount)

	app := NewApp(cfg, log)

	if nodeCount == 0 {
		log.Info("No nodes configured, starting admin UI")

		storage.SetAppState("status", "waiting_config")
		adminServer := server.NewAdminServer(":8080")

		go func() {
			if err := http.ListenAndServe(":8080", adminServer); err != nil {
				log.Error("Admin server error", "error", err)
			}
		}()

		go app.configWatcher(ctx)

		<-ctx.Done()
		log.Info("Shutting down...")
		return
	}

	if err := app.Start(ctx); err != nil {
		log.Fatal("Failed to start application", "error", err)
	}

	<-ctx.Done()
	log.Info("Shutting down...")

	if err := app.Shutdown(context.Background()); err != nil {
		log.Error("Error during shutdown", "error", err)
	}
}

func (app *App) configWatcher(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
			status, _ := storage.GetAppState("status")
			if status == "configured" || status == "discovering" {
				app.log.Info("Starting node discovery")
				if err := app.runDiscovery(ctx); err != nil {
					app.log.Error("Discovery failed", "error", err)
					storage.SetAppState("status", "error")
					continue
				}

				count, _ := storage.NodeCount()
				app.log.Info("Discovery complete", "nodes", count)

				if count > 0 {
					if err := app.Start(ctx); err != nil {
						app.log.Error("Failed to start application", "error", err)
						storage.SetAppState("status", "error")
						continue
					}

					storage.SetAppState("status", "running")

					<-ctx.Done()
					app.Shutdown(context.Background())
					return
				}
			}
		}
	}
}

func (app *App) runDiscovery(ctx context.Context) error {
	namespace, _ := storage.GetSetting("namespace")
	ns, err := strconv.Atoi(namespace)
	if err != nil {
		return fmt.Errorf("invalid namespace: %w", err)
	}

	certData, privKey, err := app.certMgr.LoadOrGenerate(app.cfg.GetGenCert())
	if err != nil {
		return err
	}

	discovery := opcua.NewDiscovery(app.cfg.GetOPCEndpoint(), app.log)
	endpoints, err := discovery.GetEndpoints(ctx)
	if err != nil {
		app.log.Warn("Discovery failed, trying direct connect", "error", err)
		app.opcClient = opcua.NewClientDirect(app.cfg.GetOPCEndpoint(), app.log)
	} else {
		endpoint := discovery.FindEndpoint(endpoints, app.cfg.GetSecurityPolicy(), app.cfg.GetSecurityMode())
		if endpoint != nil {
			app.opcClient = opcua.NewClient(app.cfg.GetOPCEndpoint(), endpoint, certData, privKey, app.log)
		} else {
			app.opcClient = opcua.NewClientDirect(app.cfg.GetOPCEndpoint(), app.log)
		}
	}

	if err := app.opcClient.Connect(ctx); err != nil {
		return err
	}

	app.reader = opcua.NewReader(app.opcClient, nil, app.log)

	app.log.Info("Browsing for nodes", "namespace", ns)
	nodes, err := app.reader.BrowseAllNodes(ctx, "ns=0;i=85", ns)
	if err != nil {
		return fmt.Errorf("browse failed: %w", err)
	}

	app.log.Info("Found nodes", "count", len(nodes))

	saveCount := len(nodes)
	if saveCount > 15 {
		saveCount = 15
		app.log.Info("Limiting to 15 nodes for UDP packet size")
	}

	for i := 0; i < saveCount; i++ {
		storage.SaveNode(nodes[i].ID, nodes[i].Name, nodes[i].DataType)
	}

	return nil
}

func NewApp(cfg *config.Config, log *logger.Logger) *App {
	certMgr := cert.NewCertManager(cfg.GetCertFile(), cfg.GetKeyFile(), "urn:client")
	return &App{
		cfg:     cfg,
		log:     log,
		certMgr: certMgr,
	}
}

func (app *App) Start(ctx context.Context) error {
	certData, privKey, err := app.certMgr.LoadOrGenerate(app.cfg.GetGenCert())
	if err != nil {
		return err
	}

	discovery := opcua.NewDiscovery(app.cfg.GetOPCEndpoint(), app.log)
	endpoints, err := discovery.GetEndpoints(ctx)
	if err != nil {
		app.log.Warn("Discovery failed, trying direct connect", "error", err)
		app.opcClient = opcua.NewClientDirect(app.cfg.GetOPCEndpoint(), app.log)
	} else {
		endpoint := discovery.FindEndpoint(endpoints, app.cfg.GetSecurityPolicy(), app.cfg.GetSecurityMode())
		if endpoint != nil {
			app.log.Info("Using endpoint",
				"url", endpoint.EndpointURL,
				"security", endpoint.SecurityPolicyURI,
				"mode", endpoint.SecurityMode,
			)
			app.opcClient = opcua.NewClient(app.cfg.GetOPCEndpoint(), endpoint, certData, privKey, app.log)
		} else if len(endpoints) > 0 {
			app.log.Warn("No matching endpoint, using first available")
			app.opcClient = opcua.NewClientDirect(app.cfg.GetOPCEndpoint(), app.log)
		} else {
			app.log.Warn("No endpoints found, trying direct connect")
			app.opcClient = opcua.NewClientDirect(app.cfg.GetOPCEndpoint(), app.log)
		}
	}

	if err := app.opcClient.Connect(ctx); err != nil {
		return err
	}

	app.reader = opcua.NewReader(app.opcClient, nil, app.log)

	if !app.cfg.GetReadOnly() {
		app.sender = sender.NewUDStreamSender(app.cfg.GetUDPDest(), app.cfg.GetSourceID())
		if err := app.sender.Connect(); err != nil {
			return err
		}
	}

	go app.pollLoop(ctx)
	return nil
}

func (app *App) pollLoop(ctx context.Context) {
	ticker := time.NewTicker(app.cfg.GetPollInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			nodes, err := storage.GetEnabledNodes()
			if err != nil {
				app.log.Error("Failed to get nodes from storage", "error", err)
				continue
			}

			if len(nodes) == 0 {
				continue
			}

			opcuaNodes := make([]opcua.Node, len(nodes))
			for i, n := range nodes {
				opcuaNodes[i] = opcua.Node{ID: n.NodeID, Name: n.Name}
			}

			data, err := app.reader.ReadMultiple(ctx, opcuaNodes)
			if err != nil {
				app.log.Error("Failed to read data", "error", err)
				continue
			}

			if len(data) > 0 {
				sendData := data
				if len(sendData) > 15 {
					sendData = sendData[:15]
				}

				nodeInfos := make([]opcua.NodeInfo, len(sendData))
				for i, d := range sendData {
					nodeInfos[i] = opcua.NodeInfo{
						ID:        d.NodeID,
						Value:     d.Value,
						Timestamp: d.Timestamp,
					}
				}

				if app.cfg.GetReadOnly() {
					app.log.Info("Read values", "count", len(sendData))
				} else {
					if err := app.sender.SendNodesWithMetadata(app.cfg.GetOPCEndpoint(), nodeInfos); err != nil {
						app.log.Error("Failed to send", "error", err)
					} else {
						app.log.Info("Sent", "count", len(sendData))
					}
				}
			}
		}
	}
}

func (app *App) Shutdown(ctx context.Context) error {
	app.log.Info("Closing OPC client...")
	if app.opcClient != nil {
		_ = app.opcClient.Disconnect(ctx)
	}

	if !app.cfg.GetReadOnly() && app.sender != nil {
		app.log.Info("Closing UDP sender...")
		_ = app.sender.Close()
	}

	app.log.Info("Shutdown complete")
	return nil
}