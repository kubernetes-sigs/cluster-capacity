package apiserver

import (
	"fmt"
	"github.com/emicklei/go-restful"
	"net/http"
	"time"
)

var TIMELAYOUT = "2006-01-02T15:04:05Z07:00"

type RestResource struct {
	cache *Cache
}

//TODO: add posibility to list records in time interval
//TODO: handle watch method
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

	ws.Route(ws.GET("/status/list").To(r.listStatus).
		Doc("List all reports since and to specified date.").
		Operation("listRange").
		Param(ws.QueryParameter("since", "RFC3339 standard").DataType("string")).
		Param(ws.QueryParameter("to", "RFC3339 standard").DataType("string")).
		Writes([]Report{}))
	container.Add(ws)
}

func NewResource(c *Cache) *RestResource {
	return &RestResource{
		cache: c,
	}
}
func ListenAndServe(r *RestResource) error {
	wsContainer := restful.NewContainer()
	r.Register(wsContainer)

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
