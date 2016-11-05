package apiserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/emicklei/go-restful"
	"github.com/ingvagabund/cluster-capacity/pkg/client/emulator"
)

var TIMELAYOUT = "2006-01-02T15:04:05Z07:00"

type RestResource struct {
	cache   *Cache
	watcher *WatchChannelDistributor
}

func (r *RestResource) Register(container *restful.Container) {
	ws := new(restful.WebService)
	ws.
		Path("/capacity").
		Doc("Manage cluster capacity checker").
		Consumes(restful.MIME_XML, restful.MIME_JSON).
		Produces(restful.MIME_JSON, restful.MIME_XML)

	ws.Route(ws.GET("/status/last").To(r.getLastStatus).
		Doc("Get most recent cluster capacity report").
		Operation("getStatus").
		Param(ws.QueryParameter("num", "number of last records to be listed").DataType("string")).
		Writes(emulator.Report{}))

	ws.Route(ws.GET("/status/watch").To(r.watchStatus).
		Doc("Watch for following statuses").
		Operation("watchStatus"))

	ws.Route(ws.GET("/status/list").To(r.listStatus).
		Doc("List all reports since and to specified date.").
		Operation("listRange").
		Param(ws.QueryParameter("since", "RFC3339 standard").DataType("string")).
		Param(ws.QueryParameter("to", "RFC3339 standard").DataType("string")).
		Writes([]emulator.Report{}))
	container.Add(ws)
}

func NewResource(c *Cache, watch chan *emulator.Report) *RestResource {
	return &RestResource{
		cache:   c,
		watcher: NewWatchChannelDistributor(watch),
	}
}

func ListenAndServe(r *RestResource) error {
	wsContainer := restful.NewContainer()
	r.Register(wsContainer)

	go r.watcher.Run()
	server := &http.Server{Addr: ":8081", Handler: wsContainer}
	return server.ListenAndServe()
}

func (r *RestResource) getLastStatus(request *restful.Request, response *restful.Response) {
	numStr := request.QueryParameter("num")
	if numStr == "" {
		numStr = "1"
	}
	num, err := strconv.Atoi(numStr)
	if err != nil {
		response.AddHeader("Content-Type", "text/plain")
		str := fmt.Sprintf("400: Failed to parse parameter \"num\": %v\n", err)
		response.WriteErrorString(http.StatusBadRequest, str)
		return
	}
	report := r.cache.GetLast(num)
	if report == nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusNotFound, "404: No reports found.")
		return
	}
	response.WriteAsJson(report)
}

// use this to avoid multiple response.WriteHeader calls
func writeJson(resp *restful.Response, r *emulator.Report) error {
	output, err := json.MarshalIndent(r, " ", " ")
	if err != nil {
		return err
	}
	_, err = resp.Write(output)
	return err
}

func (r *RestResource) watchStatus(request *restful.Request, response *restful.Response) {
	w := response.ResponseWriter

	//receive read channel
	ch := make(chan *emulator.Report)
	chpos, err := r.watcher.AddChannel(ch)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		errmsg := fmt.Sprintf("Can't start watching: %v", err)
		response.WriteErrorString(http.StatusForbidden, errmsg)
		return
	}
	defer r.watcher.RemoveChannel(chpos)

	//list all
	response.WriteAsJson(r.cache.All())
	w.Header().Set("Transfer-Encoding", "chunked")
	w.(http.Flusher).Flush()

	//listen read channel
	for {
		select {
		case <-w.(http.CloseNotifier).CloseNotify():
			return
		case report, ok := <-ch:
			if !ok {
				return
			}
			if err := writeJson(response, report); err != nil {
				continue

			}
			w.(http.Flusher).Flush()
		}
	}
}

func (r *RestResource) listStatus(request *restful.Request, response *restful.Response) {

	//if since is not defined set is as start of epoch
	sinceStr := request.QueryParameter("since")
	if sinceStr == "" {
		sinceStr = "1970-01-01T00:00:00+00:00"
	}
	since, err := time.Parse(TIMELAYOUT, sinceStr)
	if err != nil {
		str := fmt.Sprintf("400: Failed to parse parameter \"since\": %v\n", err)
		response.WriteErrorString(http.StatusBadRequest, str)
		return
	}

	//if to is not defined set it as now
	toStr := request.QueryParameter("to")
	var to time.Time
	if toStr == "" {
		to = time.Now()
	} else {
		to, err = time.Parse(TIMELAYOUT, toStr)
		if err != nil {
			str := fmt.Sprintf("400: Failed to parse parameter \"to\": %v\n", err)
			response.WriteErrorString(http.StatusBadRequest, str)
			return
		}
	}

	reports := r.cache.List(since, to)
	if reports == nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusNotFound, "404: No reports found.")
		return
	}
	response.WriteAsJson(reports)
}
