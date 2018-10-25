// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// Source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package diagnosis

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"strings"
	"time"

	"aahframe.work/config"
	"aahframe.work/log"
)

// New method creates new Diagnosis instance to collection various
// insights of application. Such as heap, profile, block, trace
// and mutex.
//
// Basically all capabilities supported by `runtime/pprof` and `runtime/trace`
// brought into HTTP or File mode collection.
func New(appName string, diagnosisCfg *config.Config, al log.Loggerer) (*Diagnosis, error) {
	if !diagnosisCfg.BoolDefault("runtime.diagnosis.enable", false) {
		return nil, nil
	}
	mode := diagnosisCfg.StringDefault("runtime.diagnosis.mode", "")
	if len(strings.TrimSpace(mode)) == 0 {
		return nil, errors.New("diagnosis: missing required config 'runtime.diagnosis.mode'")
	}
	d := &Diagnosis{appName: appName, cfg: diagnosisCfg, mode: strings.ToLower(mode)}
	switch d.mode {
	case "http":
		d.createHTTPServer()
	case "file":
		d.createFiles()
	}
	return d, nil
}

// Diagnosis brings feature of aah application profiling for various diagnosis.
// It support HTTP and File (upcoming) modes.
//
// Documentation and sample config refer to https://docs.aahframework.org/diagnosis.html
type Diagnosis struct {
	appName            string
	cfg                *config.Config
	log                log.Loggerer
	mode               string
	pathPrefix         string
	server             *http.Server
	serverWriteTimeout time.Duration
}

// Run method runs diagnosis solutions on current aah application based on
// given diagnosis configuration on application startup.
func (d *Diagnosis) Run() {
	if d.mode == "http" {
		d.log.Info("Diagnosis server starting at %s", d.server.Addr)
		if err := d.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			d.log.Error(err)
		}
		return
	}
	// File Mode upcoming :)

}

// Stop method to stop the diagnosis profiles, server and close file descriptors.
func (d *Diagnosis) Stop() {
	if d.server != nil {
		_ = d.server.Close()
		d.log.Info("Diagnosis server stopped successfully")
	}
	// stop the profilers for file mode and close the file descriptors
}

func (d *Diagnosis) createHTTPServer() {
	d.pathPrefix = "/diagnosis"
	mux := http.NewServeMux()
	mux.HandleFunc(d.pathPrefix, d.indexHandler)
	mux.HandleFunc(d.pathPrefix+"/", d.indexHandler)
	mux.HandleFunc(d.pathPrefix+"/pprof/", d.dynamicProfileHandler)
	mux.HandleFunc(d.pathPrefix+"/pprof/cmdline", d.cmdlineHandler)
	mux.HandleFunc(d.pathPrefix+"/pprof/profile", d.profileHandler)
	mux.HandleFunc(d.pathPrefix+"/pprof/symbol", d.symbolHandler)
	mux.HandleFunc(d.pathPrefix+"/pprof/trace", d.traceHandler)
	var err error
	d.serverWriteTimeout, err = time.ParseDuration(d.cfg.StringDefault("runtime.diagnosis.http.timeout.write", "2m"))
	if err != nil {
		d.serverWriteTimeout = time.Minute * 2
	}
	d.server = &http.Server{
		Addr:         d.cfg.StringDefault("runtime.diagnosis.http.address", ":7070"),
		Handler:      mux,
		WriteTimeout: d.serverWriteTimeout,
	}
}

func (d *Diagnosis) createFiles() {
	// Not yet implemented, upcoming feature though :)
}

func (d *Diagnosis) cpuProfile(w io.Writer, seconds time.Duration) error {
	if err := pprof.StartCPUProfile(w); err != nil {
		return fmt.Errorf("diagnosis: could not enable CPU profiling: %s", err)
	}
	if d.mode == "http" {
		d.sleep(w, seconds)
		pprof.StopCPUProfile()
	}
	return nil
}

func (d *Diagnosis) trace(w io.Writer, seconds time.Duration) error {
	if err := trace.Start(w); err != nil {
		return fmt.Errorf("diagnosis: could not enable tracing: %s", err)
	}
	if d.mode == "http" {
		d.sleep(w, seconds)
		trace.Stop()
	}
	return nil
}

func (d *Diagnosis) doProfileByName(w io.Writer, name string, gc bool, debug int) error {
	p := pprof.Lookup(name)
	if p == nil {
		return errors.New("diagnosis: unknown profile")
	}
	if name == "heap" && gc {
		runtime.GC()
	}
	p.WriteTo(w, debug)
	return nil
}

func (d *Diagnosis) sleep(w io.Writer, dur time.Duration) {
	var clientGone <-chan bool
	if cn, ok := w.(http.CloseNotifier); ok {
		clientGone = cn.CloseNotify()
	}
	select {
	case <-time.After(dur):
	case <-clientGone:
	}
}
