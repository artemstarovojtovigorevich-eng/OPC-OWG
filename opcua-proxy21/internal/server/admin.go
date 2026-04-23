package server

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"

	"opcua-proxy21/internal/storage"
)

type AdminServer struct {
	addr string
	mux  *http.ServeMux
}

func NewAdminServer(addr string) *AdminServer {
	s := &AdminServer{
		addr: addr,
		mux:  http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *AdminServer) routes() {
	s.mux.HandleFunc("/", s.handleIndex)
	s.mux.HandleFunc("/api/settings", s.handleSettings)
	s.mux.HandleFunc("/api/nodes", s.handleNodes)
	s.mux.HandleFunc("/api/node/enable", s.handleNodeEnable)
	s.mux.HandleFunc("/api/node/delete", s.handleNodeDelete)
	s.mux.HandleFunc("/api/discover", s.handleDiscover)
	s.mux.HandleFunc("/api/state", s.handleState)
}

func (s *AdminServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *AdminServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	t, err := template.New("index").Parse(indexHTML)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	nodes, _ := storage.GetAllNodes()
	state, _ := storage.GetAppState("status")
	namespace, _ := storage.GetSetting("namespace")

	data := struct {
		Nodes     []storage.Node
		State     string
		Namespace string
	}{
		Nodes:     nodes,
		State:     state,
		Namespace: namespace,
	}

	w.Header().Set("Content-Type", "text/html")
	t.Execute(w, data)
}

func (s *AdminServer) handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		namespace, _ := storage.GetSetting("namespace")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"namespace": namespace,
		})
		return
	}

	if r.Method == http.MethodPost {
		namespace := r.FormValue("namespace")
		if namespace != "" {
			storage.SetSetting("namespace", namespace)
			storage.SetAppState("status", "configured")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}

	http.Error(w, "method not allowed", 405)
}

func (s *AdminServer) handleNodes(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		nodes, err := storage.GetAllNodes()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(nodes)
		return
	}

	http.Error(w, "method not allowed", 405)
}

func (s *AdminServer) handleNodeEnable(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		idStr := r.FormValue("id")
		enabledStr := r.FormValue("enabled")

		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid id", 400)
			return
		}

		enabled := enabledStr == "true"
		if err := storage.SetNodeEnabled(id, enabled); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}

	http.Error(w, "method not allowed", 405)
}

func (s *AdminServer) handleNodeDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		idStr := r.FormValue("id")

		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid id", 400)
			return
		}

		if err := storage.DeleteNode(id); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}

	http.Error(w, "method not allowed", 405)
}

func (s *AdminServer) handleDiscover(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		storage.SetAppState("status", "discovering")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}

	http.Error(w, "method not allowed", 405)
}

func (s *AdminServer) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		state, _ := storage.GetAppState("status")
		namespace, _ := storage.GetSetting("namespace")
		count, _ := storage.NodeCount()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    state,
			"namespace": namespace,
			"nodeCount": count,
		})
		return
	}

	http.Error(w, "method not allowed", 405)
}