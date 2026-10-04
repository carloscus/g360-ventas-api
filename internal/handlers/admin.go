// Package handlers: admin endpoints para operacion del API.
//
// Incluye el endpoint POST /api/admin/refresh que permite a los clientes
// solicitar un refresh del snapshot de forma on-demand. El servidor procesa
// la solicitud de forma atomica (solo un refresh a la vez) y reporta el estado.
//
// El refresh se hace leyendo la DB de Tauri (NTFS, vía /mnt/c) con SQLite
// backup API embebida en Go, evitando depender de Python o wsl.exe.

package handlers

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// TriggerFile es el nombre del archivo trigger para refresh on-demand.
const TriggerFile = "refresh_trigger"

// RefreshState representa el estado actual del refresh.
type RefreshState struct {
	mu          sync.RWMutex
	Status      string  `json:"status"` // "idle" | "running" | "ok" | "error"
	Message     string  `json:"message"`
	StartedAt   string  `json:"started_at,omitempty"`
	FinishedAt  string  `json:"finished_at,omitempty"`
	DurationSec float64 `json:"duration_sec,omitempty"`
	Error       string  `json:"error,omitempty"`
}

// HandleRefreshRequest crea el archivo trigger para solicitar un refresh.
func (s *Server) HandleRefreshRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Solo un refresh a la vez
	s.refreshState.mu.RLock()
	if s.refreshState.Status == "running" {
		s.refreshState.mu.RUnlock()
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "busy",
			"message": "Ya hay un refresh en curso. Espere o consulte /api/admin/refresh-status",
		})
		return
	}
	s.refreshState.mu.RUnlock()

	triggerPath := filepath.Join(s.Cfg.ExportDir, TriggerFile)
	dir := filepath.Dir(triggerPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo crear dir: "+err.Error())
		return
	}

	timestamp := time.Now().UTC().Format(time.RFC3339)
	if err := os.WriteFile(triggerPath, []byte(timestamp), 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo escribir trigger: "+err.Error())
		return
	}

	log.Printf("admin: refresh trigger creado a las %s", timestamp)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "triggered",
		"timestamp": timestamp,
		"message":   "Refresh solicitado. Use GET /api/admin/refresh-status para ver el estado.",
	})
}

// HandleRefreshStatus retorna el estado actual del refresh.
func (s *Server) HandleRefreshStatus(w http.ResponseWriter, r *http.Request) {
	s.refreshState.mu.RLock()
	defer s.refreshState.mu.RUnlock()
	writeJSON(w, http.StatusOK, s.refreshState)
}

// RunRefreshWorkflow observa el trigger file y ejecuta el refresh cuando se detecta.
// Debe correr en un goroutine de background.
func (s *Server) RunRefreshWorkflow() {
	triggerPath := filepath.Join(s.Cfg.ExportDir, TriggerFile)
	log.Printf("admin: watcher de refresh iniciado (trigger: %s)", triggerPath)

	for {
		time.Sleep(5 * time.Second)

		data, err := os.ReadFile(triggerPath)
		if err != nil {
			continue // No hay trigger todavía
		}

		timestamp := string(data)
		if len(timestamp) < 10 {
			os.Remove(triggerPath)
			continue
		}

		// Procesar el refresh
		s.refreshState.mu.Lock()
		s.refreshState.Status = "running"
		s.refreshState.Message = "Iniciando refresh del snapshot..."
		s.refreshState.StartedAt = time.Now().UTC().Format(time.RFC3339)
		s.refreshState.Error = ""
		t0 := time.Now()
		s.refreshState.mu.Unlock()

		log.Printf("admin: ejecutando refresh del snapshot (trigger: %s)", timestamp)

		err = s.doRefresh()
		if err != nil {
			s.refreshState.mu.Lock()
			s.refreshState.Status = "error"
			s.refreshState.Message = "Refresh falló"
			s.refreshState.Error = err.Error()
			s.refreshState.FinishedAt = time.Now().UTC().Format(time.RFC3339)
			s.refreshState.DurationSec = time.Since(t0).Seconds()
			s.refreshState.mu.Unlock()
			log.Printf("admin: refresh fallo: %v", err)
		} else {
			s.refreshState.mu.Lock()
			s.refreshState.Status = "ok"
			s.refreshState.Message = "Snapshot actualizado exitosamente"
			s.refreshState.FinishedAt = time.Now().UTC().Format(time.RFC3339)
			s.refreshState.DurationSec = time.Since(t0).Seconds()
			s.refreshState.mu.Unlock()
			log.Printf("admin: refresh completado en %.1fs", s.refreshState.DurationSec)
		}

		os.Remove(triggerPath)
	}
}

// doRefresh delega en el script Python existente (backup_snapshot.py) que ya
// tiene pruebas y logica probada de backup consistente con SQLite.
func (s *Server) doRefresh() error {
	pyPath := os.Getenv("G360_PYTHON")
	if pyPath == "" {
		pyPath = "python3"
	}

	ntfsDB := os.Getenv("G360_NTFS_DB_PATH")
	if ntfsDB == "" {
		ntfsDB = "/mnt/c/Users/ccusi/AppData/Roaming/g360-db-ventas/data/historial.db"
	}

	stageDir := os.Getenv("G360_STAGE_DIR")
	if stageDir == "" {
		stageDir = "/tmp/g360_snapshot"
	}
	if err := os.MkdirAll(stageDir, 0755); err != nil {
		return fmt.Errorf("crear stage dir: %w", err)
	}
	stageDB := filepath.Join(stageDir, "historial.db")

	// Ruta del script de backup: override por env, luego default absoluto
	// (el repo vive en /mnt/c, no es derivable desde DBPath que esta en ~).
	backupScript := os.Getenv("G360_BACKUP_SCRIPT")
	if backupScript == "" {
		backupScript = "/mnt/c/Users/ccusi/Documents/Proyect_Coder/G360-ecosystem/projects/g360-ventas-api/deploy/backup_snapshot.py"
	}
	if _, err := os.Stat(backupScript); err != nil {
		return fmt.Errorf("no se encontro backup_snapshot.py en %s: %w", backupScript, err)
	}

	log.Printf("admin: backup %s -> %s", backupScript, stageDB)
	cmd := exec.Command(pyPath, backupScript, "--source", ntfsDB, "--dest", stageDB)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("backup fallo: %w: %s", err, string(out))
	}

	// Copy stage -> tmp por streaming (el snapshot pesa ~2.6 GB, no cabe
	// entero en RAM con ReadFile), luego rename atomico al snapshot final.
	wslDB := s.Cfg.DBPath
	tmpDB := stageDB + ".tmp"
	if err := streamCopy(stageDB, tmpDB); err != nil {
		return fmt.Errorf("copiar stage->tmp: %w", err)
	}
	if err := os.Rename(tmpDB, wslDB); err != nil {
		return fmt.Errorf("rename tmp->wsl: %w", err)
	}

	// Reabrir el pool para que las queries vean el inode nuevo (si no,
	// las conexiones viejas seguirian sirviendo el snapshot anterior) y
	// limpiar caches para no servir agregados viejos.
	if err := s.Store.Reopen(); err != nil {
		s.statusCache.clear()
		s.checksumCache.clear()
		s.statsCache.clear()
		return fmt.Errorf("snapshot reemplazado pero Reopen fallo (reiniciar API): %w", err)
	}
	s.statusCache.clear()
	s.checksumCache.clear()
	s.statsCache.clear()

	log.Printf("admin: snapshot actualizado (%s)", wslDB)
	return nil
}

// streamCopy copia un archivo grande sin cargarlo entero en memoria.
func streamCopy(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
