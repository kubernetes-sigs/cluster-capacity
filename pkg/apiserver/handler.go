package apiserver

import (
	"fmt"
	"github.com/emicklei/go-restful"
	"net/http"
	"time"
	"encoding/json"
)

var TIMELAYOUT = "2006-01-02T15:04:05Z07:00"

type RestResource struct {
	cache *Cache
	// watchChannelInput continuously receives new reports. If watching=true, watch
	// method forwards new reports to internalWatchChannel
	watchChannelInput chan *Report
	internalWatchChannel chan *Report
	watching bool

}

func (r *RestResource) Register(container *restful.Container) {
	ws := new(restful.WebService)
	ws.
		Path("/capacity").
		Doc("Manage cluster capacity checker").
		Consumes(restful.MIME_XML, restful.MIME_JSON).
		Produces(restful.MIME_JSON, restful.MIME_XML)

	ws.Route(ws.GET("/status/last").To(r.getStatus).
		Doc("Get most recent cluster capacity report").
		Operation("getStatus").
		Writes(Report{}))

	ws.Route(ws.GET("/status/watch").To(r.watchStatus).
		Doc("Watch for following statuses").
		Operation("watchStatus"))

	ws.Route(ws.GET("/status/list").To(r.listStatus).
		Doc("List all reports since and to specified date.").
		Operation("listRange").
		Param(ws.QueryParameter("since", "RFC3339 standard").DataType("string")).
		Param(ws.QueryParameter("to", "RFC3339 standard").DataType("string")).
		Writes([]Report{}))
	container.Add(ws)
}

func NewResource(c *Cache, watch chan *Report) *RestResource {
	return &RestResource{
		cache: c,
		watchChannelInput: watch,
		internalWatchChannel: make(chan *Report),
		watching: false,
	}
}

func ListenAndServe(r *RestResource) error {
	wsContainer := restful.NewContainer()
	r.Register(wsContainer)

	go r.watch()
	server := &http.Server{Addr: ":8081", Handler: wsContainer}
	return server.ListenAndServe()
}

func (r *RestResource) getStatus(request *restful.Request, response *restful.Response) {
	report := r.cache.GetLast()
	if report == nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusNotFound, "404: No reports found.")
		return
	}
	response.WriteAsJson(report)
}

// use this to avoid multiple response.WriteHeader calls
func writeJson(resp *restful.Response, r *Report) error {
	output, err := json.MarshalIndent(r, " ", " ")
	if err != nil {
		return err
	}
	_, err = resp.Write(output)
	return err
}

// watch method takes care of input channel also when there are no watch requests
func (r *RestResource) watch() {
	for {
		select {
		case report := <-r.watchChannelInput:
			if r.watching {
				r.internalWatchChannel <- report
			}
		}
	}
}

func (r *RestResource) watchStatus(request *restful.Request, response *restful.Response) {
	w := response.ResponseWriter
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(http.StatusOK)
	w.(http.Flusher).Flush()

	r.watching=true
	for {
		select {
		case <- w.(http.CloseNotifier).CloseNotify():
			r.watching=false
			return
		case report, ok := <-r.internalWatchChannel:
			if !ok {
				r.watching=false
				return
			}
			if err := writeJson(response, report); err != nil {
				continue

			}
			if len(r.internalWatchChannel) == 0 {
				w.(http.Flusher).Flush()
			}
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
