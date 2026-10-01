package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type exportEntry struct {
	Nombre        string          `json:"nombre"`
	TamanoBytes   int64           `json:"tamano_bytes"`
	ModificadoEn  string          `json:"modificado_en"`
	Manifest      json.RawMessage `json:"manifest,omitempty"`
	ManifestError string          `json:"manifest_error,omitempty"`
}

func (s *Server) listExports() ([]exportEntry, error) {
	entries, err := os.ReadDir(s.Cfg.ExportDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []exportEntry{}, nil
		}
		return nil, err
	}

	var out []exportEntry
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "base_canonica_") || !strings.HasSuffix(e.Name(), ".db") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		item := exportEntry{
			Nombre:       e.Name(),
			TamanoBytes:  info.Size(),
			ModificadoEn: info.ModTime().UTC().Format(time.RFC3339),
		}
		manifestPath := filepath.Join(s.Cfg.ExportDir, e.Name()+".manifest.json")
		if raw, err := os.ReadFile(manifestPath); err == nil {
			var probe any
			if json.Unmarshal(raw, &probe) == nil {
				item.Manifest = json.RawMessage(raw)
			} else {
				item.ManifestError = "manifest invalido"
			}
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Nombre > out[j].Nombre })
	return out, nil
}

func (s *Server) handleExportList(w http.ResponseWriter, r *http.Request) {
	list, err := s.listExports()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"directorio": s.Cfg.ExportDir,
		"total":      len(list),
		"snapshots":  list,
	})
}

func (s *Server) handleExportDownload(w http.ResponseWriter, r *http.Request) {
	list, err := s.listExports()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	if len(list) == 0 {
		writeError(w, http.StatusNotFound, "no hay snapshots en "+s.Cfg.ExportDir)
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		name = list[0].Nombre
	}
	var found *exportEntry
	for i := range list {
		if list[i].Nombre == name {
			found = &list[i]
			break
		}
	}
	if found == nil {
		writeError(w, http.StatusNotFound, "snapshot no encontrado: "+name)
		return
	}

	if r.URL.Query().Get("manifest") == "1" {
		if found.Manifest == nil {
			writeError(w, http.StatusNotFound, "sin manifiesto para "+name)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(found.Manifest)
		return
	}

	path := filepath.Join(s.Cfg.ExportDir, found.Nombre)
	if _, err := os.Stat(path); err != nil {
		writeError(w, http.StatusNotFound, "archivo no disponible")
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="`+found.Nombre+`"`)
	http.ServeFile(w, r, path)
}
